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

package discovery

import (
	"fmt"
	"strconv"

	"github.com/ibm-aiu/dra-driver-spyre/pkg/topology"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/types"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/utils"
	"github.com/jaypipes/ghw"
	"golang.org/x/exp/slices"
	klog "k8s.io/klog/v2"
)

const (
	maxVendorNameLen  = 20
	maxProductNameLen = 40
	classIDBaseInt    = 16
	classIDBitSize    = 64
)

var (
	/*
		Supported PCI Device Classes. ref: https://pci-ids.ucw.cz/read/PD
		12	Processing accelerators

		Processing accelerators subclasses. ref: https://pci-ids.ucw.cz/read/PD/12
		00	Processing accelerators
		01	AI Inference Accelerator
	*/
	supportedDeviceCodes = []int64{0x00, 0x12}
	supportedDrivers     = []string{"vfio-pci"}
	supportedVendors     = []string{"1014"}
	supportedProductIDs  = []string{"06a7", "06a8"}
)

type DeviceDiscovery struct {
	topologyFile string
}

func NewDeviceDiscovery(topologyFile string) (*DeviceDiscovery, error) {
	return &DeviceDiscovery{
		topologyFile: topologyFile,
	}, nil
}

// GetAllocatableDevices is called only once at NewDeviceState
func (d *DeviceDiscovery) GetAllocatableDevices() (types.AllocatableDevices, error) {
	var devices []*ghw.PCIDevice
	topo, err := topology.GetPciTopology(d.topologyFile)
	if utils.IsPseudoDeviceMode() {
		if topo != nil {
			for _, dev := range topo.GetDevices() {
				devices = append(devices,
					types.GeneratePseudoDevice(dev, types.ProductIDPf))
			}
		} else {
			klog.Warningf("cannot get PCI topology config: %v, use default pseudo devices", err)
			devices = []*ghw.PCIDevice{
				// spyre_pf devices
				types.GeneratePseudoDevice("0000:1a:00.0", types.ProductIDPf),
				types.GeneratePseudoDevice("0000:1c:00.0", types.ProductIDPf),
				types.GeneratePseudoDevice("0000:1d:00.0", types.ProductIDPf),
				types.GeneratePseudoDevice("0000:1e:00.0", types.ProductIDPf),
				types.GeneratePseudoDevice("0000:3d:00.0", types.ProductIDPf),
				types.GeneratePseudoDevice("0000:3f:00.0", types.ProductIDPf),
				types.GeneratePseudoDevice("0000:40:00.0", types.ProductIDPf),
				types.GeneratePseudoDevice("0000:41:00.0", types.ProductIDPf),
			}
		}
	} else {
		devices, err = d.discoverTargetDevices()
		if err != nil {
			return nil, fmt.Errorf("failed to discover target devices: %w", err)
		}
	}
	if len(devices) == 0 {
		klog.Warningf("discoverDevices(): no PCI device found")
	}
	return types.NewAllocatableDevices(devices, topo), nil
}

// discoverTargetDevices lists all devices from hwloc and filters only target device codes
func (dp *DeviceDiscovery) discoverTargetDevices() ([]*ghw.PCIDevice, error) {
	targetDeviceList := make([]*ghw.PCIDevice, 0)
	devices, err := listDevices()
	if err != nil {
		return targetDeviceList, fmt.Errorf("failed to list devices: %w", err)
	}
	for _, device := range devices {
		if !isSupportedDevice(device) {
			continue
		}
		vendor := device.Vendor
		vendorName := vendor.Name
		if len(vendor.Name) > maxVendorNameLen {
			vendorName = string([]byte(vendorName)[0:17]) + "..."
		}
		product := device.Product
		productName := product.Name
		if len(product.Name) > maxProductNameLen {
			productName = string([]byte(productName)[0:37]) + "..."
		}
		klog.Infof("device address: %-12s, classID: %-12s, vendor: %s (%-20s), product: %s (%-40s), driver: %s",
			device.Address, device.Class.ID, vendor.ID, vendorName, product.ID, productName, device.Driver)
		targetDeviceList = append(targetDeviceList, device)
	}
	return targetDeviceList, nil
}

// listDevices calls ghw module and lists all pci devices
func listDevices() ([]*ghw.PCIDevice, error) {
	pci, err := ghw.PCI()
	if err != nil {
		return nil, fmt.Errorf("discoverDevices(): error getting PCI info: %v", err)
	}
	devices := pci.Devices
	if len(devices) == 0 {
		klog.Warningf("discoverDevices(): no PCI device found")
	}
	return devices, nil
}

func isSupportedDevice(device *ghw.PCIDevice) bool {
	devClass, err := strconv.ParseInt(device.Class.ID, classIDBaseInt, classIDBitSize)
	if err != nil {
		klog.Warningf("failed to parse device class: %v, skip", err)
		return false
	}
	if !slices.Contains(supportedDeviceCodes, devClass) {
		return false
	}
	if !slices.Contains(supportedProductIDs, device.Product.ID) {
		return false
	}
	if !slices.Contains(supportedDrivers, device.Driver) {
		return false
	}
	if !slices.Contains(supportedVendors, device.Vendor.ID) {
		return false
	}
	return true
}
