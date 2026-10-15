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

package pcitopo_test

import (
	"os"

	. "github.com/ibm-aiu/dra-driver-spyre/pkg/topology/types/pcitopo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// tierGroupsTC is the input/output spec for a single DescribeTable entry.
type tierGroupsTC struct {
	topoFile string
	// allAddrs is the complete list of PCI addresses expected in every tier map.
	// Its length is used to confirm map lengths at all three tiers.
	allAddrs []string
	// expected number of *distinct* group IDs at each tier
	wantTier0Groups int
	wantTier1Groups int
	wantTier2Groups int
	// tier0SameGroup: all addrs must share the same tier0 group ID
	tier0SameGroup []string
	// tier1SameGroup: all addrs must share the same tier1 group ID
	tier1SameGroup []string
	// tier2SameGroup: all addrs must share the same tier2 group ID
	tier2SameGroup []string
	// tier0DiffGroups: each inner slice is a set of addrs that must all have
	// *different* tier0 group IDs from one another (one representative per group).
	tier0DiffGroups [][]string
	// tier1DiffGroups: same semantics for tier1.
	tier1DiffGroups [][]string
}

var _ = DescribeTable("GenerateTierGroupsFromTopo",
	func(tc tierGroupsTC) {
		var topo Pcitopo
		if tc.topoFile != "" {
			s, err := os.ReadFile(tc.topoFile)
			Expect(err).To(BeNil())
			topo, err = UnmarshalPciTopo(s)
			Expect(err).To(BeNil())
		}

		groups := GenerateTierGroupsFromTopo(topo)

		By("checking map lengths match the number of known devices")
		Expect(groups.Tier0).To(HaveLen(len(tc.allAddrs)))
		Expect(groups.Tier1).To(HaveLen(len(tc.allAddrs)))
		Expect(groups.Tier2).To(HaveLen(len(tc.allAddrs)))

		By("confirming every device address is a key in all three tier maps")
		for _, addr := range tc.allAddrs {
			Expect(groups.Tier0).To(HaveKey(addr), "tier0 missing address %s", addr)
			Expect(groups.Tier1).To(HaveKey(addr), "tier1 missing address %s", addr)
			Expect(groups.Tier2).To(HaveKey(addr), "tier2 missing address %s", addr)
		}

		By("checking distinct group counts")
		Expect(distinctValues(groups.Tier0)).To(Equal(tc.wantTier0Groups))
		Expect(distinctValues(groups.Tier1)).To(Equal(tc.wantTier1Groups))
		Expect(distinctValues(groups.Tier2)).To(Equal(tc.wantTier2Groups))

		By("checking tier0 co-group addresses")
		if len(tc.tier0SameGroup) > 1 {
			ref := groups.Tier0[tc.tier0SameGroup[0]]
			Expect(ref).NotTo(BeEmpty())
			for _, addr := range tc.tier0SameGroup[1:] {
				Expect(groups.Tier0[addr]).To(Equal(ref), "addr %s should share tier0 group with %s", addr, tc.tier0SameGroup[0])
			}
		}

		By("checking tier1 co-group addresses")
		if len(tc.tier1SameGroup) > 1 {
			ref := groups.Tier1[tc.tier1SameGroup[0]]
			Expect(ref).NotTo(BeEmpty())
			for _, addr := range tc.tier1SameGroup[1:] {
				Expect(groups.Tier1[addr]).To(Equal(ref), "addr %s should share tier1 group with %s", addr, tc.tier1SameGroup[0])
			}
		}

		By("checking tier2 co-group addresses")
		if len(tc.tier2SameGroup) > 1 {
			ref := groups.Tier2[tc.tier2SameGroup[0]]
			Expect(ref).NotTo(BeEmpty())
			for _, addr := range tc.tier2SameGroup[1:] {
				Expect(groups.Tier2[addr]).To(Equal(ref), "addr %s should share tier2 group with %s", addr, tc.tier2SameGroup[0])
			}
		}

		By("confirming addresses from different tier0 groups get different IDs")
		for _, representatives := range tc.tier0DiffGroups {
			for i := 0; i < len(representatives); i++ {
				for j := i + 1; j < len(representatives); j++ {
					Expect(groups.Tier0[representatives[i]]).NotTo(
						Equal(groups.Tier0[representatives[j]]),
						"addr %s and %s should be in different tier0 groups",
						representatives[i], representatives[j],
					)
				}
			}
		}

		By("confirming addresses from different tier1 groups get different IDs")
		for _, representatives := range tc.tier1DiffGroups {
			for i := 0; i < len(representatives); i++ {
				for j := i + 1; j < len(representatives); j++ {
					Expect(groups.Tier1[representatives[i]]).NotTo(
						Equal(groups.Tier1[representatives[j]]),
						"addr %s and %s should be in different tier1 groups",
						representatives[i], representatives[j],
					)
				}
			}
		}

		By("verifying tier group counts are monotonically non-increasing")
		Expect(distinctValues(groups.Tier1)).To(BeNumerically("<=", distinctValues(groups.Tier0)))
		Expect(distinctValues(groups.Tier2)).To(BeNumerically("<=", distinctValues(groups.Tier1)))

		By("verifying results are deterministic")
		groups2 := GenerateTierGroupsFromTopo(topo)
		Expect(groups.Tier0).To(Equal(groups2.Tier0))
		Expect(groups.Tier1).To(Equal(groups2.Tier1))
		Expect(groups.Tier2).To(Equal(groups2.Tier2))
	},

	// ── PF topology (with-pf.json) ────────────────────────────────────────────
	// 8 PF devices:
	//   Tier0 groups (3): {29,2a,2b} | {3b,3c} | {aa,ab,ac}
	//   Tier1 groups (2): {29,2a,2b,3b,3c} | {aa,ab,ac}   (29's peers_1={3b,3c})
	//   Tier2 groups (1): all 8 merged via peers_2
	Entry("PF topology: tier groups derived from peers_0/1/2",
		tierGroupsTC{
			topoFile: pfTopoFilepath,
			// All 8 PF device addresses present in with-pf.json.
			allAddrs: []string{
				"0000:29:00.0", "0000:2a:00.0", "0000:2b:00.0",
				"0000:3b:00.0", "0000:3c:00.0",
				"0000:aa:00.0", "0000:ab:00.0", "0000:ac:00.0",
			},
			wantTier0Groups: 3,
			wantTier1Groups: 2,
			wantTier2Groups: 1,
			// peers_0 of 0000:29:00.0 = {2a, 2b} → same tier0 group
			tier0SameGroup: []string{"0000:29:00.0", "0000:2a:00.0", "0000:2b:00.0"},
			// 29's peers_1 = {3b, 3c} → same tier1 group as 29
			tier1SameGroup: []string{"0000:29:00.0", "0000:3b:00.0", "0000:3c:00.0"},
			// peers_2 bridges both halves → all in one tier2 group
			tier2SameGroup: []string{"0000:29:00.0", "0000:aa:00.0", "0000:ab:00.0"},
			// one representative per tier0 group — all three must differ
			tier0DiffGroups: [][]string{
				{"0000:29:00.0", "0000:3b:00.0", "0000:aa:00.0"},
			},
			// one representative per tier1 group — the two halves must differ
			tier1DiffGroups: [][]string{
				{"0000:29:00.0", "0000:aa:00.0"},
			},
		},
	),

	// ── VF topology (with-vf.json) ────────────────────────────────────────────
	// 13 VF devices (spyre_vf_peers used):
	//   Tier0 groups (4): {2a,2b.1,2b.2} | {3b,3c} | {aa.1,aa.2,ac.1,ac.2} | {ba.1,ba.2,bb.1,bb.2}
	//   Tier1 groups (2): {2a,2b.1,2b.2,3b,3c} | {aa.1,aa.2,ac.1,ac.2,ba.1,ba.2,bb.1,bb.2}
	//   Tier2 groups (1): all 13 merged via peers_2
	Entry("VF topology: tier groups derived from spyre_vf_peers_0/1/2",
		tierGroupsTC{
			topoFile: vfTopoFilepath,
			// All 13 VF device addresses present in with-vf.json.
			allAddrs: []string{
				"0000:2a:00.2", "0000:2b:00.1", "0000:2b:00.2",
				"0000:3b:00.2", "0000:3c:00.2",
				"0000:aa:00.1", "0000:aa:00.2", "0000:ac:00.1", "0000:ac:00.2",
				"0000:ba:00.1", "0000:ba:00.2", "0000:bb:00.1", "0000:bb:00.2",
			},
			wantTier0Groups: 4,
			wantTier1Groups: 2,
			wantTier2Groups: 1,
			// spyre_vf_peers_0 of 2a = {2b.1, 2b.2}
			tier0SameGroup: []string{"0000:2a:00.2", "0000:2b:00.1", "0000:2b:00.2"},
			// 2a's peers_1 = {3b, 3c} → same tier1 group
			tier1SameGroup: []string{"0000:2a:00.2", "0000:3b:00.2", "0000:3c:00.2"},
			// peers_2 bridges both halves → all in one tier2 group
			tier2SameGroup: []string{"0000:2a:00.2", "0000:aa:00.1", "0000:ba:00.1"},
			// one representative per tier0 group — all four must differ
			tier0DiffGroups: [][]string{
				{"0000:2a:00.2", "0000:3b:00.2", "0000:aa:00.1", "0000:ba:00.1"},
			},
			// one representative per tier1 group — the two halves must differ
			tier1DiffGroups: [][]string{
				{"0000:2a:00.2", "0000:aa:00.1"},
			},
		},
	),

	// ── Empty topology ────────────────────────────────────────────────────────
	Entry("empty topology: returns empty maps without panicking",
		tierGroupsTC{
			topoFile: "", // no file — Pcitopo{} used directly
			allAddrs: []string{},
		},
	),
)

// distinctValues counts the number of unique values in a string->string map.
func distinctValues(m map[string]string) int {
	seen := make(map[string]struct{})
	for _, v := range m {
		seen[v] = struct{}{}
	}
	return len(seen)
}
