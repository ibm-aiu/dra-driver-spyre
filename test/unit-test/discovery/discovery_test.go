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
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ibm-aiu/dra-driver-spyre/pkg/types"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/utils"
	"github.com/jaypipes/ghw"
)

var _ = Describe("main functions", func() {

	It("get allocatable devices", func() {
		allocatableDevices, err := deviceDiscovery.GetAllocatableDevices()
		Expect(err).To(BeNil())
		Expect(len(allocatableDevices)).To(Equal(24))
		numaCount := make(map[string]int)
		for _, device := range allocatableDevices {
			attrs := device.Attributes
			numa := attrs["numaInfo"]
			numaCount[*numa.StringValue] += 1
		}
		Expect(len(numaCount)).To(Equal(2))
		Expect(numaCount["0"]).To(Equal(12))
		Expect(numaCount["1"]).To(Equal(12))
	})

})

var _ = DescribeTable("managed attribute",
	func(cardMgmtEnabled bool, pciAddr string, productID types.ProductID, wantPresent bool, wantVal bool) {
		if cardMgmtEnabled {
			os.Setenv("CARD_MANAGEMENT_ENABLED", "1")
		} else {
			os.Unsetenv("CARD_MANAGEMENT_ENABLED")
		}
		DeferCleanup(os.Unsetenv, "CARD_MANAGEMENT_ENABLED")

		devName := utils.PciAddressToDeviceName(pciAddr)
		devices := types.NewAllocatableDevices([]*ghw.PCIDevice{
			types.GeneratePseudoDevice(pciAddr, productID, 0),
		}, nil, false)

		attr, present := devices[devName].Attributes[types.ManagedAttribute]
		Expect(present).To(Equal(wantPresent))
		if wantPresent {
			Expect(*attr.BoolValue).To(Equal(wantVal))
		}
	},
	// card management disabled — managed attribute must never appear
	Entry("disabled, PF 06a7 .0  → no managed attr", false, "0000:1a:00.0", types.ProductIDPf, false, false),
	Entry("disabled, VF 06a8 .1  → no managed attr", false, "0000:1a:00.1", types.ProductIDVf, false, false),
	Entry("disabled, VF 06a8 .0  → no managed attr", false, "0000:1a:00.0", types.ProductIDVf, false, false), // s390x devices
	// card management enabled — PF always managed
	Entry("enabled,  PF 06a7 .0  → managed=true", true, "0000:1a:00.0", types.ProductIDPf, true, true),
	// card management enabled — VF .1 is managed
	Entry("enabled,  VF 06a8 .1  → managed=true", true, "0000:1a:00.1", types.ProductIDVf, true, true),
	// card management enabled — VF at other function index is NOT managed
	Entry("enabled,  VF 06a8 .2  → no managed attr", true, "0000:1a:00.2", types.ProductIDVf, false, false),
)
