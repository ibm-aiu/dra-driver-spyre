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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cdispec "tags.cncf.io/container-device-interface/specs-go"

	cst "github.com/ibm-aiu/dra-driver-spyre/pkg/const"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/topology"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/types"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/utils"
	klog "k8s.io/klog/v2"
)

var (
	commonMntOpts    = []string{"nosuid", "nodev", "bind"}
	readOnlyMntOpts  = append(commonMntOpts, "ro")
	readWriteMntOpts = append(commonMntOpts, "rw")
)

var senlibConfigGenerator SenlibConfigGenerator
var configHostPath string
var configContainerPath string

type ConfigHandler struct {
	// uuidMap maps from deviceIDs to generated uuid
	uuidMap map[string]string
}

func uniqueStringFromDeviceIDs(deviceIDs []string) string {
	return strings.Join(deviceIDs, "-")
}

func GetUuidFromPath(hostConfigPath string) string {
	folderName := filepath.Base(hostConfigPath)
	if folderName == "." {
		return ""
	}
	return folderName
}

// InitConfigMount prepares config folder and return handler
func InitConfigMount() (*ConfigHandler, error) {
	cfgHandler := &ConfigHandler{make(map[string]string)}
	configHostPath = utils.GetConfigHostPath()
	configContainerPath = utils.GetConfigContainerPath()
	if err := InitMetricsMountPath(); err != nil {
		return cfgHandler, err
	}
	senlibConfigGenerator = NewSenlibConfigGenerator()
	err := utils.CreateFolderIfNotExists(configHostPath)
	return cfgHandler, err
}

// GetConfigMetricsMount returns config and metrics mounts
func (h *ConfigHandler) GetConfigMetricsMount(productId types.ProductID,
	deviceIDs []string) (mnts []*cdispec.Mount, err error) {
	// create new folder on each container requests
	outputPath, err := utils.CreateNewConfigFolder(configHostPath)
	if err != nil {
		return mnts, err
	}
	if err := CopyTopologyFile(outputPath); err != nil {
		klog.Warningf("cannot copy topology file to %s: %v, gracefully skip", outputPath, err)
	}
	configMnt, err := getSenlibConfigMount(productId, deviceIDs, outputPath)
	if err != nil {
		return mnts, err
	}
	mnts = append(mnts, configMnt)
	metricsMnt, err := getMetricsMount(configMnt.HostPath)
	if err == nil {
		mnts = append(mnts, metricsMnt)
		key := uniqueStringFromDeviceIDs(deviceIDs)
		h.uuidMap[key] = outputPath
	}
	return mnts, err
}

// GetMountedPath returns mounted path for deviceIDs
func (h *ConfigHandler) GetMountedPath(deviceIDs []string) (val string, ok bool) {
	key := uniqueStringFromDeviceIDs(deviceIDs)
	val, ok = h.uuidMap[key]
	return val, ok
}

// Still outstanding: clean up folder after permanently delete device plugin pod. Might be called
// via some API from controller when cluster policy is deleted.
func getSenlibConfigMount(productId types.ProductID, deviceIDs []string, outputPath string) (*cdispec.Mount, error) {
	err := senlibConfigGenerator.GenerateConfigFile(productId, deviceIDs, outputPath)
	if err != nil {
		return nil, err
	}

	return &cdispec.Mount{
		ContainerPath: configContainerPath,
		HostPath:      outputPath,
		Options:       readOnlyMntOpts,
	}, nil
}

func CopyTopologyFile(outputPath string) error {
	dst := fmt.Sprintf("%s/topo.json", outputPath)
	topologyFilepath := topology.GetTopologyFile()
	return utils.CopyFile(topologyFilepath, dst)
}

func ListAllMounts(hostPath string) ([]string, error) {
	files, err := os.ReadDir(hostPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", hostPath, err)
	}

	mnts := []string{}
	for _, file := range files {
		if file.IsDir() {
			mnt := filepath.Join(hostPath, file.Name())
			mnts = append(mnts, mnt)
		}
	}
	return mnts, nil
}

func IsConfigHostPathExist() bool {
	hostpath := utils.GetConfigHostPath()
	_, err := os.Stat(hostpath)
	return err == nil
}

func IsSomeContainerMounted() bool {
	hostpath := utils.GetConfigHostPath()
	files, err := os.ReadDir(hostpath)
	if err == nil {
		return len(files) > 0
	}
	return false
}

func IsConfigMnt(containerMntPath, hostMntPath string) bool {
	targetConfigPath := utils.GetConfigContainerPath()
	return containerMntPath == targetConfigPath && strings.Contains(hostMntPath, cst.SpyreConfigBaseFolderName)
}
