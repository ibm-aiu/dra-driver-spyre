/*
 * +-------------------------------------------------------------------+
 * | (C) Copyright IBM Corp. 2025, 2026                                |
 * | SPDX-License-Identifier: Apache-2.0                               |
 * +-------------------------------------------------------------------+
 */
package health

import (
	"context"
	"sync"

	pb "github.com/ibm-aiu/spyre-health-checker/pkg/health/spyre"

	"github.com/ibm-aiu/dra-driver-spyre/pkg/types"
)

// OnChangeFunc is called with the latest health state keyed by PCI address
// every time the spyre health service reports an update.
type OnChangeFunc func(ctx context.Context, healths map[string]DeviceHealthState)

// DeviceHealthMonitor tracks the health of the allocatable Spyre devices
// reported by the spyre health service. It is independent of the driver
// liveness probe implemented by HealthCheck.
type DeviceHealthMonitor struct {
	client   *SpyreHealthClient
	initial  *pb.Devices
	base     map[string]DeviceHealthState
	onChange OnChangeFunc

	updateChan chan struct{}
	stopChan   chan struct{}
	wg         sync.WaitGroup

	mu      sync.RWMutex
	healths map[string]DeviceHealthState
}

// NewDeviceHealthMonitor returns a monitor for the given allocatable devices.
// The devices are snapshotted here, so the map may be modified afterwards.
// It returns an error if the spyre health socket is not configured.
func NewDeviceHealthMonitor(allocatable types.AllocatableDevices, onChange OnChangeFunc) (*DeviceHealthMonitor, error) {
	client, err := NewSpyreHealthClient()
	if err != nil {
		return nil, err
	}
	base := InitHealthInfo(allocatable)
	return &DeviceHealthMonitor{
		client:     client,
		initial:    InitialDevices(allocatable),
		base:       base,
		onChange:   onChange,
		updateChan: make(chan struct{}, 1),
		stopChan:   make(chan struct{}),
		healths:    copyHealths(base),
	}, nil
}

// Start registers the allocatable devices with the spyre health service
// and applies the health updates it streams back.
func (m *DeviceHealthMonitor) Start(ctx context.Context) error {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.processUpdate(ctx)
	}()
	return m.client.Start(ctx, m.updateChan, m.initial)
}

// Stop stops the health client and the update loop.
func (m *DeviceHealthMonitor) Stop() {
	m.client.Stop()
	close(m.stopChan)
	m.wg.Wait()
}

// Healths returns a snapshot of the latest health state keyed by PCI address.
func (m *DeviceHealthMonitor) Healths() map[string]DeviceHealthState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return copyHealths(m.healths)
}

func (m *DeviceHealthMonitor) processUpdate(ctx context.Context) {
	for {
		select {
		case <-m.updateChan:
			healths := copyHealths(m.base)
			m.client.UpdateHealths(healths)
			m.mu.Lock()
			m.healths = healths
			m.mu.Unlock()
			if m.onChange != nil {
				m.onChange(ctx, copyHealths(healths))
			}
		case <-m.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

func copyHealths(in map[string]DeviceHealthState) map[string]DeviceHealthState {
	out := make(map[string]DeviceHealthState, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
