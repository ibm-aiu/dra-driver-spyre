/*
 * Copyright 2024 The IBM Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package types

import (
	resourceapi "k8s.io/api/resource/v1beta1"
	drapbv1 "k8s.io/kubelet/pkg/apis/dra/v1beta1"
	"k8s.io/utils/ptr"
	cdispec "tags.cncf.io/container-device-interface/specs-go"

	configapi "github.com/ibm-aiu/dra-driver-spyre/api/ibm.com/resource/spyre/v1alpha1"
)

type PreparedDevice struct {
	drapbv1.Device
	Config        configapi.SpyreConfig
	PciAddress    string
	CDIDeviceSpec []*cdispec.DeviceNode
}

type PreparedDevices []*PreparedDevice
type PreparedClaims map[string]PreparedDevices

type PciDevice interface {
	GetVendor() string
	GetDriver() string
	GetProductID() string
	GetPciAddr() string
	GetPfPciAddr() string
	IsSriovPF() bool
	GetSubClass() string
	GetEnvVal() string
	GetCDIDeviceSpec() []*cdispec.DeviceNode
	GetVFID() int
	GetNumaInfo() string
}

func (p *PreparedDevice) ApplyConfig() error {
	return nil
}

// GetDevices extracts the list of drapbv1.Devices from PreparedDevices.
func (pds PreparedDevices) GetDevices() []*drapbv1.Device {
	devices := []*drapbv1.Device{}
	for _, pd := range pds {
		devices = append(devices, &pd.Device)
	}
	return devices
}

func GetStringDeviceAttribute(value string) resourceapi.DeviceAttribute {
	return resourceapi.DeviceAttribute{StringValue: ptr.To(value)}
}

func GetBoolDeviceAttribute(value bool) resourceapi.DeviceAttribute {
	return resourceapi.DeviceAttribute{BoolValue: ptr.To(value)}
}

func GetIntDeviceAttribute(value int) resourceapi.DeviceAttribute {
	return resourceapi.DeviceAttribute{IntValue: ptr.To(int64(value))}
}
