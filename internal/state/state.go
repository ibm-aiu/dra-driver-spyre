/*
 * Copyright 2023 The Kubernetes Authors.
 * Modified by IBM Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package state

import (
	"fmt"
	"slices"
	"sync"

	"github.com/ibm-aiu/dra-driver-spyre/internal/discovery"
	"github.com/ibm-aiu/dra-driver-spyre/internal/handler"
	cst "github.com/ibm-aiu/dra-driver-spyre/pkg/const"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/flags"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/topology"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/types"
	resourceapi "k8s.io/api/resource/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	klog "k8s.io/klog/v2"
	drapbv1 "k8s.io/kubelet/pkg/apis/dra/v1beta1"
	"k8s.io/kubernetes/pkg/kubelet/checkpointmanager"

	configapi "github.com/ibm-aiu/dra-driver-spyre/api/ibm.com/resource/spyre/v1alpha1"
)

type DeviceState struct {
	mu                sync.Mutex
	CDI               *handler.CDIHandler
	Allocatable       types.AllocatableDevices
	checkpointManager checkpointmanager.CheckpointManager
	deviceDiscovery   *discovery.DeviceDiscovery
}

func NewDeviceState(config *flags.Config) (*DeviceState, error) {
	topologyFile := config.DiscoveryConfig.TopologyFilepath
	if topologyFile == "" {
		topologyFile = topology.GetTopologyFile()
	}
	deviceDiscovery, err := discovery.NewDeviceDiscovery(topologyFile)
	if err != nil {
		return nil, fmt.Errorf("error device discovery initialization: %v", err)
	}
	allocatable, err := deviceDiscovery.GetAllocatableDevices()
	if err != nil {
		return nil, fmt.Errorf("error get allocatable devices: %w", err)
	}

	cdi, err := handler.NewCDIHandler(config)
	if err != nil {
		return nil, fmt.Errorf("unable to create CDI handler: %v", err)
	}
	err = cdi.CreateCommonSpecFile()
	if err != nil {
		return nil, fmt.Errorf("unable to create CDI spec file for common edits: %v", err)
	}
	err = cdi.CleanZombieConfigFolders()
	if err != nil {
		klog.Warningf("failed to clean zombie senlib config folder: %v", err)
	}
	checkpointManager, err := checkpointmanager.NewCheckpointManager(cst.DriverPluginPath)
	if err != nil {
		return nil, fmt.Errorf("unable to create checkpoint manager: %v", err)
	}

	state := &DeviceState{
		CDI:               cdi,
		Allocatable:       allocatable,
		checkpointManager: checkpointManager,
		deviceDiscovery:   deviceDiscovery,
	}

	checkpoints, err := state.checkpointManager.ListCheckpoints()
	if err != nil {
		return nil, fmt.Errorf("unable to list checkpoints: %v", err)
	}

	for _, c := range checkpoints {
		if c == cst.DriverPluginCheckpointFile {
			return state, nil
		}
	}

	checkpoint := newCheckpoint()
	if err := state.checkpointManager.CreateCheckpoint(cst.DriverPluginCheckpointFile, checkpoint); err != nil {
		return nil, fmt.Errorf("unable to sync to checkpoint: %v", err)
	}

	return state, nil
}

func (s *DeviceState) Prepare(claim *resourceapi.ResourceClaim) ([]*drapbv1.Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	claimUID := string(claim.UID)

	checkpoint := newCheckpoint()
	if err := s.checkpointManager.GetCheckpoint(cst.DriverPluginCheckpointFile, checkpoint); err != nil {
		return nil, fmt.Errorf("unable to sync from checkpoint: %v", err)
	}
	preparedClaims := checkpoint.V1.PreparedClaims

	if preparedClaims[claimUID] != nil {
		return preparedClaims[claimUID].GetDevices(), nil
	}

	productId, preparedDevices, err := s.prepareDevices(claim)
	if err != nil {
		return nil, fmt.Errorf("prepare failed: %v", err)
	}
	if spec, err := s.CDI.CreateClaimSpecFile(claimUID, productId, preparedDevices); err != nil {
		return nil, fmt.Errorf("unable to create CDI spec file for claim: %v", err)
	} else {
		klog.Infof("Create CDI spec: %v", spec)
	}

	preparedClaims[claimUID] = preparedDevices
	if err := s.checkpointManager.CreateCheckpoint(cst.DriverPluginCheckpointFile, checkpoint); err != nil {
		return nil, fmt.Errorf("unable to sync to checkpoint: %v", err)
	}

	return preparedClaims[claimUID].GetDevices(), nil
}

func (s *DeviceState) Unprepare(claimUID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	checkpoint := newCheckpoint()
	if err := s.checkpointManager.GetCheckpoint(cst.DriverPluginCheckpointFile, checkpoint); err != nil {
		return fmt.Errorf("unable to sync from checkpoint: %v", err)
	}
	preparedClaims := checkpoint.V1.PreparedClaims

	if preparedClaims[claimUID] == nil {
		return nil
	}

	s.unprepareDevices(claimUID, preparedClaims[claimUID])
	err := s.CDI.DeleteClaimSpec(claimUID)
	if err != nil {
		return fmt.Errorf("unable to delete CDI spec file for claim: %v", err)
	}

	delete(preparedClaims, claimUID)
	if err := s.checkpointManager.CreateCheckpoint(cst.DriverPluginCheckpointFile, checkpoint); err != nil {
		return fmt.Errorf("unable to sync to checkpoint: %v", err)
	}

	return nil
}

func (s *DeviceState) prepareDevices(claim *resourceapi.ResourceClaim) (types.ProductID, types.PreparedDevices, error) {
	if claim.Status.Allocation == nil {
		return "", nil, fmt.Errorf("claim not yet allocated")
	}

	var productId types.ProductID

	// Walk through each device allocation and prepare it.
	var preparedDevices types.PreparedDevices
	for _, result := range claim.Status.Allocation.Devices.Results {
		allocatableDevice, exists := s.Allocatable[result.Device]
		if !exists {
			return "", nil, fmt.Errorf("requested device is not allocatable: %v", result.Device)
		}
		if productId == "" {
			productId = allocatableDevice.ProductID
		} else if allocatableDevice.ProductID != productId {
			return "", nil,
				fmt.Errorf("requested devices are not the same type: %s, %s, please use pre-defined class",
					productId, allocatableDevice.ProductID)
		}
		config := configapi.DefaultParams()
		device := &types.PreparedDevice{
			Device: drapbv1.Device{
				RequestNames: []string{result.Request},
				PoolName:     result.Pool,
				DeviceName:   result.Device,
				CDIDeviceIDs: handler.GetClaimDevices([]string{result.Device}),
			},
			Config:        config.Config,
			PciAddress:    allocatableDevice.GetPciAddr(),
			CDIDeviceSpec: allocatableDevice.GetCDIDeviceSpec(),
		}

		// Apply any requested configuration here.
		//
		// In this example driver there is nothing to do at this point, but a
		// real driver would likely need to do some sort of hardware
		// configuration , based on the config that has been passed in.
		if err := device.ApplyConfig(); err != nil {
			return "", nil, fmt.Errorf("error applying GPU config: %v", err)
		}
		preparedDevices = append(preparedDevices, device)
	}

	return productId, preparedDevices, nil
}

func (s *DeviceState) unprepareDevices(claimUID string, devices types.PreparedDevices) {
	deviceIDs := handler.GetDeviceIDs(devices)
	klog.Infof("Unprepare devices: %v", deviceIDs)
	spec, err := s.CDI.ReadSpec(claimUID)
	if err != nil {
		klog.Warningf("unable to read spec claim %s: %v, cannot delete config file", claimUID, err)
	}
	s.CDI.DeleteConfigFile(spec)
}

// GetOpaqueDeviceConfigs returns an ordered list of configs specified for a device request in a resource claim.
//
// Configs can either come from the resource claim itself or from the device
// class associated with the request. Configs coming directly from the resource
// claim take precedence over configs coming from the device class. Moreover,
// configs found later in the list of configs attached to its source take
// precedence over configs found earlier in the list for that source.
//
// All of the configs relevant to the specified request for this driver will be
// returned in order of precedence (from lowest to highest). If no config is
// found, nil is returned.
func GetOpaqueDeviceConfigs(
	decoder runtime.Decoder,
	driverName,
	request string,
	possibleConfigs []resourceapi.DeviceAllocationConfiguration,
) ([]runtime.Object, error) {
	// Collect all configs in order of reverse precedence.
	var classConfigs []resourceapi.DeviceConfiguration
	var claimConfigs []resourceapi.DeviceConfiguration
	var candidateConfigs []resourceapi.DeviceConfiguration
	for _, config := range possibleConfigs {
		// If the config is for specific requests and the current request isn't
		// one of those, the config can be ignored.
		if len(config.Requests) != 0 && !slices.Contains(config.Requests, request) {
			continue
		}
		switch config.Source {
		case resourceapi.AllocationConfigSourceClass:
			classConfigs = append(classConfigs, config.DeviceConfiguration)
		case resourceapi.AllocationConfigSourceClaim:
			claimConfigs = append(claimConfigs, config.DeviceConfiguration)
		default:
			return nil, fmt.Errorf("invalid config source: %v", config.Source)
		}
	}
	candidateConfigs = append(candidateConfigs, classConfigs...)
	candidateConfigs = append(candidateConfigs, claimConfigs...)

	// Decode all configs that are relevant for the driver.
	resultConfigs := []runtime.Object{}
	for _, config := range candidateConfigs {
		// If this is nil, the driver doesn't support some future API extension
		// and needs to be updated.
		if config.Opaque == nil {
			return nil, fmt.Errorf("only opaque parameters are supported by this driver")
		}

		// Configs for different drivers may have been specified because a
		// single request can be satisfied by different drivers. This is not
		// an error -- drivers must skip over other driver's configs in order
		// to support this.
		if config.Opaque.Driver != driverName {
			continue
		}

		c, err := runtime.Decode(decoder, config.Opaque.Parameters.Raw)
		if err != nil {
			return nil, fmt.Errorf("error decoding config parameters: %w", err)
		}

		resultConfigs = append(resultConfigs, c)
	}

	return resultConfigs, nil
}

func GetProductID(device resourceapi.Device) {

}
