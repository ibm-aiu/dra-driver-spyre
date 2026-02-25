// (C) Copyright IBM Corp.  2025,2026 IBM Corp.
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

// resourceclaim.go defines functions to handle ResourceClaim and ResourceClaimTemplate resource
// and their corresponding variables and constants

package testutil

import (
	"context"

	. "github.com/onsi/gomega"
	resourcev1beta1 "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const (
	numaAttribute = "spyre.ibm.com/numaInfo"
	PfProductId   = "06a7"
	VfProductId   = "06a8"
)

const ResourceClaimTemplate = `
apiVersion: resource.k8s.io/v1beta1
kind: ResourceClaimTemplate
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
spec:
  spec:
    devices:
      requests:
      - name: spyre
        deviceClassName: spyre.ibm.com
        {{- if gt .Count 0 }}
        count: {{ .Count }}
        {{- end}}
        {{- if .PCIAddress }}
        selectors:
        - cel:
            expression: |-
              device.attributes["spyre.ibm.com"].pciAddress == "{{ .PCIAddress }}"
        {{- else if .ProductId }}
        selectors:
        - cel:
            expression: |-
              device.attributes["spyre.ibm.com"].productId == "{{ .ProductId }}"
        {{- end}}
      {{- if .MatchAttribute }}
      constraints:
      - requests: ["spyre"]
        matchAttribute: {{ .MatchAttribute }}
      {{- end}}
`

// ResourceClaimTemplateData holds data used to populate a Kubernetes ResourceClaimTemplate template.
//
// Example usage:
//
//	data := BasicResourceClaimTemplateData(name, namespace)
//	data = data.SetCount(image) // default: not set (DRA default: 1)
//	data = data.SetPCIAddressSelector(pciAddress) // default: not set
//	data = data.SetProductId(productId)   // default: not set
//	data = data.SetMatchAttribute(matchAttribute) // default: not set
//
// pciAddress and productId cannot be applied at the same time.
// If both specified, pciAddress will be used.
type ResourceClaimTemplateData struct {
	Name           string
	Namespace      string
	Count          int
	PCIAddress     string
	ProductId      string
	MatchAttribute string
}

// BasicResourceClaimTemplateData init data
func BasicResourceClaimTemplateData(name, namespace string) *ResourceClaimTemplateData {
	return &ResourceClaimTemplateData{
		Name:      name,
		Namespace: namespace,
		ProductId: PfProductId, // default product id
	}
}

// SetCount sets count for ExactCount which is default allocation mode
func (c *ResourceClaimTemplateData) SetCount(count int) *ResourceClaimTemplateData {
	c.Count = count
	return c
}

// SetPCIAddressSelector sets PCI address in selector
func (c *ResourceClaimTemplateData) SetPCIAddressSelector(pciAddress string) *ResourceClaimTemplateData {
	c.PCIAddress = pciAddress
	return c
}

// SetProductId sets productId in selector
func (c *ResourceClaimTemplateData) SetProductId(productId string) *ResourceClaimTemplateData {
	c.ProductId = productId
	return c
}

// SetMatchAttribute sets an attribute to match in constraints
func (c *ResourceClaimTemplateData) SetMatchAttribute(matchAttribute string) *ResourceClaimTemplateData {
	c.MatchAttribute = matchAttribute
	return c
}

// IsVF returns whether the claim requests for vf
func (c *ResourceClaimTemplateData) IsVF() bool {
	return c.ProductId == VfProductId
}

// BuildResourceClaimTemplate creates ResourceClaimTemplate according to template data
// Always check nil error.
func BuildResourceClaimTemplate(ctx context.Context, dynClient *dynamic.DynamicClient, discoClient *discovery.DiscoveryClient,
	data *ResourceClaimTemplateData) {
	yamlData := YamlFromTemplate(ResourceClaimTemplate, *data)
	_, err := CreateResourceFromYaml(ctx, dynClient, discoClient, data.Namespace, yamlData)
	Expect(err).To(BeNil())
}

// BuildTopologyAwareResourceClaimTemplate creates ResourceClaimTemplate
// with numa match for specific number of devices with specific productId (PF or VF)
func BuildTopologyAwareResourceClaimTemplate(ctx context.Context, dynClient *dynamic.DynamicClient, discoClient *discovery.DiscoveryClient,
	name, namespace string, count int, productId string) {
	data := BasicResourceClaimTemplateData(name, namespace).SetCount(count).SetMatchAttribute(numaAttribute).SetProductId(productId)
	BuildResourceClaimTemplate(ctx, dynClient, discoClient, data)
}

// BuildPCISpecificResourceClaimTemplate creates ResourceClaimTemplate with specific pci address
func BuildPCISpecificResourceClaimTemplate(ctx context.Context, dynClient *dynamic.DynamicClient, discoClient *discovery.DiscoveryClient,
	name, namespace string, pciAddress string) {
	data := BasicResourceClaimTemplateData(name, namespace).SetPCIAddressSelector(pciAddress)
	BuildResourceClaimTemplate(ctx, dynClient, discoClient, data)
}

func ListResourceSlices(ctx context.Context, k8sClientset *kubernetes.Clientset) []resourcev1beta1.ResourceSlice {
	slices, err := k8sClientset.ResourceV1beta1().ResourceSlices().List(ctx, metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())
	return slices.Items
}

func ListResourceClaims(ctx context.Context, k8sClientset *kubernetes.Clientset, namespace string) []resourcev1beta1.ResourceClaim {
	claims, err := k8sClientset.ResourceV1beta1().ResourceClaims(namespace).List(ctx, metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())
	return claims.Items
}
