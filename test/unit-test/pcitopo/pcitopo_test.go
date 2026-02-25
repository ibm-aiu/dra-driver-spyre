// (C) Copyright IBM Corp.  2025,2026 IBM Corp.
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

package pcitopo_test

import (
	"encoding/json"
	"os"

	"github.com/ibm-aiu/dra-driver-spyre/pkg/topology"
	. "github.com/ibm-aiu/dra-driver-spyre/pkg/topology/types/pcitopo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	pfTopoFilepath = "../../data/topo/with-pf.json"
	vfTopoFilepath = "../../data/topo/with-vf.json"
)

var _ = Describe("Test Topology", func() {

	Context("topology", func() {

		It("can read pcitopo file containing PF devices", func() {
			s, err := os.ReadFile(pfTopoFilepath)
			Expect(err).To(BeNil())
			Expect(s).NotTo(BeNil())
			var pcitopo Pcitopo
			err = json.Unmarshal(s, &pcitopo)
			Expect(err).To(BeNil())
			Expect(pcitopo.Devices).Should(HaveLen(pcitopo.NumDevices))
			for _, v := range pcitopo.Devices {
				Expect(v.Peers.Peer0).ShouldNot(BeEmpty())
			}
		})

		It("can read pcitopo file containing VF devices", func() {
			s, err := os.ReadFile(vfTopoFilepath)
			Expect(err).To(BeNil())
			Expect(s).NotTo(BeNil())
			var pcitopo Pcitopo
			err = json.Unmarshal(s, &pcitopo)
			Expect(err).To(BeNil())
			Expect(pcitopo.SpyreVfDevices).Should(HaveLen(pcitopo.SpyreVfNumDevices))
			for _, v := range pcitopo.SpyreVfDevices {
				Expect(v.SpyreVfPeers.Peer0).ShouldNot(BeEmpty())
			}
		})

		It("can unmarshal pseudo topology", func() {
			s := []byte(topology.PseudoTopology)
			pcitopo, err := UnmarshalPciTopo(s)
			Expect(err).To(BeNil())
			Expect(pcitopo.Devices).To(HaveLen(8))
		})
	})
})
