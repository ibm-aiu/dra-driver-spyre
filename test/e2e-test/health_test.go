// (C) Copyright IBM Corp. 2025,2026
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package e2e_test

import (
	"context"
	"os"
	"time"

	"github.com/ibm-aiu/dra-driver-spyre/test/testutil"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HealthMockKey enables the device health specs. They need the driver to be
// deployed with the spyre health service mock sidecar (make deploy-driver-health-mock).
const HealthMockKey = "E2E_HEALTH_MOCK"

// pseudoPFAddresses are the PF devices of the pseudo discovery.
var pseudoPFAddresses = []string{
	"0000:1a:00.0", "0000:1c:00.0", "0000:1d:00.0", "0000:1e:00.0",
	"0000:3d:00.0", "0000:3f:00.0", "0000:40:00.0", "0000:41:00.0",
}

const (
	stateOnline  = "ONLINE"
	stateInError = "IN_ERROR"
)

var _ = Describe("device health e2e test", Ordered, func() {
	var (
		ctx     = context.Background()
		mockPod string
	)

	BeforeAll(func() {
		if os.Getenv(HealthMockKey) == "" {
			Skip(HealthMockKey + " is not set; the driver is not deployed with the health service mock")
		}
		By("finding the driver pod that runs the health service mock")
		Eventually(func(g Gomega) {
			pods := testutil.GetPodsWithLabels(ctx, k8sClientset, g, SpyreDRADriverNamespace, SpyreDRADriverLabel, "")
			g.Expect(pods).To(HaveLen(1))
			mockPod = pods[0].Name
		}).WithTimeout(time.Minute).WithPolling(5 * time.Second).Should(Succeed())

		By("waiting until the driver registered the devices with the mock")
		Eventually(func(g Gomega) {
			states := testutil.GetHealthMockStates(ctx, k8sClientset, g, SpyreDRADriverNamespace, mockPod)
			for _, addr := range pseudoPFAddresses {
				g.Expect(states).To(HaveKey(addr))
			}
		}).WithTimeout(2 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())
	})

	// Restore every PF so one failing spec does not leak into the next one.
	AfterEach(func() {
		if mockPod == "" {
			return
		}
		for _, addr := range pseudoPFAddresses {
			testutil.SetHealthMockState(ctx, k8sClientset, Default, SpyreDRADriverNamespace, mockPod, addr, stateOnline)
		}
	})

	It("publishes ONLINE for all devices by default", func() {
		Eventually(func(g Gomega) {
			states := testutil.ListDeviceStates(ctx, k8sClientset)
			for _, addr := range pseudoPFAddresses {
				g.Expect(states).To(HaveKeyWithValue(addr, stateOnline))
			}
		}).WithTimeout(time.Minute).WithPolling(2 * time.Second).Should(Succeed())
	})

	It("reflects a device state change in the ResourceSlice and back", func() {
		target := pseudoPFAddresses[0]

		By("marking one PF as IN_ERROR")
		testutil.SetHealthMockState(ctx, k8sClientset, Default, SpyreDRADriverNamespace, mockPod, target, stateInError)
		Eventually(func(g Gomega) {
			states := testutil.ListDeviceStates(ctx, k8sClientset)
			g.Expect(states).To(HaveKeyWithValue(target, stateInError))
			for _, addr := range pseudoPFAddresses[1:] {
				g.Expect(states).To(HaveKeyWithValue(addr, stateOnline))
			}
		}).WithTimeout(time.Minute).WithPolling(2 * time.Second).Should(Succeed())

		By("restoring the PF to ONLINE")
		testutil.SetHealthMockState(ctx, k8sClientset, Default, SpyreDRADriverNamespace, mockPod, target, stateOnline)
		Eventually(func(g Gomega) {
			g.Expect(testutil.ListDeviceStates(ctx, k8sClientset)).To(HaveKeyWithValue(target, stateOnline))
		}).WithTimeout(time.Minute).WithPolling(2 * time.Second).Should(Succeed())
	})

	It("keeps a claim selecting ONLINE devices pending until a device is healthy", func() {
		By("marking all PFs as IN_ERROR")
		for _, addr := range pseudoPFAddresses {
			testutil.SetHealthMockState(ctx, k8sClientset, Default, SpyreDRADriverNamespace, mockPod, addr, stateInError)
		}
		Eventually(func(g Gomega) {
			states := testutil.ListDeviceStates(ctx, k8sClientset)
			for _, addr := range pseudoPFAddresses {
				g.Expect(states).To(HaveKeyWithValue(addr, stateInError))
			}
		}).WithTimeout(time.Minute).WithPolling(2 * time.Second).Should(Succeed())

		testNamespace := testutil.CreateRandomTestNamespace(ctx, k8sClientset)
		claimName := "health-claim"
		podName := "health-pod"
		By("deploying a claim that only selects ONLINE devices")
		claimData := testutil.BasicResourceClaimTemplateData(claimName, testNamespace).
			SetCount(1).
			SetDeviceState(stateOnline)
		testutil.BuildResourceClaimTemplate(ctx, dynClient, discoClient, claimData)
		podData := testutil.BasicPodTemplateData(podName, testNamespace).SetArg0(testutil.PrintSenlibConfig)
		testutil.BuildPod(ctx, dynClient, discoClient, podData, claimName)

		By("expecting the pod to stay Pending while no device is ONLINE")
		Consistently(func(g Gomega) {
			pod, err := k8sClientset.CoreV1().Pods(testNamespace).Get(ctx, podName, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(pod.Status.Phase).To(Equal(v1.PodPending))
		}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).Should(Succeed())

		By("restoring one PF to ONLINE")
		testutil.SetHealthMockState(ctx, k8sClientset, Default, SpyreDRADriverNamespace, mockPod, pseudoPFAddresses[0], stateOnline)
		testutil.CheckPodPhases(ctx, k8sClientset, []*testutil.PodTemplateData{podData}, map[v1.PodPhase]int{v1.PodRunning: 1})
		allocations := testutil.CheckAndGetAllocationsFromPodLog(ctx, k8sClientset, podName, testNamespace, claimData.IsVF())
		Expect(allocations).To(ConsistOf(pseudoPFAddresses[0]))

		By("deleting pod")
		testutil.DeletePod(ctx, k8sClientset, podData)
	})
})
