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
	"os"
	"testing"
	"time"

	"github.com/ibm-aiu/dra-driver-spyre/test/testutil"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	resourcev1beta1 "k8s.io/api/resource/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var k8sClientset *kubernetes.Clientset
var dynClient *dynamic.DynamicClient
var discoClient *discovery.DiscoveryClient
var scheme = runtime.NewScheme()

const (
	KubeConfigFilePathKey   = "E2E_KUBECONFIG"
	SpyreDRADriverNamespace = "dra-driver-spyre"
	SpyreDRADriverLabel     = "app=dra-driver-spyre"
)

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Spyre DRA Driver End 2 End Suite")
}

var _ = BeforeSuite(func() {
	log.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	kubeconfig, ok := os.LookupEnv(KubeConfigFilePathKey)
	Expect(ok).To(BeTrue(), "%s must be set", KubeConfigFilePathKey)
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	Expect(err).To(BeNil())

	// further configure the client
	config.Timeout = 90 * time.Second
	config.Burst = 100
	config.QPS = 50.0
	config.WarningHandler = rest.NoWarnings{}

	// instantiate the client
	k8sClientset, err = kubernetes.NewForConfig(config)
	Expect(err).To(BeNil())
	err = clientgoscheme.AddToScheme(scheme)
	Expect(err).To(BeNil())
	err = resourcev1beta1.AddToScheme(scheme)
	Expect(err).To(BeNil())
	err = corev1.AddToScheme(scheme)
	Expect(err).To(BeNil())
	dynClient, err = dynamic.NewForConfig(config)
	Expect(err).To(BeNil())
	discoClient, err = discovery.NewDiscoveryClientForConfig(config)
	Expect(err).To(BeNil())

	By("Wait for DRA Driver to be running")
	ctx := context.Background()
	Eventually(func(g Gomega) {
		pods := testutil.GetPodsWithLabels(ctx, k8sClientset, g, SpyreDRADriverNamespace, SpyreDRADriverLabel, "")
		g.Expect(pods).To(HaveLen(1))
		g.Expect(pods[0].Status.Phase).To(BeEquivalentTo(corev1.PodRunning))
	}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())

	By("Wait for ResourceSlice")
	Eventually(func(g Gomega) {
		slices := testutil.ListResourceSlices(ctx, k8sClientset)
		g.Expect(slices).To(HaveLen(1))
		g.Expect(slices[0].Spec.Devices).To(HaveLen(8))
	}).WithTimeout(2 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())
})

var _ = AfterSuite(func() {
})
