/*
 * +-------------------------------------------------------------------+
 * | (C) Copyright IBM Corp. 2025, 2026                                |
 * | SPDX-License-Identifier: Apache-2.0                               |
 * +-------------------------------------------------------------------+
 */
package health

import (
	"strings"

	"github.com/golang/glog"
	pb "github.com/ibm-aiu/spyre-health-checker/pkg/health/spyre"

	"github.com/ibm-aiu/dra-driver-spyre/pkg/types"
)

// DeviceHealthState holds the health state and type of a Spyre device.
type DeviceHealthState struct {
	state      pb.DEVICE_STATE
	deviceType pb.DEVICE_TYPE
}

// NewDeviceHealthState returns a new DeviceHealthState with online (healthy) state
func NewDeviceHealthState(deviceType pb.DEVICE_TYPE) *DeviceHealthState {
	return &DeviceHealthState{
		state:      pb.DEVICE_STATE_ONLINE,
		deviceType: deviceType,
	}
}

func (d *DeviceHealthState) SetHealthState(state pb.DEVICE_STATE) {
	d.state = state
}

func (d *DeviceHealthState) Healthy() bool {
	return d.state == pb.DEVICE_STATE_ONLINE
}

func (d *DeviceHealthState) GetHealthState() pb.DEVICE_STATE {
	return d.state
}

func (d *DeviceHealthState) IsSriovPF() bool {
	return d.deviceType == pb.DEVICE_TYPE_PF
}

// StateName returns the name of the state (e.g. "ONLINE", "IN_ERROR"), which is the
// value published in the device state attribute. The generated enum names are already
// bare; only DEVICE_STATE_UNSPECIFIED carries a prefix, which is dropped here.
func (d *DeviceHealthState) StateName() string {
	return strings.TrimPrefix(d.state.String(), "DEVICE_STATE_")
}

func deviceTypeOf(device types.PciDevice) pb.DEVICE_TYPE {
	if device.IsSriovPF() {
		return pb.DEVICE_TYPE_PF
	}
	return pb.DEVICE_TYPE_VF
}

// StateNames converts health states keyed by PCI address into the values published
// in the device state attribute (e.g. "ONLINE", "IN_ERROR").
func StateNames(healths map[string]DeviceHealthState) map[string]string {
	names := make(map[string]string, len(healths))
	for addr, hs := range healths {
		names[addr] = hs.StateName()
	}
	return names
}

// InitHealthInfo returns a healthy DeviceHealthState for each allocatable device keyed by PCI address.
func InitHealthInfo(allocatable types.AllocatableDevices) map[string]DeviceHealthState {
	healthInfoMap := make(map[string]DeviceHealthState, len(allocatable))
	for _, device := range allocatable {
		healthInfoMap[device.GetPciAddr()] = *NewDeviceHealthState(deviceTypeOf(device))
	}
	return healthInfoMap
}

// InitialDevices converts allocatable devices to the initial device list sent on health client registration.
func InitialDevices(allocatable types.AllocatableDevices) *pb.Devices {
	pbDevices := make([]*pb.Device, 0, len(allocatable))
	for _, device := range allocatable {
		pbDevices = append(pbDevices, &pb.Device{
			DeviceID:    &pb.DeviceID{PCIAddress: device.GetPciAddr()},
			DeviceType:  deviceTypeOf(device),
			DeviceState: pb.DEVICE_STATE_ONLINE,
		})
	}
	return &pb.Devices{Devices: pbDevices}
}

func MapsEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, aVal := range a {
		bVal, ok := b[k]
		if !ok || aVal != bVal {
			return false
		}
	}
	return true
}

// SafeTriggerUpdate sends a non-blocking notification on updateChan.
func SafeTriggerUpdate(updateChan chan struct{}) {
	select {
	case updateChan <- struct{}{}:
		glog.V(1).Info("Update triggered")
	default:
		glog.V(1).Info("Update already pending, skipping")
	}
}
