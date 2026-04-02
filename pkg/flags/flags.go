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

package flags

import (
	"path/filepath"

	cst "github.com/ibm-aiu/dra-driver-spyre/pkg/const"
	coreclientset "k8s.io/client-go/kubernetes"
)

type Flags struct {
	KubeClientConfig KubeClientConfig
	DiscoveryConfig  DiscoveryConfig
	LoggingConfig    *LoggingConfig

	NodeName                      string
	CDIRoot                       string
	KubeletRegistrarDirectoryPath string
	KubeletPluginsDirectoryPath   string
	HealthCheckPort               int
}

type Config struct {
	*Flags
	Coreclient coreclientset.Interface
}

func (c Config) DriverPluginPath() string {
	return filepath.Join(c.Flags.KubeletPluginsDirectoryPath, cst.DriverName)
}
