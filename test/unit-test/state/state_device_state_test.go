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

package state_test

import (
	"github.com/ibm-aiu/dra-driver-spyre/pkg/types"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	resourceapi "k8s.io/api/resource/v1"
)

var _ = Describe("Device state attribute", func() {
	const (
		addr1 = "0001:00:00.0"
		addr2 = "0002:00:00.0"
	)

	stateOf := func(devices []resourceapi.Device, addr string) string {
		name := utils.PciAddressToDeviceName(addr)
		for _, d := range devices {
			if d.Name == name {
				attr, ok := d.Attributes[types.DeviceStateAttribute]
				Expect(ok).To(BeTrue(), "device %s has no %s attribute", name, types.DeviceStateAttribute)
				Expect(attr.StringValue).NotTo(BeNil())
				return *attr.StringValue
			}
		}
		Fail("device not found: " + name)
		return ""
	}

	// NewAllocatableDevices sets the default attribute; the test helper bypasses it,
	// so seed it the same way the driver does.
	seeded := func(addrs ...string) types.AllocatableDevices {
		alloc := makeAllocatable(addrs...)
		for name, d := range alloc {
			d.Name = name
			d.Attributes = map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
				types.DeviceStateAttribute: types.GetStringDeviceAttribute(types.DeviceStateOnline),
			}
			alloc[name] = d
		}
		return alloc
	}

	It("updates only the devices reported and returns whether anything changed", func() {
		s := newState()
		s.Allocatable = seeded(addr1, addr2)

		Expect(s.UpdateDeviceStates(map[string]string{addr2: "IN_ERROR"})).To(BeTrue())
		devices := s.Devices()
		Expect(stateOf(devices, addr1)).To(Equal(types.DeviceStateOnline))
		Expect(stateOf(devices, addr2)).To(Equal("IN_ERROR"))

		By("reporting the same value again")
		Expect(s.UpdateDeviceStates(map[string]string{addr2: "IN_ERROR"})).To(BeFalse())

		By("reporting an unknown device")
		Expect(s.UpdateDeviceStates(map[string]string{"0009:00:00.0": "IN_ERROR"})).To(BeFalse())
	})

	It("does not mutate a snapshot returned before the update", func() {
		s := newState()
		s.Allocatable = seeded(addr1)

		before := s.Devices()
		Expect(s.UpdateDeviceStates(map[string]string{addr1: "IN_ERROR"})).To(BeTrue())

		Expect(stateOf(before, addr1)).To(Equal(types.DeviceStateOnline))
		Expect(stateOf(s.Devices(), addr1)).To(Equal("IN_ERROR"))
	})

	It("recovers to ONLINE", func() {
		s := newState()
		s.Allocatable = seeded(addr1)

		Expect(s.UpdateDeviceStates(map[string]string{addr1: "IN_ERROR"})).To(BeTrue())
		Expect(s.UpdateDeviceStates(map[string]string{addr1: types.DeviceStateOnline})).To(BeTrue())
		Expect(stateOf(s.Devices(), addr1)).To(Equal(types.DeviceStateOnline))
	})
})
