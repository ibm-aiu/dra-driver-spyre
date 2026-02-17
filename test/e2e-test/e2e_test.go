// Copyright 2026 IBM Corp.
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

	"github.com/ibm-aiu/dra-driver-spyre/test/testutil"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
)

var _ = Describe("e2e test", Ordered, func() {
	Context("pseudo mode", func() {
		DescribeTable("simple allocation", func(count int, expectedPhase v1.PodPhase) {
			ctx := context.Background()
			claimName := "test-claim"
			podName := "test-pod"
			testNamespace := testutil.CreateRandomTestNamespace(ctx, k8sClientset)
			By("deploying ResourceClaimTemplate")
			claimData := testutil.BasicResourceClaimTemplateData(claimName, testNamespace).SetCount(count)
			testutil.BuildResourceClaimTemplate(ctx, dynClient, discoClient, claimData)
			By("deploying Pod")
			podData := testutil.BasicPodTemplateData(podName, testNamespace).SetArg0(testutil.PrintSenlibConfig)
			testutil.BuildPod(ctx, dynClient, discoClient, podData, claimName)
			testutil.CheckPodPhases(ctx, k8sClientset, []*testutil.PodTemplateData{podData}, map[v1.PodPhase]int{expectedPhase: 1})
			if expectedPhase == v1.PodRunning {
				allocations := testutil.CheckAndGetAllocationsFromPodLog(ctx, k8sClientset, podName, testNamespace)
				Expect(allocations).To(HaveLen(count))
			}
			By("deleting pod")
			testutil.DeletePod(ctx, k8sClientset, podData)
		}, Entry("one device", 1, v1.PodRunning),
			Entry("two devices", 2, v1.PodRunning),
			Entry("four devices", 4, v1.PodRunning),
			Entry("all devices", 8, v1.PodRunning),
			Entry("over availability", 9, v1.PodPending),
		)
	})
})
