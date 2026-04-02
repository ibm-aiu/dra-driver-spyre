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

package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ibm-aiu/dra-driver-spyre/pkg/types"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/utils"

	cst "github.com/ibm-aiu/dra-driver-spyre/pkg/const"
)

const (
	GeneralKey = "GENERAL"
	MetricsKey = "METRICS"
	RISCVKey   = "RISCV"
	SNTMCIKey  = "SNT_MCI"

	MetricGeneralKey        = "general"
	RISCVDoomKey            = "DOOM"
	SNTMCIDCRKey            = "DCR"
	DCRMCICTRLKey           = "MCI_CTRL"
	BusIdKey                = "sen_bus_id"
	EnableKey               = "enable"
	PathKey                 = "path"
	MultiSpyreConfigPathKey = "multi_aiu_config_path"
	EnableRISCVKey          = "ENABLE_RISCV"

	resourcePoolFileName = "resource_pool"
	defaultTemplatePath  = "/etc/senlib-config-template"

	SharedMetricPath           = "/data/multi-aiu-sentientmap"
	SharedMetricPathFormat     = "/data/sentientmap_%s"
	SharedMultiSpyreConfigPath = "/data/multi-aiu-config"
	LocalMetricPath            = "./sentientmap"
	LocalMultiSpyreConfigPath  = "/tmp/testing"
	UnknownDevice              = "unknown"
)

var (
	ErrNoGeneralKey = fmt.Errorf("cannot find %s config", GeneralKey)
)

type SenlibConfigGeneral struct {
	PciAddresses         []string `json:"sen_bus_id"`
	MultiSpyreConfigPath string   `json:"multi_aiu_config_path"`
	Doom                 bool     `json:"doom"`
}

type SenlibConfigMetricGeneral struct {
	General SenlibConfigMetric `json:"general"`
}

type SenlibConfigMetric struct {
	Enable bool   `json:"enable"`
	Path   string `json:"path"`
}

type SenlibConfig struct {
	General SenlibConfigGeneral       `json:"GENERAL"`
	Metric  SenlibConfigMetricGeneral `json:"METRICS"`
	RISCV   map[string]any            `json:"RISCV,omitempty"`
}

type SenlibConfigGenerator struct {
	templateFilePath string
}

func NewSenlibConfigGenerator() SenlibConfigGenerator {
	templatePath := utils.GetEnvOrDefault(cst.TemplatePathKey, defaultTemplatePath)
	templateFilePath := fmt.Sprintf("%s/%s", templatePath, utils.GetConfigFileName())

	return SenlibConfigGenerator{
		templateFilePath: templateFilePath,
	}
}

/*
GenerateConfigContent generates config json file based on senlib config template file and allocated bus ids by
 1. adding bus ids to `GENERAL.sen_bus_id`.
 2. setting `METRICS.path` to a file/folder location for a process to write metrics of single/multiple Spyre(s)
    2.1. For `METRICS.enable: true`,
    2.1.1. For single-Spyre, set to /data/sentientmap_[bus_id]
    2.1.2. For multi-Spyre, set to /data/multi-aiu-sentientmap
    2.2. Otherwise, set to ./sentientmap (default value)
 3. For multi-Spyre, setting `GENERAL.multi_aiu_config_path` to a folder for a split program to place
    senlib_config_[rank].json
    3.1. For `METRICS.enable: true`, setting to `/data/multi-aiu-config`
    3.2. Otherwise, setting to `/tmp/testing` (refer to current multi-aiu script)
*/
func (g SenlibConfigGenerator) GenerateConfigContent(productId types.ProductID, busIds []string) (content []byte, err error) { //nolint:lll
	// Open the JSON file
	var file []byte
	file, err = os.ReadFile(g.templateFilePath)
	if err != nil {
		return content, fmt.Errorf("error opening file: %v", err)
	}

	var configMap map[string]any

	// Unmarshal the JSON data
	if err = json.Unmarshal(file, &configMap); err == nil {
		if generalConfigInterface, found := configMap[GeneralKey]; found {
			if _, ok := generalConfigInterface.(map[string]any); ok {
				// set .GENERAL.PciAddresses
				configMap[GeneralKey].(map[string]any)[BusIdKey] = busIds
				// set .GENERAL.doom based on device type (true for VF, false for PF)
				if productId == types.ProductIDVf {
					configMap[GeneralKey].(map[string]any)["doom"] = true
				} else {
					configMap[GeneralKey].(map[string]any)["doom"] = false
				}
				// set .METRICS.general by checking .METRICS.general.enable
				if metricsConfigInterface, found := configMap[MetricsKey]; found {
					var metricsConfig map[string]any
					if metricsConfig, ok = metricsConfigInterface.(map[string]any); ok {
						if metricsGeneralConfigInterface, found := metricsConfig[MetricGeneralKey]; found {
							if metricGeneralConfig, ok := metricsGeneralConfigInterface.(map[string]any); ok {
								if enableInterface, found := metricGeneralConfig[EnableKey]; found {
									metricEnabled := false
									if metricEnabled, ok = enableInterface.(bool); !ok {
										metricEnabled = false
									}
									if metricEnabled {
										if len(busIds) <= 1 {
											busId0 := UnknownDevice
											if len(busIds) == 1 && busIds[0] != "" {
												busId0 = busIds[0]
											}
											configMap[MetricsKey].(map[string]any)[MetricGeneralKey].(map[string]any)[PathKey] = fmt.Sprintf(SharedMetricPathFormat, busId0) //nolint:lll
										} else if len(busIds) > 1 {
											configMap[MetricsKey].(map[string]any)[MetricGeneralKey].(map[string]any)[PathKey] = SharedMetricPath //nolint:lll
											configMap[GeneralKey].(map[string]any)[MultiSpyreConfigPathKey] = SharedMultiSpyreConfigPath
										}
									} else {
										configMap[MetricsKey].(map[string]any)[MetricGeneralKey].(map[string]any)[PathKey] = LocalMetricPath //nolint:lll
										if len(busIds) > 1 {
											configMap[GeneralKey].(map[string]any)[MultiSpyreConfigPathKey] = LocalMultiSpyreConfigPath
										}
									}
								}
							} else {
								err = fmt.Errorf("failed to parse METRICS.general: %v", metricsGeneralConfigInterface)
							}
						} else {
							// general key not found
							configMap[MetricsKey].(map[string]any)[MetricGeneralKey] = map[string]any{
								EnableKey: false,
								PathKey:   LocalMetricPath,
							}
							if len(busIds) > 1 {
								configMap[GeneralKey].(map[string]any)[MultiSpyreConfigPathKey] = LocalMultiSpyreConfigPath
							}
						}
					} else {
						err = fmt.Errorf("failed to parse METRICS: %v", metricsConfigInterface)
					}
				} else {
					// METRICS key not found
					configMap[MetricsKey] = map[string]any{
						MetricGeneralKey: map[string]any{
							EnableKey: false,
							PathKey:   LocalMetricPath,
						},
					}
					if len(busIds) > 1 {
						configMap[GeneralKey].(map[string]any)[MultiSpyreConfigPathKey] = LocalMultiSpyreConfigPath
					}
				}
				configMap = modifyRISCVContent(productId, configMap)
				if err == nil {
					content, err = json.Marshal(configMap)
				}
			} else {
				err = fmt.Errorf("failed to parse GENERAL: %v", generalConfigInterface)
			}
		} else {
			err = ErrNoGeneralKey
		}
	} else {
		err = fmt.Errorf("error unmarshalling JSON: %v", err)
	}
	return content, err
}

func (g SenlibConfigGenerator) GenerateConfigFile(productId types.ProductID, busIds []string, outputPath string) error { //nolint:lll
	content, err := g.GenerateConfigContent(productId, busIds)
	if err != nil {
		return err
	}
	outputFilePath := filepath.Join(outputPath, utils.GetConfigFileName())
	file, err := os.Create(outputFilePath)
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck
	_, err = file.Write(content)
	return err
}

func ReadSenlibConfig(mntPath string) ([]string, error) {
	senlibFilepath := filepath.Join(mntPath, utils.GetConfigFileName())
	file, err := os.Open(senlibFilepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close() //nolint:errcheck
	var config SenlibConfig
	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %v", err)
	}
	return config.General.PciAddresses, nil
}

func ReadResourcePool(mntPath string) (string, error) {
	senlibFilepath := filepath.Join(mntPath, resourcePoolFileName)
	file, err := os.Open(senlibFilepath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close() //nolint:errcheck
	data, err := io.ReadAll(file)
	return string(data), err
}

// modifyRISCVContent applies the following config based on device type:
// - For PF Mode: sets ENABLE_RISCV to 0x0 in SNT_MCI
//
// Note: DOOM configuration is now handled via GENERAL.doom (set in GenerateConfigContent)
// and RISCV.DOOM.enable is no longer needed.
//
// PF mode:
//
//	{
//	  "SNT_MCI": {
//	    "DCR": {
//	      "MCI_CTRL": {
//	        "ENABLE_RISCV": "0x0"
//	      }
//	    }
//	  }
//	}
func modifyRISCVContent(deviceProductId types.ProductID, configMap map[string]any) map[string]any {
	if deviceProductId == types.ProductIDVf {
		// VF mode: no additional RISCV configuration needed
		// DOOM is configured via GENERAL.doom
		return configMap
	}

	// PF mode: configure SNT_MCI
	if _, ok := configMap[SNTMCIKey]; !ok {
		configMap[SNTMCIKey] = make(map[string]any)
	}
	if _, ok := configMap[SNTMCIKey].(map[string]any)[SNTMCIDCRKey]; !ok {
		configMap[SNTMCIKey].(map[string]any)[SNTMCIDCRKey] = make(map[string]any)
	}
	if _, ok := configMap[SNTMCIKey].(map[string]any)[SNTMCIDCRKey].(map[string]any)[DCRMCICTRLKey]; !ok {
		configMap[SNTMCIKey].(map[string]any)[SNTMCIDCRKey].(map[string]any)[DCRMCICTRLKey] = make(map[string]any)
	}
	configMap[SNTMCIKey].(map[string]any)[SNTMCIDCRKey].(map[string]any)[DCRMCICTRLKey].(map[string]any)[EnableRISCVKey] = "0x0" //nolint:lll
	return configMap
}
