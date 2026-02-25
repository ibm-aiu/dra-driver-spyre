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

package driver

import (
	"context"
	"fmt"

	coreclientset "k8s.io/client-go/kubernetes"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"
	klog "k8s.io/klog/v2"

	"github.com/ibm-aiu/dra-driver-spyre/internal/health"
	"github.com/ibm-aiu/dra-driver-spyre/internal/state"
	cst "github.com/ibm-aiu/dra-driver-spyre/pkg/const"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/flags"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/utils"
	resourceapi "k8s.io/api/resource/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/dynamic-resource-allocation/resourceslice"
)

var _ kubeletplugin.DRAPlugin = &driver{}

type driver struct {
	client      coreclientset.Interface
	helper      *kubeletplugin.Helper
	state       *state.DeviceState
	healthcheck *health.HealthCheck
}

// NewDriver creates a driver with device state and kubelet plugin
// DeviceState discovers allocatable devices
// (using a call to traditional device plugin function)
// KubeletPlugin publishes resources for generating ResourceSlice.
func NewDriver(ctx context.Context, config *flags.Config) (*driver, error) {
	driver := &driver{
		client: config.Coreclient,
	}

	if utils.IsPseudoDeviceMode() {
		klog.Info("New Driver in Pseudo Device Mode")
	}

	state, err := state.NewDeviceState(config)
	if err != nil {
		return nil, err
	}
	driver.state = state

	helper, err := kubeletplugin.Start(
		ctx,
		driver,
		kubeletplugin.KubeClient(config.Coreclient),
		kubeletplugin.NodeName(config.Flags.NodeName),
		kubeletplugin.DriverName(cst.DriverName),
		kubeletplugin.RegistrarDirectoryPath(config.Flags.KubeletRegistrarDirectoryPath),
		kubeletplugin.PluginDataDirectoryPath(config.DriverPluginPath()),
	)
	if err != nil {
		return nil, err
	}
	driver.helper = helper

	if config.Flags.HealthCheckPort >= 0 {
		driver.healthcheck, err = health.StartHealthcheck(ctx, config)
		if err != nil {
			return nil, fmt.Errorf("start healthcheck: %w", err)
		}
	}

	devices := make([]resourceapi.Device, 0, len(state.Allocatable))
	for _, device := range state.Allocatable {
		devices = append(devices, device.Device)
	}
	var resources resourceslice.DriverResources
	resources.Pools = map[string]resourceslice.Pool{
		config.Flags.NodeName: {
			Slices: []resourceslice.Slice{{
				Devices: devices,
			},
			},
		},
	}

	if err := helper.PublishResources(ctx, resources); err != nil {
		return nil, err
	}

	return driver, nil
}

func (d *driver) Shutdown(logger klog.Logger) error {
	if d.healthcheck != nil {
		d.healthcheck.Stop(logger)
	}
	d.helper.Stop()
	return nil
}

func (d *driver) PrepareResourceClaims(ctx context.Context,
	claims []*resourceapi.ResourceClaim) (map[types.UID]kubeletplugin.PrepareResult, error) {
	result := make(map[types.UID]kubeletplugin.PrepareResult)

	for _, claim := range claims {
		result[claim.UID] = d.prepareResourceClaim(ctx, claim)
	}

	return result, nil
}

func (d *driver) prepareResourceClaim(_ context.Context, claim *resourceapi.ResourceClaim) kubeletplugin.PrepareResult {
	preparedPBs, err := d.state.Prepare(claim)
	if err != nil {
		return kubeletplugin.PrepareResult{
			Err: fmt.Errorf("error preparing devices for claim %v: %w", claim.UID, err),
		}
	}
	prepared := make([]kubeletplugin.Device, len(preparedPBs))
	for i, preparedPB := range preparedPBs {
		prepared[i] = kubeletplugin.Device{
			Requests:     preparedPB.GetRequestNames(),
			PoolName:     preparedPB.GetPoolName(),
			DeviceName:   preparedPB.GetDeviceName(),
			CDIDeviceIDs: preparedPB.GetCDIDeviceIDs(),
		}
	}

	klog.Infof("Returning newly prepared devices for claim '%v': %v", claim.UID, prepared)
	return kubeletplugin.PrepareResult{Devices: prepared}
}

func (d *driver) UnprepareResourceClaims(ctx context.Context, claims []kubeletplugin.NamespacedObject) (map[types.UID]error, error) {
	result := make(map[types.UID]error)

	for _, claim := range claims {
		result[claim.UID] = d.unprepareResourceClaim(ctx, claim)
	}

	return result, nil
}

func (d *driver) unprepareResourceClaim(_ context.Context, claim kubeletplugin.NamespacedObject) error {
	if err := d.state.Unprepare(string(claim.UID)); err != nil {
		return fmt.Errorf("error unpreparing devices for claim %v: %w", claim.UID, err)
	}
	return nil
}
