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

package cdi_test

import (
	"path/filepath"

	"github.com/google/uuid"
	configapi "github.com/ibm-aiu/dra-driver-spyre/api/ibm.com/resource/spyre/v1alpha1"
	. "github.com/ibm-aiu/dra-driver-spyre/internal/handler"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/types"
	. "github.com/ibm-aiu/dra-driver-spyre/pkg/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	drapbv1 "k8s.io/kubelet/pkg/apis/dra/v1beta1"
	cdispec "tags.cncf.io/container-device-interface/specs-go"
)

func generateUID() string {
	return uuid.New().String()
}

func getPreparedDevices(pciAddresses []string) types.PreparedDevices {
	var preparedDevices types.PreparedDevices
	for _, pciAddress := range pciAddresses {
		config := configapi.DefaultParams()
		deviceName := PciAddressToDeviceName(pciAddress)
		pciDevice := types.GeneratePseudoDevice(pciAddress, types.ProductIDPf)
		pseudoDevice := types.NewPseudoPciDevice(pciDevice)
		device := &types.PreparedDevice{
			Device: drapbv1.Device{
				RequestNames: []string{""},
				PoolName:     "",
				DeviceName:   deviceName,
				CDIDeviceIDs: GetClaimDevices([]string{deviceName}),
			},
			Config:        config.Config,
			PciAddress:    pseudoDevice.GetPciAddr(),
			CDIDeviceSpec: pseudoDevice.GetCDIDeviceSpec(),
		}
		preparedDevices = append(preparedDevices, device)
	}
	return preparedDevices
}

var _ = Describe("CDI", Ordered, func() {
	var claimUID = generateUID()
	pciAddresses := []string{"0000:1a:00.0", "0000:1c:00.0"}

	It("prepare", func() {
		expectedEnvs := []string{"PCIDEVICE_IBM_COM_AIU_PF=0000:1a:00.0,0000:1c:00.0"}
		devices := getPreparedDevices(pciAddresses)
		Expect(cdiHandler).NotTo(BeNil())
		_, err := cdiHandler.CreateClaimSpecFile(claimUID, types.ProductIDPf, devices)
		Expect(err).To(BeNil())
		spec, err := cdiHandler.ReadSpec(claimUID)
		Expect(err).To(BeNil())
		Expect(spec.ContainerEdits.Env).To(BeEquivalentTo(expectedEnvs))
		Expect(len(spec.ContainerEdits.Mounts)).To(BeNumerically(">", 0))
		mnts := spec.ContainerEdits.Mounts
		var senlibMnt *cdispec.Mount
		var metricsMnt *cdispec.Mount
		for _, mnt := range mnts {
			if mnt.ContainerPath == GetConfigContainerPath() {
				parentHostPath := filepath.Dir(mnt.HostPath)
				Expect(parentHostPath).To(Equal(ConfigHostPath))
				senlibMnt = mnt
			} else if mnt.ContainerPath == GetMetricsContainerPath() {
				parentHostPath := filepath.Dir(mnt.HostPath)
				Expect(parentHostPath).To(Equal(MetricsHostPath))
				metricsMnt = mnt
			}
		}
		Expect(senlibMnt).ToNot(BeNil())
		Expect(metricsMnt).ToNot(BeNil())
	})

	It("unprepare", func() {
		// confirm spec valid
		_, err := cdiHandler.ReadSpec(claimUID)
		Expect(err).To(BeNil())
		// call DeleteClaimSpec
		err = cdiHandler.DeleteClaimSpec(claimUID)
		Expect(err).To(BeNil())
		// spec should not be valid anymore
		_, err = cdiHandler.ReadSpec(claimUID)
		Expect(err).ToNot(BeNil())
	})
})
