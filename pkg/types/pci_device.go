/*
 * (C) Copyright IBM Corp.  2025,2026 IBM Corp. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package types

import (
	"strconv"

	cst "github.com/ibm-aiu/dra-driver-spyre/pkg/const"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/utils"
	"github.com/jaypipes/ghw"
	klog "k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	cdispec "tags.cncf.io/container-device-interface/specs-go"
)

// pciDevice common abstract for Spyre PCI device
type pciDevice struct {
	basePciDevice *ghw.PCIDevice
	pfAddr        string
	driver        string
	vfID          int
	numa          string
	apiDevice     *pluginapi.Device
}

// Convert NUMA node number to string.
// A node of -1 represents "unknown" and is converted to the empty string.
func nodeToStr(nodeNum int) string {
	if nodeNum >= 0 {
		return strconv.Itoa(nodeNum)
	}
	return ""
}

// NewPciSpyreDevice returns an instance of PciDevice interface
func NewPciSpyreDevice(dev *ghw.PCIDevice) (PciDevice, error) {
	pciAddr := dev.Address

	// Get PF PCI address
	pfAddr, err := utils.GetPfAddr(pciAddr)
	if err != nil {
		return nil, err
	}

	// Get driver info
	driverName, err := utils.GetDriverName(pciAddr)
	if err != nil {
		return nil, err
	}

	vfID, err := utils.GetVFID(pciAddr)
	if err != nil {
		return nil, err
	}

	nodeNum := utils.GetDevNode(pciAddr)
	apiDevice := &pluginapi.Device{
		ID:     pciAddr,
		Health: pluginapi.Healthy,
	}
	if nodeNum >= 0 {
		numaInfo := &pluginapi.NUMANode{
			ID: int64(nodeNum),
		}
		apiDevice.Topology = &pluginapi.TopologyInfo{
			Nodes: []*pluginapi.NUMANode{numaInfo},
		}
	}

	// 	Create pciAiuDevice object with all relevant info
	return &pciDevice{
		basePciDevice: dev,
		pfAddr:        pfAddr,
		driver:        driverName,
		vfID:          vfID,
		apiDevice:     apiDevice,
		numa:          nodeToStr(nodeNum),
	}, nil
}

func (pd *pciDevice) GetPfPciAddr() string {
	return pd.pfAddr
}

func (pd *pciDevice) GetVendor() string {
	return pd.basePciDevice.Vendor.ID
}

func (pd *pciDevice) GetProductID() string {
	return pd.basePciDevice.Product.ID
}

func (pd *pciDevice) GetPciAddr() string {
	return pd.basePciDevice.Address
}

func (pd *pciDevice) GetDriver() string {
	return pd.driver
}

func (pd *pciDevice) IsSriovPF() bool {
	return false
}

func (pd *pciDevice) GetSubClass() string {
	return pd.basePciDevice.Subclass.ID
}

func (pd *pciDevice) GetCDIDeviceSpec() []*cdispec.DeviceNode {
	devSpecs := make([]*cdispec.DeviceNode, 0)
	devSpecs = append(devSpecs, &cdispec.DeviceNode{
		HostPath:    cst.VfioMount,
		Path:        cst.VfioMount,
		Permissions: "mrw",
	})
	pciAddr := pd.GetPciAddr()
	vfioDevHost, vfioDevContainer, err := utils.GetVFIODeviceFile(pciAddr)
	if err != nil {
		klog.Errorf("GetCDIDeviceSpec(): error getting vfio device file for device: %s, %s", pciAddr, err.Error())
	} else {
		devSpecs = append(devSpecs, &cdispec.DeviceNode{
			HostPath:    vfioDevHost,
			Path:        vfioDevContainer,
			Permissions: "mrw",
		})
	}
	return devSpecs
}

func (pd *pciDevice) GetEnvVal() string {
	return pd.GetPciAddr()
}

func (pd *pciDevice) GetMounts() []*pluginapi.Mount {
	mnt := make([]*pluginapi.Mount, 0)
	return mnt
}

func (pd *pciDevice) GetAPIDevice() *pluginapi.Device {
	return pd.apiDevice
}

func (pd *pciDevice) GetVFID() int {
	return pd.vfID
}

func (pd *pciDevice) GetNumaInfo() string {
	return pd.numa
}
