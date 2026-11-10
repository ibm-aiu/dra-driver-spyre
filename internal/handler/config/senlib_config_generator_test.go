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
	"path/filepath"
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

var expectedEnabledMetricPath = filepath.Join(TestMetricsContainerPath, WildCardLocalMetricPath)

var _ = Describe("SenlibConfigGenerator", func() {
	DescribeTable("generator test", func(tcSubFolder string, deviceProductId types.ProductID, busIds []string, metricEnabled bool, expectedMetricPath string, expectedRisvDisabled bool, expectedError error) {
		os.Setenv(cst.TemplatePathKey, fmt.Sprintf("./test-template/%s", tcSubFolder))
		defer os.Unsetenv(cst.TemplatePathKey)
		generator := NewSenlibConfigGenerator()
		content, err := generator.GenerateConfigContent(deviceProductId, busIds, TestMetricsContainerPath)
		if expectedError != nil {
			Expect(err).NotTo(BeNil())
			Expect(strings.Contains(err.Error(), expectedError.Error())).To(BeTrue())
			return
		}
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
		// multi_aiu_config_path is no longer set by the generator
		Expect(senlibConfig.General.MultiSpyreConfigPath).To(BeEmpty())

		// VF devices should have doom=true, PF devices should have doom=false
		expectedDoom := deviceProductId == types.ProductIDVf
		Expect(senlibConfig.General.Doom).To(Equal(expectedDoom))
	},
		Entry("single Spyre, disable metrics", "disable", types.ProductIDVf, []string{"01"}, false, WildCardLocalMetricPath, false, nil),
		Entry("multiple Spyres, disable metrics", "disable", types.ProductIDVf, []string{"01", "02"}, false, WildCardLocalMetricPath, false, nil),
		Entry("single Spyre, enable metrics", "enable", types.ProductIDVf, []string{"01"}, true, expectedEnabledMetricPath, false, nil),
		Entry("multiple Spyre, enable metrics", "enable", types.ProductIDVf, []string{"01", "02"}, true, expectedEnabledMetricPath, false, nil),
		Entry("single Spyre, no METRICS.general defined", "only-metrics", types.ProductIDVf, []string{"01"}, false, WildCardLocalMetricPath, false, nil),
		Entry("multiple Spyre, no METRICS.general defined", "only-metrics", types.ProductIDVf, []string{"01", "02"}, false, WildCardLocalMetricPath, false, nil),
		Entry("single Spyre, no METRICS defined", "only-general", types.ProductIDVf, []string{"01"}, false, WildCardLocalMetricPath, false, nil),
		Entry("multiple Spyre, no METRICS defined", "only-general", types.ProductIDVf, []string{"01", "02"}, false, WildCardLocalMetricPath, false, nil),
		Entry("empty", "empty", types.ProductIDVf, []string{"01"}, false, "", false, ErrNoGeneralKey),
		Entry("wrong format", "wrong-format", types.ProductIDVf, []string{"01"}, false, "", false, fmt.Errorf("unmarshal")),
		Entry("wrong format of GENERAL", "wrong-format-general", types.ProductIDVf, []string{"01"}, false, "", false, fmt.Errorf("failed to parse GENERAL:")),
		Entry("wrong format of METRICS", "wrong-format-metrics", types.ProductIDVf, []string{"01"}, false, "", false, fmt.Errorf("failed to parse METRICS:")),
		Entry("wrong format of METRICS.general", "wrong-format-metrics-general", types.ProductIDVf, []string{"01"}, false, "", false, fmt.Errorf("failed to parse METRICS.general:")),
		Entry("wrong template path", "wrong-path", types.ProductIDVf, []string{"01"}, false, "", false, fmt.Errorf("error opening")),
		Entry("unknown Spyre, enable metrics", "enable", types.ProductIDVf, []string{""}, true, expectedEnabledMetricPath, false, nil),
		Entry("PF mode", "disable", types.ProductIDPf, []string{"01"}, false, WildCardLocalMetricPath, true, nil),
		Entry("PF mode with riscv-enabled template", "riscv-enable", types.ProductIDPf, []string{"01"}, false, WildCardLocalMetricPath, true, nil),
		Entry("VF mode with riscv-enabled template", "riscv-enable", types.ProductIDVf, []string{"01"}, false, WildCardLocalMetricPath, false, nil),
	)

	DescribeTable("DOOM configuration", func(tcSubFolder string, deviceProductId types.ProductID, busIds []string, expectedGeneralDoom bool, checkSNTMCI bool) {
		os.Setenv(cst.TemplatePathKey, fmt.Sprintf("./test-template/%s", tcSubFolder))
		generator := NewSenlibConfigGenerator()
		content, err := generator.GenerateConfigContent(deviceProductId, busIds, TestMetricsContainerPath)
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

	It("sets a different metrics path", func() {
		os.Setenv(cst.TemplatePathKey, "./test-template/enable")
		defer os.Unsetenv(cst.TemplatePathKey)
		generator := NewSenlibConfigGenerator()
		content, err := generator.GenerateConfigContent(types.ProductIDPf, []string{"01"}, "/custom/metrics")
		Expect(err).To(BeNil())
		var senlibConfig SenlibConfig
		Expect(json.Unmarshal(content, &senlibConfig)).To(Succeed())
		Expect(senlibConfig.Metric.General.Enable).To(BeTrue())
		Expect(senlibConfig.Metric.General.Path).To(Equal("/custom/metrics/metrics.%BUSID"))
	})
})
