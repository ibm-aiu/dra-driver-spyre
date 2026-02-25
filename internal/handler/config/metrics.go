// (C) Copyright IBM Corp.  2025,2026 IBM Corp.
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
	"fmt"
	"path/filepath"
	"strings"

	cst "github.com/ibm-aiu/dra-driver-spyre/pkg/const"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	cdispec "tags.cncf.io/container-device-interface/specs-go"
)

var metricsHostPath string
var metricsContainerPath string

// InitMetricsMountPath initializes path host/container path for sentient map metrics
func InitMetricsMountPath() error {
	metricsHostPath = utils.GetMetricsHostPath()
	metricsContainerPath = utils.GetMetricsContainerPath()
	return utils.CreateFolderIfNotExists(metricsHostPath)
}

func getMetricsMount(configHostMntPath string) (*cdispec.Mount, error) {
	outputPath, err := CreateNewMetricsFolder(metricsHostPath, configHostMntPath)
	if err != nil {
		return nil, err
	}
	return &cdispec.Mount{
		ContainerPath: metricsContainerPath,
		HostPath:      outputPath,
		Options:       readWriteMntOpts,
	}, nil
}

func IsMetricsMnt(containerMntPath, hostMntPath string) bool {
	targetMetricsPath := utils.GetMetricsContainerPath()
	return containerMntPath == targetMetricsPath && strings.Contains(hostMntPath, cst.SpyreMetricBaseFolderName)
}

func WritePodInfo(mntHostPaths []string, pod corev1.Pod) error {
	for _, hostMntPath := range mntHostPaths {
		if strings.Contains(hostMntPath, cst.SpyreMetricBaseFolderName) {
			if err := writeInfoFiles(hostMntPath, pod); err != nil {
				return fmt.Errorf("error writing pod info to %s: %v", hostMntPath, err)
			}
			return nil
		}
	}
	return fmt.Errorf("%s not found in %v", cst.SpyreMetricBaseFolderName, mntHostPaths)
}

// CreateNewMetricsFolder creates a corresponding metrics folder to the config path
func CreateNewMetricsFolder(metricsHostPath, configHostPath string) (string, error) {
	uuidValue := GetUuidFromPath(configHostPath)
	newFolder := filepath.Join(metricsHostPath, uuidValue)
	err := utils.CreateFolderIfNotExists(newFolder)
	return newFolder, err
}
