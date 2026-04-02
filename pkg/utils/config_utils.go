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

package utils

import (
	"path/filepath"

	cst "github.com/ibm-aiu/dra-driver-spyre/pkg/const"
)

var (
	deviceDriverPath = "/usr/local/etc/spyre-dra-driver"
	defaultHostPath  = filepath.Join(deviceDriverPath, cst.SpyreConfigBaseFolderName)

	defaultOutputPath     = "/etc/aiu"
	defaultConfigFileName = "senlib_config.json"

	defaultMetricsHostPath   = filepath.Join(deviceDriverPath, cst.SpyreMetricBaseFolderName)
	defaultMetricsOutputPath = "/data"
)

func GetConfigContainerPath() string {
	return GetEnvOrDefault(cst.OutputPathKey, defaultOutputPath)
}

func GetConfigHostPath() string {
	return GetEnvOrDefault(cst.ConfigHostPathKey, defaultHostPath)
}

func GetConfigFileName() string {
	return GetEnvOrDefault(cst.ConfigFileNameKey, defaultConfigFileName)
}

func GetMetricsHostPath() string {
	return GetEnvOrDefault(cst.MetricsHostPathKey, defaultMetricsHostPath)
}

func GetMetricsContainerPath() string {
	return GetEnvOrDefault(cst.MetricsOutputPathKey, defaultMetricsOutputPath)
}
