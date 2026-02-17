/*
 * Copyright (c) 2022 IBM Corp. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package types

import (
	cst "github.com/ibm-aiu/dra-driver-spyre/pkg/const"
	"github.com/jaypipes/ghw"
	"github.com/jaypipes/pcidb"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	cdispec "tags.cncf.io/container-device-interface/specs-go"
)

type PseudoPciDevice struct {
	PciAddress string
	ProductID  string
}

func NewPseudoPciDevice(dev *ghw.PCIDevice) PciDevice {
	return PseudoPciDevice{
		PciAddress: dev.Address,
		ProductID:  dev.Product.ID,
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
		ID:                   d.PciAddress,
		Health:               "Healthy",
		Topology:             nil,
		XXX_NoUnkeyedLiteral: struct{}{},
		XXX_sizecache:        0,
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
	return true
}

func (d PseudoPciDevice) GetSubClass() string {
	return "0"
}

func (d PseudoPciDevice) GetCDIDeviceSpec() []*cdispec.DeviceNode {
	v := []*cdispec.DeviceNode{
		{
			HostPath:    cst.VfioMount,
			Path:        cst.VfioMount,
			Permissions: "mrw",
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
	return "0"
}

func GeneratePseudoDevice(address string, productId ProductID) *ghw.PCIDevice {
	pId := productId
	if len(pId) == 0 {
		pId = ProductIDPf // default
	}
	return &ghw.PCIDevice{
		Address: address,
		Vendor:  &pcidb.Vendor{ID: "1014", Name: "IBM"},
		Product: &pcidb.Product{ID: string(pId), Name: "unknown"},
		Class:   &pcidb.Class{ID: "00"},
		Driver:  "pseudoDriver",
	}
}
