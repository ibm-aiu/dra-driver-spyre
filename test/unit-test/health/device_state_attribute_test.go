/*
 * +-------------------------------------------------------------------+
 * | (C) Copyright IBM Corp. 2025, 2026                                |
 * | SPDX-License-Identifier: Apache-2.0                               |
 * +-------------------------------------------------------------------+
 */
package health_test

import (
	"context"
	"os"
	"time"

	pb "github.com/ibm-aiu/spyre-health-checker/pkg/health/spyre"
	"github.com/jaypipes/ghw"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/ibm-aiu/dra-driver-spyre/internal/health"
	"github.com/ibm-aiu/dra-driver-spyre/internal/state"
	cst "github.com/ibm-aiu/dra-driver-spyre/pkg/const"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/types"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/utils"
)

// These specs mock the spyre health service with DummyServer and check that the
// reported health ends up in the published deviceState attribute of DeviceState.
var _ = Describe("device state attribute driven by the health service", Serial, Ordered, func() {
	const (
		addr1 = "0001:00:00.0"
		addr2 = "0002:00:00.0"
		addr3 = "0003:00:00.0"
	)

	var (
		ctx         context.Context
		cancel      context.CancelFunc
		server      *DummyServer
		monitor     *DeviceHealthMonitor
		deviceState *state.DeviceState
		// changed receives, for every health update, whether DeviceState reported
		// an attribute change (i.e. whether the driver would republish the slice).
		changed chan bool
	)

	// attributeOf returns the published deviceState attribute of the device at addr.
	attributeOf := func(addr string) string {
		name := utils.PciAddressToDeviceName(addr)
		for _, device := range deviceState.Devices() {
			if device.Name == name {
				attr, ok := device.Attributes[types.DeviceStateAttribute]
				Expect(ok).To(BeTrue(), "device %s has no %s attribute", name, types.DeviceStateAttribute)
				Expect(attr.StringValue).NotTo(BeNil())
				return *attr.StringValue
			}
		}
		Fail("device not published: " + name)
		return ""
	}

	// report makes the mocked health service report the given states and waits
	// until the monitor has applied them to DeviceState. It returns whether the
	// attribute changed.
	report := func(devices ...SimplifiedDevice) bool {
		server.UpdateDeviceState(devices)
		var didChange bool
		Eventually(changed, 5*time.Second).Should(Receive(&didChange))
		return didChange
	}

	BeforeAll(func() {
		_ = os.Setenv(TLSCertPathEnvKey, "/tmp/certs/tls.crt")
		_ = os.Setenv(TLSKeyPathEnvKey, "/tmp/certs/tls.key")
		_ = os.Setenv(TLSCAPathEnvKey, "/tmp/certs/tls.crt")
		_ = os.Setenv(cst.PseudoDeviceModeKey, cst.ModeEnabledValue)
		Expect(createDummyTLSCertificates()).To(Succeed())

		// Build the allocatable devices the same way the driver does, so the
		// default attribute comes from the production code and not from the test.
		hwDevices := []*ghw.PCIDevice{
			types.GeneratePseudoDevice(addr1, types.ProductIDPf, 0),
			types.GeneratePseudoDevice(addr2, types.ProductIDPf, 0),
			types.GeneratePseudoDevice(addr3, types.ProductIDPf, 0),
		}
		allocatable := types.NewAllocatableDevices(hwDevices, nil, false)
		Expect(allocatable).To(HaveLen(len(hwDevices)))
		deviceState = &state.DeviceState{Allocatable: allocatable}

		ctx, cancel = context.WithCancel(context.Background())
		server = NewDummyServer()
		changed = make(chan bool, 8)

		var err error
		monitor, err = NewDeviceHealthMonitor(allocatable,
			func(_ context.Context, healths map[string]DeviceHealthState) {
				// Same steps as the driver's onDeviceHealthChanged, minus the publish.
				changed <- deviceState.UpdateDeviceStates(StateNames(healths))
			})
		Expect(err).NotTo(HaveOccurred())
		Expect(monitor.Start(ctx)).To(Succeed())
	})

	AfterAll(func() {
		monitor.Stop()
		server.Stop()
		cancel()
		cleanupDummyTLSCertificates()
		_ = os.Unsetenv(cst.PseudoDeviceModeKey)
		_ = os.Unsetenv(TLSCertPathEnvKey)
		_ = os.Unsetenv(TLSKeyPathEnvKey)
		_ = os.Unsetenv(TLSCAPathEnvKey)
	})

	It("publishes every device as ONLINE before any report", func() {
		Expect(attributeOf(addr1)).To(Equal(types.DeviceStateOnline))
		Expect(attributeOf(addr2)).To(Equal(types.DeviceStateOnline))
		Expect(attributeOf(addr3)).To(Equal(types.DeviceStateOnline))
	})

	It("keeps ONLINE when the service reports healthy devices", func() {
		// The first report differs from the client's empty cache, so it is delivered,
		// but it must not change any attribute.
		didChange := report(
			SimplifiedDevice{PciAddress: addr1, State: pb.DEVICE_STATE_ONLINE},
			SimplifiedDevice{PciAddress: addr2, State: pb.DEVICE_STATE_ONLINE},
			SimplifiedDevice{PciAddress: addr3, State: pb.DEVICE_STATE_ONLINE},
		)
		Expect(didChange).To(BeFalse())
		Expect(attributeOf(addr1)).To(Equal(types.DeviceStateOnline))
		Expect(attributeOf(addr2)).To(Equal(types.DeviceStateOnline))
		Expect(attributeOf(addr3)).To(Equal(types.DeviceStateOnline))
	})

	It("updates only the failing device to IN_ERROR", func() {
		didChange := report(
			SimplifiedDevice{PciAddress: addr1, State: pb.DEVICE_STATE_ONLINE},
			SimplifiedDevice{PciAddress: addr2, State: pb.DEVICE_STATE_IN_ERROR},
			SimplifiedDevice{PciAddress: addr3, State: pb.DEVICE_STATE_ONLINE},
		)
		Expect(didChange).To(BeTrue(), "the driver must republish the ResourceSlice")
		Expect(attributeOf(addr1)).To(Equal(types.DeviceStateOnline))
		Expect(attributeOf(addr2)).To(Equal("IN_ERROR"))
		Expect(attributeOf(addr3)).To(Equal(types.DeviceStateOnline))
	})

	It("tracks other state transitions with their state name", func() {
		didChange := report(
			SimplifiedDevice{PciAddress: addr1, State: pb.DEVICE_STATE_OFFLINE},
			SimplifiedDevice{PciAddress: addr2, State: pb.DEVICE_STATE_IN_ERROR},
			SimplifiedDevice{PciAddress: addr3, State: pb.DEVICE_STATE_ONLINE},
		)
		Expect(didChange).To(BeTrue())
		Expect(attributeOf(addr1)).To(Equal("OFFLINE"))
		Expect(attributeOf(addr2)).To(Equal("IN_ERROR"))
		Expect(attributeOf(addr3)).To(Equal(types.DeviceStateOnline))
	})

	It("does not trigger an update when the report is unchanged", func() {
		server.UpdateDeviceState([]SimplifiedDevice{
			{PciAddress: addr1, State: pb.DEVICE_STATE_OFFLINE},
			{PciAddress: addr2, State: pb.DEVICE_STATE_IN_ERROR},
			{PciAddress: addr3, State: pb.DEVICE_STATE_ONLINE},
		})
		Consistently(changed, 1*time.Second).ShouldNot(Receive())
		Expect(attributeOf(addr1)).To(Equal("OFFLINE"))
		Expect(attributeOf(addr2)).To(Equal("IN_ERROR"))
	})

	It("returns the devices to ONLINE when they recover", func() {
		didChange := report(
			SimplifiedDevice{PciAddress: addr1, State: pb.DEVICE_STATE_ONLINE},
			SimplifiedDevice{PciAddress: addr2, State: pb.DEVICE_STATE_ONLINE},
			SimplifiedDevice{PciAddress: addr3, State: pb.DEVICE_STATE_ONLINE},
		)
		Expect(didChange).To(BeTrue())
		Expect(attributeOf(addr1)).To(Equal(types.DeviceStateOnline))
		Expect(attributeOf(addr2)).To(Equal(types.DeviceStateOnline))
		Expect(attributeOf(addr3)).To(Equal(types.DeviceStateOnline))
	})

	It("ignores a reported device that is not allocatable", func() {
		didChange := report(
			SimplifiedDevice{PciAddress: addr1, State: pb.DEVICE_STATE_ONLINE},
			SimplifiedDevice{PciAddress: addr2, State: pb.DEVICE_STATE_ONLINE},
			SimplifiedDevice{PciAddress: addr3, State: pb.DEVICE_STATE_ONLINE},
			SimplifiedDevice{PciAddress: "0099:00:00.0", State: pb.DEVICE_STATE_IN_ERROR},
		)
		Expect(didChange).To(BeFalse())
		Expect(attributeOf(addr1)).To(Equal(types.DeviceStateOnline))
		Expect(attributeOf(addr2)).To(Equal(types.DeviceStateOnline))
		Expect(attributeOf(addr3)).To(Equal(types.DeviceStateOnline))
	})
})
