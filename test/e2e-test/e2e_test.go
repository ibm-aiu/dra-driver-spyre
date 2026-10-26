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

package e2e_test

import (
	"context"

	"github.com/ibm-aiu/dra-driver-spyre/test/testutil"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
)

// tierAllocationTC is the shared input spec for all tier-aware DescribeTable entries.
type tierAllocationTC struct {
	claimNameSuffix string      // unique suffix — avoids name collisions between entries
	isVF            bool        // true → VF allocation; false → PF
	tier            int         // 0, 1, or 2
	count           int         // number of devices requested
	expectedPhase   v1.PodPhase // expected pod phase after scheduling
}

var _ = Describe("e2e test", Ordered, func() {

	Context("pseudo mode", func() {
		DescribeTable("simple allocation", func(count int, expectedPhase v1.PodPhase) {
			ctx := context.Background()
			claimName := "test-claim"
			podName := "test-pod"
			testNamespace := testutil.CreateRandomTestNamespace(ctx, k8sClientset)
			By("deploying ResourceClaimTemplate")
			claimData := testutil.BasicResourceClaimTemplateData(claimName, testNamespace).SetCount(count)
			testutil.BuildResourceClaimTemplate(ctx, dynClient, discoClient, claimData)
			By("deploying Pod")
			podData := testutil.BasicPodTemplateData(podName, testNamespace).SetArg0(testutil.PrintSenlibConfig)
			testutil.BuildPod(ctx, dynClient, discoClient, podData, claimName)
			testutil.CheckPodPhases(ctx, k8sClientset, []*testutil.PodTemplateData{podData}, map[v1.PodPhase]int{expectedPhase: 1})
			if expectedPhase == v1.PodRunning {
				allocations := testutil.CheckAndGetAllocationsFromPodLog(ctx, k8sClientset, podName, testNamespace, claimData.IsVF())
				Expect(allocations).To(HaveLen(count))
			}
			By("deleting pod")
			testutil.DeletePod(ctx, k8sClientset, podData)
		}, Entry("one device", 1, v1.PodRunning),
			Entry("two devices", 2, v1.PodRunning),
			Entry("four devices", 4, v1.PodRunning),
			Entry("all devices", 8, v1.PodRunning),
			Entry("over availability", 9, v1.PodPending),
		)
	})

	Context("pseudo mode - VF", func() {
		DescribeTable("simple allocation", func(count int, expectedPhase v1.PodPhase) {
			ctx := context.Background()
			claimName := "test-claim-vf"
			podName := "test-pod-vf"
			testNamespace := testutil.CreateRandomTestNamespace(ctx, k8sClientset)
			By("deploying ResourceClaimTemplate")
			claimData := testutil.BasicResourceClaimTemplateData(claimName, testNamespace).
				SetCount(count).
				SetProductId(testutil.VfProductId)
			testutil.BuildResourceClaimTemplate(ctx, dynClient, discoClient, claimData)
			By("deploying Pod")
			podData := testutil.BasicPodTemplateData(podName, testNamespace).SetArg0(testutil.PrintSenlibConfig)
			testutil.BuildPod(ctx, dynClient, discoClient, podData, claimName)
			testutil.CheckPodPhases(ctx, k8sClientset, []*testutil.PodTemplateData{podData}, map[v1.PodPhase]int{expectedPhase: 1})
			if expectedPhase == v1.PodRunning {
				allocations := testutil.CheckAndGetAllocationsFromPodLog(ctx, k8sClientset, podName, testNamespace, claimData.IsVF())
				Expect(allocations).To(HaveLen(count))
			}
			By("deleting pod")
			testutil.DeletePod(ctx, k8sClientset, podData)
		}, Entry("one VF device", 1, v1.PodRunning),
			Entry("four VF devices", 4, v1.PodRunning),
			Entry("all VF devices", 29, v1.PodRunning),
			Entry("over availability", 30, v1.PodPending),
		)
	})

	Context("pseudo mode - tier allocation", func() {
		DescribeTable("allocates devices in the same tier group",
			func(tc tierAllocationTC) {
				ctx := context.Background()
				claimName := "tier-claim-" + tc.claimNameSuffix
				podName := "tier-pod-" + tc.claimNameSuffix
				testNamespace := testutil.CreateRandomTestNamespace(ctx, k8sClientset)

				By("deploying tier ResourceClaimTemplate")
				claimData := testutil.BasicResourceClaimTemplateData(claimName, testNamespace).
					SetCount(tc.count)
				if tc.isVF {
					claimData.SetVfTierAllocation(tc.tier)
				} else {
					claimData.SetPfTierAllocation(tc.tier)
				}
				testutil.BuildResourceClaimTemplate(ctx, dynClient, discoClient, claimData)

				By("deploying Pod")
				podData := testutil.BasicPodTemplateData(podName, testNamespace).
					SetArg0(testutil.PrintSenlibConfig)
				testutil.BuildPod(ctx, dynClient, discoClient, podData, claimName)

				testutil.CheckPodPhases(ctx, k8sClientset,
					[]*testutil.PodTemplateData{podData},
					map[v1.PodPhase]int{tc.expectedPhase: 1})

				if tc.expectedPhase == v1.PodRunning {
					allocations := testutil.CheckAndGetAllocationsFromPodLog(
						ctx, k8sClientset, podName, testNamespace, tc.isVF)
					Expect(allocations).To(HaveLen(tc.count))

					By("verifying all allocated devices share the same tier group")
					testutil.CheckAllocationsShareTierGroup(ctx, k8sClientset, allocations, tc.tier)
				}

				By("deleting pod")
				testutil.DeletePod(ctx, k8sClientset, podData)
			},

			// ── PF tier0 (same PCI switch, peers_0) ──────────────────────────
			// Pseudo PF topo: {1a,1c,1d,1e} and {3d,3f,40,41} → 4 per tier0 group
			Entry("pf-tier0: 1 device", tierAllocationTC{
				claimNameSuffix: "pf-t0-1", tier: 0, count: 1, expectedPhase: v1.PodRunning,
			}),
			Entry("pf-tier0: 2 devices", tierAllocationTC{
				claimNameSuffix: "pf-t0-2", tier: 0, count: 2, expectedPhase: v1.PodRunning,
			}),
			Entry("pf-tier0: 4 devices (full group)", tierAllocationTC{
				claimNameSuffix: "pf-t0-4", tier: 0, count: 4, expectedPhase: v1.PodRunning,
			}),
			Entry("pf-tier0: 5 devices (exceeds group size, pending)", tierAllocationTC{
				claimNameSuffix: "pf-t0-5", tier: 0, count: 5, expectedPhase: v1.PodPending,
			}),

			// ── PF tier1 (≤1 PCI hop, peers_0 ∪ peers_1) ────────────────────
			// Pseudo PF topo: all 8 PF devices are tier1-reachable from each other
			Entry("pf-tier1: 2 devices", tierAllocationTC{
				claimNameSuffix: "pf-t1-2", tier: 1, count: 2, expectedPhase: v1.PodRunning,
			}),
			Entry("pf-tier1: 4 devices", tierAllocationTC{
				claimNameSuffix: "pf-t1-4", tier: 1, count: 4, expectedPhase: v1.PodRunning,
			}),
			Entry("pf-tier1: 8 devices (all PF devices)", tierAllocationTC{
				claimNameSuffix: "pf-t1-8", tier: 1, count: 8, expectedPhase: v1.PodRunning,
			}),
			Entry("pf-tier1: 9 devices (over availability, pending)", tierAllocationTC{
				claimNameSuffix: "pf-t1-9", tier: 1, count: 9, expectedPhase: v1.PodPending,
			}),

			// ── PF tier2 (≤2 PCI hops, peers_0 ∪ peers_1 ∪ peers_2) ─────────
			// Pseudo PF topo: all 8 PF devices are tier2-reachable
			Entry("pf-tier2: 4 devices", tierAllocationTC{
				claimNameSuffix: "pf-t2-4", tier: 2, count: 4, expectedPhase: v1.PodRunning,
			}),
			Entry("pf-tier2: 8 devices (all PF devices)", tierAllocationTC{
				claimNameSuffix: "pf-t2-8", tier: 2, count: 8, expectedPhase: v1.PodRunning,
			}),

			// ── VF tier0 (same PCI switch, spyre_vf_peers_0) ─────────────────
			// Pseudo VF topo: {2a,2b.1,2b.2}→3, {3b,3c}→2,
			//                 {aa.1,aa.2,ac.1,ac.2}→4, {ba.1,ba.2,bb.1,bb.2}→4
			Entry("vf-tier0: 1 device", tierAllocationTC{
				claimNameSuffix: "vf-t0-1", isVF: true, tier: 0, count: 1, expectedPhase: v1.PodRunning,
			}),
			Entry("vf-tier0: 2 devices", tierAllocationTC{
				claimNameSuffix: "vf-t0-2", isVF: true, tier: 0, count: 2, expectedPhase: v1.PodRunning,
			}),
			Entry("vf-tier0: 5 devices (exceeds largest group size, pending)", tierAllocationTC{
				claimNameSuffix: "vf-t0-5", isVF: true, tier: 0, count: 5, expectedPhase: v1.PodPending,
			}),

			// ── VF tier1 (≤1 PCI hop, spyre_vf_peers_0 ∪ peers_1) ───────────
			// Pseudo VF topo: {2a,2b.1,2b.2,3b,3c}→5, {aa.1,aa.2,ac.1,ac.2,ba.1,ba.2,bb.1,bb.2}→8
			Entry("vf-tier1: 2 devices", tierAllocationTC{
				claimNameSuffix: "vf-t1-2", isVF: true, tier: 1, count: 2, expectedPhase: v1.PodRunning,
			}),
			Entry("vf-tier1: 4 devices", tierAllocationTC{
				claimNameSuffix: "vf-t1-4", isVF: true, tier: 1, count: 4, expectedPhase: v1.PodRunning,
			}),
			Entry("vf-tier1: 9 devices (over availability, pending)", tierAllocationTC{
				claimNameSuffix: "vf-t1-9", isVF: true, tier: 1, count: 9, expectedPhase: v1.PodPending,
			}),

			// ── VF tier2 (≤2 PCI hops, peers_0 ∪ peers_1 ∪ peers_2) ─────────
			// Pseudo VF topo: all 13 VF devices are tier2-reachable
			Entry("vf-tier2: 4 devices", tierAllocationTC{
				claimNameSuffix: "vf-t2-4", isVF: true, tier: 2, count: 4, expectedPhase: v1.PodRunning,
			}),
			Entry("vf-tier2: 8 devices", tierAllocationTC{
				claimNameSuffix: "vf-t2-8", isVF: true, tier: 2, count: 8, expectedPhase: v1.PodRunning,
			}),
			Entry("vf-tier2: 14 devices (over availability, pending)", tierAllocationTC{
				claimNameSuffix: "vf-t2-14", isVF: true, tier: 2, count: 14, expectedPhase: v1.PodPending,
			}),
		)
	})

})
