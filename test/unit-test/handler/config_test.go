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

package cdi_test

import (
	"os"

	"github.com/ibm-aiu/dra-driver-spyre/pkg/types"
	. "github.com/ibm-aiu/dra-driver-spyre/pkg/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cdiapi "tags.cncf.io/container-device-interface/pkg/cdi"
	cdispec "tags.cncf.io/container-device-interface/specs-go"
)

var _ = Describe("Senlib Config", Ordered, func() {
	pciAddresses := []string{"0000:1a:00.0", "0000:1c:00.0"}

	Context("init/prepare - CleanZombieConfigFolders", func() {
		mnts := []*cdispec.Mount{}
		devices := getPreparedDevices(pciAddresses)

		It("should remove zombie config", func() {
			mnts = GenerateSenlibConfig(pciAddresses)
			CheckPathExists(mnts[0].HostPath, true)
			By("clean zombie config")
			err := cdiHandler.CleanZombieConfigFolders()
			Expect(err).To(BeNil())
			CheckPathExists(mnts[0].HostPath, false)
		})

		It("should not remove non-zombie config", func() {
			By("create spec")
			claimUID := generateUID()
			_, err := cdiHandler.CreateClaimSpecFile(claimUID, types.ProductIDPf, devices)
			Expect(err).To(BeNil())
			spec, err := cdiHandler.ReadSpec(claimUID)
			Expect(err).To(BeNil())
			By("clean zombie config")
			err = cdiHandler.CleanZombieConfigFolders()
			Expect(err).To(BeNil())
			for _, mnt := range spec.ContainerEdits.Mounts {
				if mnt.ContainerPath == GetConfigContainerPath() ||
					mnt.ContainerPath == GetMetricsContainerPath() {
					CheckPathExists(mnt.HostPath, true)
				}
			}
		})
	})

	It("prepare - unprepare", func() {
		mnts := GenerateSenlibConfig(pciAddresses)
		CheckPathExists(mnts[0].HostPath, true)
		spec := &cdiapi.Spec{
			Spec: &cdispec.Spec{
				ContainerEdits: cdispec.ContainerEdits{
					Mounts: mnts,
				},
			},
		}
		By("delete config")
		cdiHandler.DeleteConfigFile(spec)
		CheckPathExists(mnts[0].HostPath, false)
	})
})

func GenerateSenlibConfig(pciAddresses []string) []*cdispec.Mount {
	mnts, err := configHandler.GetConfigMetricsMount(types.ProductIDPf, pciAddresses)
	Expect(err).To(BeNil())
	// config and metrics
	Expect(len(mnts)).To(Equal(2))
	return mnts
}

func CheckPathExists(hostpath string, exist bool) {
	_, err := os.Stat(hostpath)
	Expect(os.IsNotExist(err)).To(Equal(!exist))
}
