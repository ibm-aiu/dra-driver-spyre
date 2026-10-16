/*
 * (C) Copyright IBM Corp. 2025,2026
 * SPDX-License-Identifier: Apache-2.0
 */

package types

import (
	"strconv"

	cst "github.com/ibm-aiu/dra-driver-spyre/pkg/const"
	"github.com/jaypipes/ghw"
	ghw_topology "github.com/jaypipes/ghw/pkg/topology"
	"github.com/jaypipes/pcidb"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	cdispec "tags.cncf.io/container-device-interface/specs-go"
)

type PseudoPciDevice struct {
	PciAddress string
	ProductID  string
	Numa       string
}

func NewPseudoPciDevice(dev *ghw.PCIDevice) PciDevice {
	numa := "0"
	if dev != nil && dev.Node != nil {
		numa = strconv.Itoa(dev.Node.ID)
	}
	return PseudoPciDevice{
		PciAddress: dev.Address,
		ProductID:  dev.Product.ID,
		Numa:       numa,
	}
}

// PciDevice interface
func (d PseudoPciDevice) GetPciAddr() string {
	if d.PciAddress == "" {
		d.PciAddress = "0000:0000:00:00"
	}
	return d.PciAddress
}

func (d PseudoPciDevice) GetAPIDevice() *pluginapi.Device {
	v := &pluginapi.Device{
		ID:       d.PciAddress,
		Health:   "Healthy",
		Topology: nil,
	}
	return v
}

func (d PseudoPciDevice) GetVendor() string {
	return "1014"
}

func (d PseudoPciDevice) GetDriver() string {
	return "vfio-pci"
}

func (d PseudoPciDevice) GetProductID() string {
	return d.ProductID
}

func (d PseudoPciDevice) GetPFName() string {
	return "GetPFNameResult"
}

//
// following functions won't be used.
//

func (d PseudoPciDevice) GetPfPciAddr() string {
	return "0"
}

func (d PseudoPciDevice) IsSriovPF() bool {
	return d.ProductID == string(ProductIDPf)
}

func (d PseudoPciDevice) GetSubClass() string {
	return "0"
}

func (d PseudoPciDevice) GetCDIDeviceSpec() []*cdispec.DeviceNode {
	v := []*cdispec.DeviceNode{
		{
			HostPath:    cst.VfioMount,
			Path:        cst.VfioMount,
			Permissions: cst.DeviceNodePermission,
		},
	}
	return v
}

func (d PseudoPciDevice) GetEnvVal() string {
	return d.PciAddress
}

func (d PseudoPciDevice) GetMounts() []*pluginapi.Mount {
	var v []*pluginapi.Mount
	return v
}

func (d PseudoPciDevice) GetVFID() int {
	return 0
}

func (d PseudoPciDevice) GetNumaInfo() string {
	if d.Numa != "" {
		return d.Numa
	}
	return "0"
}

func GeneratePseudoDevice(address string, productId ProductID, numaNode int) *ghw.PCIDevice {
	pId := productId
	if len(pId) == 0 {
		pId = ProductIDPf // default
	}
	var node *ghw_topology.Node
	if numaNode >= 0 {
		node = &ghw_topology.Node{
			ID: numaNode,
		}
	}
	return &ghw.PCIDevice{
		Address: address,
		Vendor:  &pcidb.Vendor{ID: "1014", Name: "IBM"},
		Product: &pcidb.Product{ID: string(pId), Name: "unknown"},
		Class:   &pcidb.Class{ID: "00"},
		Driver:  "pseudoDriver",
		Node:    node,
	}
}
