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
	"github.com/google/uuid"
	"github.com/ibm-aiu/dra-driver-spyre/internal/state"
	cst "github.com/ibm-aiu/dra-driver-spyre/pkg/const"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/types"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	drapbv1 "k8s.io/kubelet/pkg/apis/dra/v1beta1"
)

func makeAllocatable(pciAddresses ...string) types.AllocatableDevices {
	alloc := make(types.AllocatableDevices)
	for _, addr := range pciAddresses {
		name := utils.PciAddressToDeviceName(addr)
		pseudo := types.NewPseudoPciDevice(types.GeneratePseudoDevice(addr, types.ProductIDPf))
		alloc[name] = types.SpyreDevice{
			ProductID: types.ProductIDPf,
			PciDevice: pseudo,
		}
	}
	return alloc
}

func makeResult(driver, pool, device, request string) resourceapi.DeviceRequestAllocationResult {
	return resourceapi.DeviceRequestAllocationResult{
		Driver:  driver,
		Pool:    pool,
		Device:  device,
		Request: request,
	}
}

func makeClaimWithResults(uid string, results []resourceapi.DeviceRequestAllocationResult) *resourceapi.ResourceClaim {
	return &resourceapi.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{UID: k8stypes.UID(uid)},
		Status: resourceapi.ResourceClaimStatus{
			Allocation: &resourceapi.AllocationResult{
				Devices: resourceapi.DeviceAllocationResult{
					Results: results,
				},
			},
		},
	}
}

func newState(pciAddresses ...string) *state.DeviceState {
	// constructs a DeviceState with the provided dependencies and an empty checkpoint
	s := &state.DeviceState{
		Allocatable: makeAllocatable(pciAddresses...),
		CDI:         cdiHandler,
	}
	s.SetCheckpointManager(cpManager)
	err := s.SetNewCheckPoint()
	Expect(err).NotTo(HaveOccurred())
	return s
}

type prepareEntry struct {
	pciAddresses  []string
	results       []resourceapi.DeviceRequestAllocationResult
	nilAllocation bool
	wantErr       bool
	// assertions on the returned []*drapbv1.Device slice
	wantDeviceCount int
	wantDeviceNames []string // optional ordered check
}

var _ = Describe("Prepare", func() {

	spyrePCI := "0000:1a:00.0"
	spyrePCI2 := "0000:1b:00.0"
	spyreDevice := utils.PciAddressToDeviceName(spyrePCI)
	spyreDevice2 := utils.PciAddressToDeviceName(spyrePCI2)

	DescribeTable("driver-name filter",
		func(e prepareEntry) {
			s := newState(e.pciAddresses...)

			var claim *resourceapi.ResourceClaim
			if e.nilAllocation {
				claim = &resourceapi.ResourceClaim{
					ObjectMeta: metav1.ObjectMeta{UID: k8stypes.UID(uuid.NewString())},
				}
			} else {
				claim = makeClaimWithResults(uuid.NewString(), e.results)
			}

			devices, err := s.Prepare(claim)

			if e.wantErr {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(devices).To(HaveLen(e.wantDeviceCount))
			for i, name := range e.wantDeviceNames {
				Expect(devices[i].DeviceName).To(Equal(name))
			}
		},

		Entry("single Spyre device is prepared and returned",
			prepareEntry{
				pciAddresses: []string{spyrePCI},
				results: []resourceapi.DeviceRequestAllocationResult{
					makeResult(cst.DriverName, "pool0", spyreDevice, "req0"),
				},
				wantDeviceCount: 1,
				wantDeviceNames: []string{spyreDevice},
			},
		),

		// Foreign result with absent device name must not cause an error.
		Entry("foreign-driver result with absent device name is skipped without error",
			prepareEntry{
				pciAddresses: []string{spyrePCI},
				results: []resourceapi.DeviceRequestAllocationResult{
					makeResult("other-driver.example.com", "pool-nic", "nic-device-0", "nic-req"),
					makeResult(cst.DriverName, "pool0", spyreDevice, "req0"),
				},
				wantDeviceCount: 1,
				wantDeviceNames: []string{spyreDevice},
			},
		),

		// Allocatable must still be skipped and not appear in the returned devices.
		Entry("foreign-driver result whose device name exists locally is not prepared",
			prepareEntry{
				pciAddresses: []string{spyrePCI, spyrePCI2},
				results: []resourceapi.DeviceRequestAllocationResult{
					makeResult(cst.DriverName, "pool0", spyreDevice, "req0"),
					makeResult("other-driver.example.com", "pool-nic", spyreDevice2, "nic-req"),
				},
				wantDeviceCount: 1,
				wantDeviceNames: []string{spyreDevice},
			},
		),

		Entry("idempotent: second Prepare call with same claim UID returns same devices",
			prepareEntry{
				pciAddresses: []string{spyrePCI},
				results: []resourceapi.DeviceRequestAllocationResult{
					makeResult(cst.DriverName, "pool0", spyreDevice, "req0"),
				},
				wantDeviceCount: 1,
			},
		),

		Entry("nil Allocation returns error",
			prepareEntry{
				pciAddresses:  []string{spyrePCI},
				nilAllocation: true,
				wantErr:       true,
			},
		),

		Entry("Spyre result for unknown device returns error",
			prepareEntry{
				pciAddresses: []string{spyrePCI},
				results: []resourceapi.DeviceRequestAllocationResult{
					makeResult(cst.DriverName, "pool0", "unknown-device", "req0"),
				},
				wantErr: true,
			},
		),
	)

	It("idempotent: second Prepare call with same claim UID returns same devices", func() {
		s := newState(spyrePCI)
		claimUID := uuid.NewString()
		results := []resourceapi.DeviceRequestAllocationResult{
			makeResult(cst.DriverName, "pool0", spyreDevice, "req0"),
		}
		claim := makeClaimWithResults(claimUID, results)

		first, err := s.Prepare(claim)
		Expect(err).NotTo(HaveOccurred())
		Expect(first).To(HaveLen(1))

		second, err := s.Prepare(claim)
		Expect(err).NotTo(HaveOccurred())
		Expect(second).To(HaveLen(1))

		Expect(deviceNames(second)).To(Equal(deviceNames(first)))
	})
})

func deviceNames(devices []*drapbv1.Device) []string {
	names := make([]string, len(devices))
	for i, d := range devices {
		names[i] = d.DeviceName
	}
	return names
}
