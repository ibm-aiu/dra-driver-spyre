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

package types

import (
	"strconv"

	cst "github.com/ibm-aiu/dra-driver-spyre/pkg/const"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/topology/types/pcitopo"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/utils"
	"github.com/jaypipes/ghw"
	resourceapi "k8s.io/api/resource/v1"
	klog "k8s.io/klog/v2"
)

// Constants related to device attribute and capacity
const (
	DeviceIndexAttribute   = "index"
	DriverVersionAttribute = "driverVersion"

	DeviceTypeAttribute     = "deviceType"
	DeviceTypeNameAttribute = "deviceTypeName"

	// DeviceStateAttribute holds the device health state reported by the spyre
	// health service (e.g. "ONLINE", "IN_ERROR"). It defaults to DeviceStateOnline
	// and is updated while the driver is running.
	DeviceStateAttribute = "deviceState"
	DeviceStateOnline    = "ONLINE"

	VendorAttribute           = "vendor"
	DriverAttribute           = "driver"
	ProductIdAttribute        = "productId"
	PciAddressesAttribute     = "pciAddress"
	PfPciAddressAttribute     = "pfPciAddress"
	IsPfAttribute             = "isPf"
	SubClassAttribute         = "subClass"
	VfIdAttribute             = "vfId"
	PciFunctionIndexAttribute = "pciFunctionIndex"
	VfEnabledAttribute        = "vfEnabled"
	NumaInfoAttribute         = "numaInfo"
	LinkSpeedAttribute        = "linkSpeed"
	ClockRPDAttribute         = "clockRPD"
	ClockSOCAttribute         = "clockSOC"
	MemoryBoostableAttribute  = "memoryBoostable"
	MemoryFrequencyAttribute  = "memoryFrequency"
	MemoryVendorAttribute     = "memoryVendor"
	MemorySpeedAttribute      = "memorySpeed"

	VfNumberCapacity   = "vfNum"
	MemorySizeCapacity = "memory"

	// Tier group attributes — each holds the canonical PCI address of the
	// group representative at that tier level.  Devices sharing the same value
	// belong to the same topology-aware group and can be co-allocated for
	// tensor-parallel workloads.
	//
	//   tier0Group: same PCI switch (peers_0)
	//   tier1Group: ≤1 PCI hop away (peers_0 ∪ peers_1)
	//   tier2Group: ≤2 PCI hops away (peers_0 ∪ peers_1 ∪ peers_2)
	Tier0GroupAttribute = "tier0Group"
	Tier1GroupAttribute = "tier1Group"
	Tier2GroupAttribute = "tier2Group"
)

type AllocatableDevices map[string]SpyreDevice

type SpyreDevice struct {
	ProductID
	resourceapi.Device
	PciDevice
}

func NewAllocatableDevices(hwDevices []*ghw.PCIDevice, pciTopo *pcitopo.Pcitopo, disableVirtualFunction bool) AllocatableDevices {
	alldevices := make(AllocatableDevices, 0)

	// Quick return
	if len(hwDevices) == 0 {
		return alldevices
	}

	index := 0
	spyreDevices := convertToSpyrePCIDevices(hwDevices)
	vfEnabled := hasVfDevice(spyreDevices) && !disableVirtualFunction
	var tierGroups *pcitopo.TierGroups
	if pciTopo != nil {
		tg := pcitopo.GenerateTierGroupsFromTopo(*pciTopo)
		tierGroups = &tg
	}
	for _, device := range spyreDevices {
		deviceName := utils.PciAddressToDeviceName(device.GetPciAddr())
		attributes := getAttributes(index, device, pciTopo, tierGroups, vfEnabled)
		capacities := getDeviceCapacity(device)
		resourceDevice := resourceapi.Device{
			Name:       deviceName,
			Attributes: attributes,
			Capacity:   capacities,
		}
		alldevices[deviceName] = SpyreDevice{
			ProductID: ProductID(device.GetProductID()),
			Device:    resourceDevice,
			PciDevice: device,
		}
		index += 1
	}
	return alldevices
}

// hasVfDevice reports whether any discovered device is a Spyre VF, which
// indicates that VF carving is currently active on this node.
func hasVfDevice(devices []PciDevice) bool {
	for _, device := range devices {
		if device.GetProductID() == string(ProductIDVf) {
			return true
		}
	}
	return false
}

func convertToSpyrePCIDevices(devices []*ghw.PCIDevice) []PciDevice {
	spyreDevices := make([]PciDevice, 0)
	for _, device := range devices {
		klog.V(1).Infof("+++ Device +++ : %v", device)
		if utils.IsPseudoDeviceMode() {
			newDevice := NewPseudoPciDevice(device)
			spyreDevices = append(spyreDevices, newDevice)
		} else {
			if newDevice, err := NewPciSpyreDevice(device); err == nil {
				spyreDevices = append(spyreDevices, newDevice)
			} else {
				klog.Errorf("error creating new device: %q", err)
			}
		}
	}
	return spyreDevices
}

func getAttributes(index int, spyreDevice PciDevice,
	topo *pcitopo.Pcitopo, tierGroups *pcitopo.TierGroups, vfEnabled bool) map[resourceapi.QualifiedName]resourceapi.DeviceAttribute {
	attributes := make(map[resourceapi.QualifiedName]resourceapi.DeviceAttribute, 0)
	attributes[DeviceIndexAttribute] = GetIntDeviceAttribute(index)
	attributes[DriverVersionAttribute] = GetStringDeviceAttribute(cst.DriverVersion)
	attributes[DeviceStateAttribute] = GetStringDeviceAttribute(DeviceStateOnline)

	addSpyreBasicAttributes(attributes, spyreDevice, vfEnabled)
	key := spyreDevice.GetPciAddr()
	if topo != nil && (topo.Devices != nil || topo.SpyreVfDevices != nil) {
		addTopologyMetadata(attributes, key, topo, tierGroups)
	}
	return attributes
}

func getDeviceCapacity(device PciDevice) map[resourceapi.QualifiedName]resourceapi.DeviceCapacity {
	capacities := make(map[resourceapi.QualifiedName]resourceapi.DeviceCapacity)
	// capacities[VfNumberCapacity]
	// capacities[MemorySizeCapacity]
	return capacities
}

func addSpyreBasicAttributes(attributes map[resourceapi.QualifiedName]resourceapi.DeviceAttribute, device PciDevice, vfEnabled bool) {
	attributes[VendorAttribute] = GetStringDeviceAttribute(device.GetVendor())
	attributes[DriverAttribute] = GetStringDeviceAttribute(device.GetDriver())
	attributes[ProductIdAttribute] = GetStringDeviceAttribute(device.GetProductID())
	attributes[PciAddressesAttribute] = GetStringDeviceAttribute(device.GetPciAddr())
	attributes[PfPciAddressAttribute] = GetStringDeviceAttribute(device.GetPfPciAddr())
	attributes[IsPfAttribute] = GetBoolDeviceAttribute(device.IsSriovPF())
	attributes[SubClassAttribute] = GetStringDeviceAttribute(device.GetSubClass())
	attributes[VfIdAttribute] = GetIntDeviceAttribute(device.GetVFID())
	attributes[NumaInfoAttribute] = GetStringDeviceAttribute(device.GetNumaInfo())
	attributes[PciFunctionIndexAttribute] = GetIntDeviceAttribute(utils.GetPCIFunctionIndex(device.GetPciAddr()))
	attributes[VfEnabledAttribute] = GetBoolDeviceAttribute(vfEnabled)
}

func addTopologyMetadata(attributes map[resourceapi.QualifiedName]resourceapi.DeviceAttribute,
	key string, topo *pcitopo.Pcitopo, tierGroups *pcitopo.TierGroups) {
	if device, found := lookupTopoDevice(topo, key); found {
		addDeviceTopoAttributes(attributes, device)
	}
	addTierGroupAttributes(attributes, tierGroups, key)
}

// lookupTopoDevice returns the Device entry for key from PF or VF maps.
func lookupTopoDevice(topo *pcitopo.Pcitopo, key string) (pcitopo.Device, bool) {
	if dev, ok := topo.Devices[key]; ok {
		return dev, true
	}
	dev, ok := topo.SpyreVfDevices[key]
	return dev, ok
}

// addDeviceTopoAttributes stamps link speed, numa, metadata and isPf onto attributes.
func addDeviceTopoAttributes(attributes map[resourceapi.QualifiedName]resourceapi.DeviceAttribute, device pcitopo.Device) {
	attributes[LinkSpeedAttribute] = GetStringDeviceAttribute(device.Linkspeed)
	if !utils.IsPseudoDeviceMode() {
		attributes[NumaInfoAttribute] = GetStringDeviceAttribute(strconv.Itoa(device.NumaNode))
	}
	addTopologyMetadataFromMap(attributes, device.Metadata)
	attributes[IsPfAttribute] = GetBoolDeviceAttribute(device.IsPf)
}

// addTopologyMetadataFromMap extracts clock and memory sub-maps from metadata.
func addTopologyMetadataFromMap(attributes map[resourceapi.QualifiedName]resourceapi.DeviceAttribute, metadata map[string]any) {
	if metadata == nil {
		return
	}
	if clockMap, ok := metadata[pcitopo.ClockKey].(map[string]string); ok {
		addTopologyMetadataClock(attributes, clockMap)
	}
	if memoryMap, ok := metadata[pcitopo.MemoryKey].(map[string]any); ok {
		addTopologyMetadataMemory(attributes, memoryMap)
	}
}

// addTierGroupAttributes stamps tier0/1/2 group IDs onto attributes.
func addTierGroupAttributes(attributes map[resourceapi.QualifiedName]resourceapi.DeviceAttribute, tierGroups *pcitopo.TierGroups, key string) {
	if tierGroups == nil {
		return
	}
	setIfPresent(attributes, Tier0GroupAttribute, tierGroups.Tier0[key])
	setIfPresent(attributes, Tier1GroupAttribute, tierGroups.Tier1[key])
	setIfPresent(attributes, Tier2GroupAttribute, tierGroups.Tier2[key])
}

// setIfPresent sets a string attribute only when value is non-empty.
func setIfPresent(attributes map[resourceapi.QualifiedName]resourceapi.DeviceAttribute, key resourceapi.QualifiedName, value string) {
	if value != "" {
		attributes[key] = GetStringDeviceAttribute(value)
	}
}

func addTopologyMetadataClock(attributes map[resourceapi.QualifiedName]resourceapi.DeviceAttribute,
	clockMap map[string]string) {
	if rpd, ok := clockMap[pcitopo.ClockRPDKey]; ok {
		attributes[ClockRPDAttribute] = GetStringDeviceAttribute(rpd)
	}
	if soc, ok := clockMap[pcitopo.ClockSOCKey]; ok {
		attributes[ClockSOCAttribute] = GetStringDeviceAttribute(soc)
	}
}

func addTopologyMetadataMemory(attributes map[resourceapi.QualifiedName]resourceapi.DeviceAttribute,
	memoryMap map[string]any) {
	if val, ok := memoryMap[pcitopo.MemoryBoostableKey]; ok {
		if valBool, ok := val.(bool); ok {
			attributes[MemoryBoostableAttribute] = GetBoolDeviceAttribute(valBool)
		}
	}
	addTopologyMetadataMemoryStr(attributes, memoryMap, pcitopo.MemoryFrequencyKey, MemoryFrequencyAttribute)
	addTopologyMetadataMemoryStr(attributes, memoryMap, pcitopo.MemoryVendorKey, MemoryVendorAttribute)
	addTopologyMetadataMemoryStr(attributes, memoryMap, pcitopo.MemorySpeedKey, MemorySpeedAttribute)
}

func addTopologyMetadataMemoryStr(attributes map[resourceapi.QualifiedName]resourceapi.DeviceAttribute,
	memoryMap map[string]any, mapKey string, attrKey resourceapi.QualifiedName) {
	if val, ok := memoryMap[mapKey]; ok {
		if valStr, ok := val.(string); ok {
			attributes[attrKey] = GetStringDeviceAttribute(valStr)
		}
	}
}
