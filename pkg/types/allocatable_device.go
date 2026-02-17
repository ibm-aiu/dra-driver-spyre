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

package types

import (
	"strconv"

	cst "github.com/ibm-aiu/dra-driver-spyre/pkg/const"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/topology/types/pcitopo"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/utils"
	"github.com/jaypipes/ghw"
	resourceapi "k8s.io/api/resource/v1beta1"
	klog "k8s.io/klog/v2"
)

// Constants related to device attribute and capacity
const (
	DeviceIndexAttribute   = "index"
	DriverVersionAttribute = "driverVersion"

	DeviceTypeAttribute     = "deviceType"
	DeviceTypeNameAttribute = "deviceTypeName"

	VendorAttribute          = "vendor"
	DriverAttribute          = "driver"
	ProductIdAttribute       = "productId"
	PciAddressesAttribute    = "pciAddress"
	PfPciAddressAttribute    = "pfPciAddress"
	IsPfAttribute            = "isPf"
	SubClassAttribute        = "subClass"
	VfIdAttribute            = "vfId"
	NumaInfoAttribute        = "numaInfo"
	LinkSpeedAttribute       = "linkSpeed"
	ClockRPDAttribute        = "clockRPD"
	ClockSOCAttribute        = "clockSOC"
	MemoryBoostableAttribute = "memoryBoostable"
	MemoryFrequencyAttribute = "memoryFrequency"
	MemoryVendorAttribute    = "memoryVendor"
	MemorySpeedAttribute     = "memorySpeed"

	VfNumberCapacity   = "vfNum"
	MemorySizeCapacity = "memory"
)

type AllocatableDevices map[string]SpyreDevice

type SpyreDevice struct {
	ProductID
	resourceapi.Device
	PciDevice
}

func NewAllocatableDevices(hwDevices []*ghw.PCIDevice, pciTopo *pcitopo.Pcitopo) AllocatableDevices {
	alldevices := make(AllocatableDevices, 0)

	// Quick return
	if len(hwDevices) == 0 {
		return alldevices
	}

	index := 0
	spyreDevices := convertToSpyrePCIDevices(hwDevices)
	// To remove this logic when device plugin's PseudoPciDevice supports numa info mock.
	var pseudoNumMap map[string]string
	if pciTopo != nil && utils.IsPseudoDeviceMode() {
		pseudoNumMap = pcitopo.GenerateNumaInfoMapFromTopo(*pciTopo)
	}
	for _, device := range spyreDevices {
		deviceName := utils.PciAddressToDeviceName(device.GetPciAddr())
		attributes := getAttributes(index, device, pciTopo, pseudoNumMap)
		capacities := getDeviceCapacity(device)
		resourceDevice := resourceapi.Device{
			Name: deviceName,
			Basic: &resourceapi.BasicDevice{
				Attributes: attributes,
				Capacity:   capacities,
			},
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
	topo *pcitopo.Pcitopo, pseudoNumaMap map[string]string) map[resourceapi.QualifiedName]resourceapi.DeviceAttribute {
	attributes := make(map[resourceapi.QualifiedName]resourceapi.DeviceAttribute, 0)
	attributes[DeviceIndexAttribute] = GetIntDeviceAttribute(index)
	attributes[DriverVersionAttribute] = GetStringDeviceAttribute(cst.DriverVersion)

	addSpyreBasicAttributes(attributes, spyreDevice)
	key := spyreDevice.GetPciAddr()
	if topo != nil && topo.Devices != nil {
		addTopologyMetadata(attributes, key, topo, pseudoNumaMap)
	}
	return attributes
}

func getDeviceCapacity(device PciDevice) map[resourceapi.QualifiedName]resourceapi.DeviceCapacity {
	capacities := make(map[resourceapi.QualifiedName]resourceapi.DeviceCapacity)
	// capacities[VfNumberCapacity]
	// capacities[MemorySizeCapacity]
	return capacities
}

func addSpyreBasicAttributes(attributes map[resourceapi.QualifiedName]resourceapi.DeviceAttribute, device PciDevice) {
	attributes[VendorAttribute] = GetStringDeviceAttribute(device.GetVendor())
	attributes[DriverAttribute] = GetStringDeviceAttribute(device.GetDriver())
	attributes[ProductIdAttribute] = GetStringDeviceAttribute(device.GetProductID())
	attributes[PciAddressesAttribute] = GetStringDeviceAttribute(device.GetPciAddr())
	attributes[PfPciAddressAttribute] = GetStringDeviceAttribute(device.GetPfPciAddr())
	attributes[IsPfAttribute] = GetBoolDeviceAttribute(device.IsSriovPF())
	attributes[SubClassAttribute] = GetStringDeviceAttribute(device.GetSubClass())
	attributes[VfIdAttribute] = GetIntDeviceAttribute(device.GetVFID())
	attributes[NumaInfoAttribute] = GetStringDeviceAttribute(device.GetNumaInfo())
}

func addTopologyMetadata(attributes map[resourceapi.QualifiedName]resourceapi.DeviceAttribute,
	key string, topo *pcitopo.Pcitopo, pseudoNumaMap map[string]string) {
	if device, found := topo.Devices[key]; found {
		attributes[LinkSpeedAttribute] = GetStringDeviceAttribute(device.Linkspeed)
		attributes[NumaInfoAttribute] = GetStringDeviceAttribute(strconv.Itoa(device.NumaNode))
		if device.Metadata != nil {
			if clock, ok := device.Metadata[pcitopo.ClockKey]; ok {
				if clockMap, ok := clock.(map[string]string); ok {
					addTopologyMetadataClock(attributes, clockMap)
				}
			}
			if memory, ok := device.Metadata[pcitopo.MemoryKey]; ok {
				if memoryMap, ok := memory.(map[string]any); ok {
					addTopologyMetadataMemory(attributes, memoryMap)
				}
			}
		}
		// override
		attributes[IsPfAttribute] = GetBoolDeviceAttribute(device.IsPf)
	}

	// PseudoPciDevice's GetNumaInfo always return zero. Need to override the numa info.
	if pseudoNumaMap != nil {
		// To remove this logic when device plugin's PseudoPciDevice supports numa info mock.
		if numa, found := pseudoNumaMap[key]; found {
			attributes[NumaInfoAttribute] = GetStringDeviceAttribute(numa)
		}
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
