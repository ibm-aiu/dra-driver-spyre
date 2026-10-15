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

// resourceclaim.go defines functions to handle ResourceClaim and ResourceClaimTemplate resource
// and their corresponding variables and constants

package testutil

import (
	"context"

	. "github.com/onsi/gomega"
	resourcev1 "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const (
	numaAttribute = "spyre.ibm.com/numaInfo"
	PfProductId   = "06a7"
	VfProductId   = "06a8"

	pfDeviceClassName = "spyre-pf"
	vfDeviceClassName = "spyre-standard-vf"

	// Bare attribute key names as stored in the ResourceSlice map (no domain prefix).
	pciAddressAttribute = "pciAddress"
	tier0GroupAttribute = "tier0Group"
	tier1GroupAttribute = "tier1Group"
	tier2GroupAttribute = "tier2Group"

	// Fully-qualified matchAttribute values used in ResourceClaimTemplate constraints.
	// These enforce that the scheduler picks devices that all share the same tier group.
	Tier0MatchAttribute = "spyre.ibm.com/tier0Group"
	Tier1MatchAttribute = "spyre.ibm.com/tier1Group"
	Tier2MatchAttribute = "spyre.ibm.com/tier2Group"
)

const ResourceClaimTemplate = `
apiVersion: resource.k8s.io/v1
kind: ResourceClaimTemplate
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
spec:
  spec:
    devices:
      requests:
      - name: spyre
        exactly:
          deviceClassName: {{ .DeviceClassName }}
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
	Name            string
	Namespace       string
	Count           int
	PCIAddress      string
	ProductId       string
	MatchAttribute  string
	DeviceClassName string
}

// BasicResourceClaimTemplateData init data
func BasicResourceClaimTemplateData(name, namespace string) *ResourceClaimTemplateData {
	return &ResourceClaimTemplateData{
		Name:            name,
		Namespace:       namespace,
		ProductId:       PfProductId, // default product id
		DeviceClassName: pfDeviceClassName,
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

// SetProductId sets productId in selector, and switches the DeviceClass
// (spyre-pf vs spyre-standard-vf) to match
func (c *ResourceClaimTemplateData) SetProductId(productId string) *ResourceClaimTemplateData {
	c.ProductId = productId
	if productId == VfProductId {
		c.DeviceClassName = vfDeviceClassName
	} else {
		c.DeviceClassName = pfDeviceClassName
	}
	return c
}

// SetDeviceClassName overrides the DeviceClass used by the claim template.
func (c *ResourceClaimTemplateData) SetDeviceClassName(className string) *ResourceClaimTemplateData {
	c.DeviceClassName = className
	return c
}

// SetPfTierAllocation configures the claim for PF topology-aware allocation at the
// given tier (0, 1, or 2).  It selects the spyre-pf DeviceClass and sets the
// matchAttribute so the scheduler must pick devices from the same tier group.
func (c *ResourceClaimTemplateData) SetPfTierAllocation(tier int) *ResourceClaimTemplateData {
	c.DeviceClassName = pfDeviceClassName
	c.ProductId = PfProductId
	switch tier {
	case 0:
		c.MatchAttribute = Tier0MatchAttribute
	case 1:
		c.MatchAttribute = Tier1MatchAttribute
	default:
		c.MatchAttribute = Tier2MatchAttribute
	}
	return c
}

// SetVfTierAllocation configures the claim for VF topology-aware allocation at the
// given tier (0, 1, or 2).  It selects the spyre-standard-vf DeviceClass and sets
// the matchAttribute so the scheduler must pick devices from the same tier group.
func (c *ResourceClaimTemplateData) SetVfTierAllocation(tier int) *ResourceClaimTemplateData {
	c.DeviceClassName = vfDeviceClassName
	c.ProductId = VfProductId
	switch tier {
	case 0:
		c.MatchAttribute = Tier0MatchAttribute
	case 1:
		c.MatchAttribute = Tier1MatchAttribute
	default:
		c.MatchAttribute = Tier2MatchAttribute
	}
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

// CheckAllocationsShareTierGroup reads the ResourceSlice and verifies that all
// allocated PCI addresses share the same tier group attribute value at the
// requested tier (0, 1, or 2).  It returns the common group ID so callers can
// perform further assertions if needed.
func CheckAllocationsShareTierGroup(ctx context.Context, k8sClientset *kubernetes.Clientset,
	allocations []string, tier int) string {
	Expect(tier).To(BeElementOf(0, 1, 2), "tier must be 0, 1, or 2")

	var attrKey string
	switch tier {
	case 0:
		attrKey = tier0GroupAttribute
	case 1:
		attrKey = tier1GroupAttribute
	default:
		attrKey = tier2GroupAttribute
	}

	slices := ListResourceSlices(ctx, k8sClientset)
	// Build address → tierXGroup map from the ResourceSlice attribute data.
	groupByAddr := make(map[string]string)
	for _, slice := range slices {
		for _, dev := range slice.Spec.Devices {
			pciAddrAttr, ok := dev.Attributes[pciAddressAttribute]
			if !ok || pciAddrAttr.StringValue == nil {
				continue
			}
			groupAttr, ok := dev.Attributes[resourcev1.QualifiedName(attrKey)]
			if !ok || groupAttr.StringValue == nil {
				continue
			}
			groupByAddr[*pciAddrAttr.StringValue] = *groupAttr.StringValue
		}
	}

	var commonGroup string
	for _, addr := range allocations {
		g, ok := groupByAddr[addr]
		Expect(ok).To(BeTrue(), "address %s not found in ResourceSlice tier%d attribute", addr, tier)
		if commonGroup == "" {
			commonGroup = g
		} else {
			Expect(g).To(Equal(commonGroup),
				"address %s has tier%d group %q, expected %q (same group as %s)",
				addr, tier, g, commonGroup, allocations[0])
		}
	}
	return commonGroup
}

func ListResourceSlices(ctx context.Context, k8sClientset *kubernetes.Clientset) []resourcev1.ResourceSlice {
	slices, err := k8sClientset.ResourceV1().ResourceSlices().List(ctx, metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())
	return slices.Items
}

func ListResourceClaims(ctx context.Context, k8sClientset *kubernetes.Clientset, namespace string) []resourcev1.ResourceClaim {
	claims, err := k8sClientset.ResourceV1().ResourceClaims(namespace).List(ctx, metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())
	return claims.Items
}
