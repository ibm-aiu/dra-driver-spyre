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

// health.go defines helpers for the device health e2e tests: reading the
// deviceState attribute from the ResourceSlice and driving the spyre health
// service mock (test/healthmock) that runs as a sidecar of the kubelet plugin.

package testutil

import (
	"context"
	"encoding/json"
	"fmt"

	. "github.com/onsi/gomega"
	resourcev1 "k8s.io/api/resource/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// deviceStateAttribute is the bare key of the device health attribute in the ResourceSlice.
	deviceStateAttribute = "deviceState"
	// HealthMockControlPort is the port of the health mock control API.
	HealthMockControlPort = "9090"
)

// DeviceStatesFromSlices returns pciAddress -> deviceState for every device in
// the slices that publishes both attributes.
func DeviceStatesFromSlices(slices []resourcev1.ResourceSlice) map[string]string {
	states := make(map[string]string)
	for _, slice := range slices {
		for _, dev := range slice.Spec.Devices {
			addr, ok := dev.Attributes[pciAddressAttribute]
			if !ok || addr.StringValue == nil {
				continue
			}
			state, ok := dev.Attributes[deviceStateAttribute]
			if !ok || state.StringValue == nil {
				continue
			}
			states[*addr.StringValue] = *state.StringValue
		}
	}
	return states
}

// ListDeviceStates reads the deviceState attribute of all published devices.
func ListDeviceStates(ctx context.Context, k8sClientset *kubernetes.Clientset) map[string]string {
	return DeviceStatesFromSlices(ListResourceSlices(ctx, k8sClientset))
}

// healthMockProxyName returns the API server pod proxy name of the mock control
// API, so the test needs no direct network path to the pod.
func healthMockProxyName(podName string) string {
	return fmt.Sprintf("http:%s:%s", podName, HealthMockControlPort)
}

// GetHealthMockStates returns the states (PCI address -> state name) tracked by the mock.
func GetHealthMockStates(ctx context.Context, k8sClientset *kubernetes.Clientset, g Gomega, namespace, podName string) map[string]string {
	raw, err := k8sClientset.CoreV1().RESTClient().Get().
		Namespace(namespace).Resource("pods").SubResource("proxy").
		Name(healthMockProxyName(podName)).Suffix("state").
		DoRaw(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	states := map[string]string{}
	g.Expect(json.Unmarshal(raw, &states)).To(Succeed())
	return states
}

// SetHealthMockState sets the state (e.g. "IN_ERROR", "ONLINE") of the device
// with the given PCI address in the mock. The device must already be tracked.
func SetHealthMockState(ctx context.Context, k8sClientset *kubernetes.Clientset, g Gomega, namespace, podName, pciAddress, state string) {
	_, err := k8sClientset.CoreV1().RESTClient().Post().
		Namespace(namespace).Resource("pods").SubResource("proxy").
		Name(healthMockProxyName(podName)).Suffix("state").
		Param("addr", pciAddress).Param("state", state).
		DoRaw(ctx)
	g.Expect(err).NotTo(HaveOccurred())
}
