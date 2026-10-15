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

package pcitopo

// TierGroups holds per-tier group ID maps for all devices in a topology.
// Each map entry is: pciAddress -> groupID (string).
// Devices sharing the same group ID at a given tier can be used together for
// topology-aware (TP) workloads at that tier level.
//
//   - Tier0: devices connected via the same PCI switch (peers_0 set)
//   - Tier1: devices reachable within one additional PCI hop (peers_0 ∪ peers_1)
//   - Tier2: devices reachable within two additional PCI hops (peers_0 ∪ peers_1 ∪ peers_2)
type TierGroups struct {
	Tier0 map[string]string // pciAddress -> tier0 group ID
	Tier1 map[string]string // pciAddress -> tier1 group ID
	Tier2 map[string]string // pciAddress -> tier2 group ID
}

// peerSets holds the three peer maps for a single device.
type peerSets struct {
	p0, p1, p2 map[string]int
}

// GenerateTierGroupsFromTopo computes tier0/tier1/tier2 group IDs for every
// device in the topology. Two devices receive the same group ID at tier N when
// they belong to the same connected component in the graph formed by:
//
//   - tier0: peers_0 edges only
//   - tier1: peers_0 ∪ peers_1 edges
//   - tier2: peers_0 ∪ peers_1 ∪ peers_2 edges
//
// The function handles both PF topologies (topo.Devices) and VF topologies
// (topo.SpyreVfDevices). For VF devices the peer information is read from
// SpyreVfPeers rather than Peers.
func GenerateTierGroupsFromTopo(topo Pcitopo) TierGroups {
	adjacency := buildAdjacency(topo)

	allAddrs := make([]string, 0, len(adjacency))
	for addr := range adjacency {
		allAddrs = append(allAddrs, addr)
	}

	return TierGroups{
		Tier0: connectedComponents(allAddrs, adjacency, 0),
		Tier1: connectedComponents(allAddrs, adjacency, 1),
		Tier2: connectedComponents(allAddrs, adjacency, 2),
	}
}

// buildAdjacency builds the peer-set map for every device in the topology.
func buildAdjacency(topo Pcitopo) map[string]peerSets {
	adjacency := make(map[string]peerSets, len(topo.Devices)+len(topo.SpyreVfDevices))
	for addr, dev := range topo.Devices {
		adjacency[addr] = peerSets{dev.Peers.Peer0, dev.Peers.Peer1, dev.Peers.Peer2}
	}
	for addr, dev := range topo.SpyreVfDevices {
		adjacency[addr] = peerSets{dev.SpyreVfPeers.Peer0, dev.SpyreVfPeers.Peer1, dev.SpyreVfPeers.Peer2}
	}
	return adjacency
}

// peersUpToTier returns the peer maps that are active at the given tier level.
func peersUpToTier(ps peerSets, maxTier int) []map[string]int {
	peers := []map[string]int{ps.p0}
	if maxTier >= 1 {
		peers = append(peers, ps.p1)
	}
	if maxTier >= 2 {
		peers = append(peers, ps.p2)
	}
	return peers
}

// connectedComponents runs a union-find over allAddrs using peer edges up to
// maxTier (0=peers_0 only, 1=peers_0+peers_1, 2=all three peer sets).
// Returns a map of pciAddress -> canonical group representative (string).
// The canonical representative is the lexicographically smallest address in
// the group, making the result deterministic regardless of iteration order.
func connectedComponents(allAddrs []string, adjacency map[string]peerSets, maxTier int) map[string]string {
	uf := newUnionFind(allAddrs)
	for _, addr := range allAddrs {
		for _, peers := range peersUpToTier(adjacency[addr], maxTier) {
			for neighbour := range peers {
				uf.union(addr, neighbour)
			}
		}
	}
	return uf.groups()
}

// unionFind implements a union-find (disjoint-set) structure with path
// compression and lexicographic-minimum root selection for determinism.
type unionFind struct {
	parent map[string]string
}

func newUnionFind(addrs []string) *unionFind {
	parent := make(map[string]string, len(addrs))
	for _, a := range addrs {
		parent[a] = a
	}
	return &unionFind{parent: parent}
}

func (uf *unionFind) find(x string) string {
	if uf.parent[x] != x {
		uf.parent[x] = uf.find(uf.parent[x]) // path compression
	}
	return uf.parent[x]
}

func (uf *unionFind) union(a, b string) {
	if _, ok := uf.parent[b]; !ok {
		return // neighbour not in the known device set; skip
	}
	ra, rb := uf.find(a), uf.find(b)
	if ra == rb {
		return
	}
	// Use lexicographic minimum as root for determinism.
	if ra < rb {
		uf.parent[rb] = ra
	} else {
		uf.parent[ra] = rb
	}
}

func (uf *unionFind) groups() map[string]string {
	result := make(map[string]string, len(uf.parent))
	for a := range uf.parent {
		result[a] = uf.find(a)
	}
	return result
}
