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

package discovery

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("main functions", func() {

	It("get allocatable devices", func() {
		allocatableDevices, err := deviceDiscovery.GetAllocatableDevices()
		Expect(err).To(BeNil())
		Expect(len(allocatableDevices)).To(Equal(8))
		numaCount := make(map[string]int)
		for _, device := range allocatableDevices {
			attrs := device.Basic.Attributes
			numa := attrs["numaInfo"]
			numaCount[*numa.StringValue] += 1
		}
		Expect(len(numaCount)).To(Equal(2))
		Expect(numaCount["0"]).To(Equal(4))
		Expect(numaCount["1"]).To(Equal(4))
	})

})
