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

package utils

// Driver-related constants
const (
	DriverName    = "spyre.ibm.com"
	DriverVersion = "0.0.1"

	PluginRegistrationPath     = "/var/lib/kubelet/plugins_registry/" + DriverName + ".sock"
	DriverPluginPath           = "/var/lib/kubelet/plugins/" + DriverName
	DriverPluginSocketPath     = DriverPluginPath + "/plugin.sock"
	DriverPluginCheckpointFile = "checkpoint.json"
)

// Driver env key/value
const (
	PseudoDeviceModeKey = "PSEUDO_DEVICE_MODE"
	ModeEnabledValue    = "1"

	NodeNameEnvKey = "NODE_NAME"

	ConfigHostPathKey = "CONFIG_HOSTPATH"
	OutputPathKey     = "CONFIG_FILEPATH"
	ConfigFileNameKey = "CONFIG_FILENAME"

	MetricsHostPathKey   = "SENTIENT_MAP_HOSTPATH"
	MetricsOutputPathKey = "SENTIENT_MAP_FILEPATH"

	TemplatePathKey = "SENLIB_CONFIG_TEMPLATE_FILEPATH"
)

// Driver host paths
const (
	SpyreConfigBaseFolderName = "container-config"
	SpyreMetricBaseFolderName = "container-metrics"
)

// Container-related constants
const (
	VfioMount    = "/dev/vfio/vfio"
	DeviceEnvKey = "PCIDEVICE_IBM_COM_AIU_PF"
)
