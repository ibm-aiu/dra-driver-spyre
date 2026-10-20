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

package driver

import (
	"context"
	"errors"
	"fmt"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	coreclientset "k8s.io/client-go/kubernetes"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"
	klog "k8s.io/klog/v2"

	"github.com/ibm-aiu/dra-driver-spyre/internal/health"
	"github.com/ibm-aiu/dra-driver-spyre/internal/state"
	cst "github.com/ibm-aiu/dra-driver-spyre/pkg/const"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/flags"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/utils"
	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/dynamic-resource-allocation/resourceslice"
)

var _ kubeletplugin.DRAPlugin = &driver{}

type driver struct {
	client        coreclientset.Interface
	helper        *kubeletplugin.Helper
	state         *state.DeviceState
	healthcheck   *health.HealthCheck
	deviceHealthy *health.DeviceHealthMonitor
	nodeName      string
	cancelCtx     func(error)
}

// NewDriver creates a driver with device state and kubelet plugin
// DeviceState discovers allocatable devices
// (using a call to traditional device plugin function)
// KubeletPlugin publishes resources for generating ResourceSlice.
func NewDriver(ctx context.Context, config *flags.Config) (*driver, error) {
	driver := &driver{
		client:    config.Coreclient,
		cancelCtx: config.CancelMainCtx,
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

	driver.nodeName = config.Flags.NodeName
	if err := driver.publishResources(ctx); err != nil {
		return nil, err
	}

	// Device health is optional: it is enabled only when the spyre health service is configured.
	// It starts after the first publish so that health updates always republish on top of it.
	monitor, err := health.NewDeviceHealthMonitor(state.Allocatable, driver.onDeviceHealthChanged)
	if err != nil {
		klog.Infof("Spyre device health monitoring is disabled: %v", err)
		return driver, nil
	}
	if err := monitor.Start(ctx); err != nil {
		return nil, fmt.Errorf("start device health monitor: %w", err)
	}
	driver.deviceHealthy = monitor

	return driver, nil
}

// publishResources publishes the allocatable devices, including their current device state attribute.
func (d *driver) publishResources(ctx context.Context) error {
	var resources resourceslice.DriverResources
	resources.Pools = map[string]resourceslice.Pool{
		d.nodeName: {
			Slices: []resourceslice.Slice{{
				Devices: d.state.Devices(),
			}},
		},
	}
	return d.helper.PublishResources(ctx, resources)
}

// onDeviceHealthChanged reflects the reported device health into the device state attribute
// and republishes the ResourceSlice if any value changed.
func (d *driver) onDeviceHealthChanged(ctx context.Context, healths map[string]health.DeviceHealthState) {
	if !d.state.UpdateDeviceStates(health.StateNames(healths)) {
		return
	}
	if err := d.publishResources(ctx); err != nil {
		klog.ErrorS(err, "failed to republish resources after device state change")
	}
}

func (d *driver) Shutdown(logger klog.Logger) error {
	if d.deviceHealthy != nil {
		d.deviceHealthy.Stop()
	}
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
			CDIDeviceIDs: preparedPB.GetCdiDeviceIds(),
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

func (d *driver) HandleError(ctx context.Context, err error, msg string) {
	utilruntime.HandleErrorWithContext(ctx, err, msg)
	if !errors.Is(err, kubeletplugin.ErrRecoverable) && d.cancelCtx != nil {
		d.cancelCtx(fmt.Errorf("fatal background error: %w", err))
	}
}
