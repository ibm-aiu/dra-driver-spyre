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

package config_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	. "github.com/ibm-aiu/dra-driver-spyre/internal/handler/config"
	cst "github.com/ibm-aiu/dra-driver-spyre/pkg/const"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	expectedRisvDisabledContent = `"SNT_MCI":{"DCR":{"MCI_CTRL":{"ENABLE_RISCV":"0x0"}}}`
)

var _ = Describe("SenlibConfigGenerator", func() {
	DescribeTable("generator test", func(tcSubFolder string, deviceProductId types.ProductID, busIds []string, metricEnabled bool, expectedMetricPath, expectedMultiSpyreConfigPath string, expectedRisvDisabled bool, expectedError error) {
		os.Setenv(cst.TemplatePathKey, fmt.Sprintf("./test-template/%s", tcSubFolder))
		generator := NewSenlibConfigGenerator()
		content, err := generator.GenerateConfigContent(deviceProductId, busIds)
		if expectedError != nil {
			Expect(err).NotTo(BeNil())
			Expect(strings.Contains(err.Error(), expectedError.Error())).To(BeTrue())
		} else {
			Expect(err).To(BeNil())
			var senlibConfig SenlibConfig
			if expectedRisvDisabled {
				Expect(string(content)).To(ContainSubstring(expectedRisvDisabledContent))
			} else {
				Expect(string(content)).NotTo(ContainSubstring(expectedRisvDisabledContent))
			}
			err = json.Unmarshal(content, &senlibConfig)
			Expect(err).To(BeNil())
			configuredBusIds := senlibConfig.General.PciAddresses
			Expect(len(configuredBusIds)).To(Equal(len(busIds)))
			for i := range busIds {
				Expect(busIds[i]).To(BeEquivalentTo(configuredBusIds[i]))
			}
			Expect(senlibConfig.Metric.General.Enable).To(Equal(metricEnabled))
			Expect(senlibConfig.Metric.General.Path).To(Equal(expectedMetricPath))
			Expect(senlibConfig.General.MultiSpyreConfigPath).To(Equal(expectedMultiSpyreConfigPath))

			// Validate doom configuration based on device type
			var configMap map[string]any
			err = json.Unmarshal(content, &configMap)
			Expect(err).To(BeNil())
			general, ok := configMap["GENERAL"].(map[string]any)
			Expect(ok).To(BeTrue())
			doom, exists := general["doom"]
			Expect(exists).To(BeTrue())
			// VF devices should have doom=true, PF devices should have doom=false
			expectedDoom := deviceProductId == types.ProductIDVf
			Expect(doom).To(Equal(expectedDoom))

			// RISCV.DOOM.enable is no longer set - DOOM is configured via GENERAL.doom

			os.Unsetenv(cst.TemplatePathKey)
		}
	},
		Entry("single Spyre, disable metrics", "disable", types.ProductIDVf, []string{"01"}, false, LocalMetricPath, "", false, nil),
		Entry("multiple Spyres, disable metrics", "disable", types.ProductIDVf, []string{"01", "02"}, false, LocalMetricPath, LocalMultiSpyreConfigPath, false, nil),
		Entry("single Spyre, enable metrics", "enable", types.ProductIDVf, []string{"01"}, true, "/data/sentientmap_01", "", false, nil),
		Entry("multiple Spyre, enable metrics", "enable", types.ProductIDVf, []string{"01", "02"}, true, SharedMetricPath, SharedMultiSpyreConfigPath, false, nil),
		Entry("single Spyre, no METRICS.general defined", "only-metrics", types.ProductIDVf, []string{"01"}, false, LocalMetricPath, "", false, nil),
		Entry("multiple Spyre, no METRICS.general defined", "only-metrics", types.ProductIDVf, []string{"01", "02"}, false, LocalMetricPath, LocalMultiSpyreConfigPath, false, nil),
		Entry("single Spyre, no METRICS defined", "only-general", types.ProductIDVf, []string{"01"}, false, LocalMetricPath, "", false, nil),
		Entry("multiple Spyre, no METRICS defined", "only-general", types.ProductIDVf, []string{"01", "02"}, false, LocalMetricPath, LocalMultiSpyreConfigPath, false, nil),
		Entry("empty", "empty", types.ProductIDVf, []string{"01"}, false, "", "", false, ErrNoGeneralKey),
		Entry("wrong format", "wrong-format", types.ProductIDVf, []string{"01"}, false, "", "", false, fmt.Errorf("unmarshal")),
		Entry("wrong format of GENERAL", "wrong-format-general", types.ProductIDVf, []string{"01"}, false, "", "", false, fmt.Errorf("failed to parse GENERAL:")),
		Entry("wrong format of METRICS", "wrong-format-metrics", types.ProductIDVf, []string{"01"}, false, "", "", false, fmt.Errorf("failed to parse METRICS:")),
		Entry("wrong format of METRICS.general", "wrong-format-metrics-general", types.ProductIDVf, []string{"01"}, false, "", "", false, fmt.Errorf("failed to parse METRICS.general:")),
		Entry("wrong template path", "wrong-path", types.ProductIDVf, []string{"01"}, false, "", "", false, fmt.Errorf("error opening")),
		Entry("unknown Spyre, enable metrics", "enable", types.ProductIDVf, []string{""}, true, "/data/sentientmap_unknown", "", false, nil),
		Entry("PF mode", "disable", types.ProductIDPf, []string{"01"}, false, LocalMetricPath, "", true, nil),
		Entry("PF mode with riscv-enabled template", "riscv-enable", types.ProductIDPf, []string{"01"}, false, LocalMetricPath, "", true, nil),
		Entry("VF mode with riscv-enabled template", "riscv-enable", types.ProductIDVf, []string{"01"}, false, LocalMetricPath, "", false, nil),
	)

	DescribeTable("DOOM configuration", func(tcSubFolder string, deviceProductId types.ProductID, busIds []string, expectedGeneralDoom bool, checkSNTMCI bool) {
		os.Setenv(cst.TemplatePathKey, fmt.Sprintf("./test-template/%s", tcSubFolder))
		generator := NewSenlibConfigGenerator()
		content, err := generator.GenerateConfigContent(deviceProductId, busIds)
		Expect(err).To(BeNil())

		var configMap map[string]any
		err = json.Unmarshal(content, &configMap)
		Expect(err).To(BeNil())

		// Validate GENERAL.doom
		general, ok := configMap["GENERAL"].(map[string]any)
		Expect(ok).To(BeTrue())
		doom, exists := general["doom"]
		Expect(exists).To(BeTrue())
		Expect(doom).To(Equal(expectedGeneralDoom))

		// RISCV.DOOM.enable is no longer set - DOOM is configured via GENERAL.doom

		// Check SNT_MCI for PF devices if requested
		if checkSNTMCI {
			sntMci, ok := configMap["SNT_MCI"].(map[string]any)
			Expect(ok).To(BeTrue())
			dcr, ok := sntMci["DCR"].(map[string]any)
			Expect(ok).To(BeTrue())
			mciCtrl, ok := dcr["MCI_CTRL"].(map[string]any)
			Expect(ok).To(BeTrue())
			enableRiscv, exists := mciCtrl["ENABLE_RISCV"]
			Expect(exists).To(BeTrue())
			Expect(enableRiscv).To(Equal("0x0"))
		}

		os.Unsetenv(cst.TemplatePathKey)
	},
		Entry("VF device with disable template - doom should be true", "disable", types.ProductIDVf, []string{"0001:00:00.0"}, true, false),
		Entry("PF device with disable template - doom should be false", "disable", types.ProductIDPf, []string{"0001:00:00.0"}, false, true),
		Entry("VF device with riscv-enable template - doom should override to true", "riscv-enable", types.ProductIDVf, []string{"0001:00:00.0"}, true, false),
		Entry("PF device with riscv-enable template - doom should be false", "riscv-enable", types.ProductIDPf, []string{"0001:00:00.0"}, false, true),
	)
})
