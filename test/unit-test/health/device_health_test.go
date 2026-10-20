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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/ibm-aiu/dra-driver-spyre/internal/health"
)

var _ = Describe("DeviceHealthState.StateName", func() {
	DescribeTable("returns the short state name",
		func(state pb.DEVICE_STATE, expected string) {
			s := NewDeviceHealthState(pb.DEVICE_TYPE_PF)
			s.SetHealthState(state)
			Expect(s.StateName()).To(Equal(expected))
		},
		Entry("online", pb.DEVICE_STATE_ONLINE, "ONLINE"),
		Entry("in error", pb.DEVICE_STATE_IN_ERROR, "IN_ERROR"),
		Entry("offline", pb.DEVICE_STATE_OFFLINE, "OFFLINE"),
	)
})

var _ = Describe("DeviceHealthMonitor", Serial, Ordered, func() {
	BeforeAll(func() {
		_ = os.Setenv(TLSCertPathEnvKey, "/tmp/certs/tls.crt")
		_ = os.Setenv(TLSKeyPathEnvKey, "/tmp/certs/tls.key")
		_ = os.Setenv(TLSCAPathEnvKey, "/tmp/certs/tls.crt")
		Expect(createDummyTLSCertificates()).To(Succeed())
	})

	AfterAll(func() {
		cleanupDummyTLSCertificates()
		_ = os.Unsetenv(TLSCertPathEnvKey)
		_ = os.Unsetenv(TLSKeyPathEnvKey)
		_ = os.Unsetenv(TLSCAPathEnvKey)
	})

	It("is not created when the spyre health socket is not configured", func() {
		_ = os.Unsetenv(SpyreHealthSocketEnvKey)
		_, err := NewDeviceHealthMonitor(createAllocatable("0001:00:00.0"), nil)
		Expect(err).To(HaveOccurred())
	})

	It("reports reported device health through the callback and Healths()", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		server := NewDummyServer()
		defer server.Stop()

		got := make(chan map[string]DeviceHealthState, 4)
		monitor, err := NewDeviceHealthMonitor(
			createAllocatable("0001:00:00.0", "0002:00:00.0"),
			func(_ context.Context, healths map[string]DeviceHealthState) { got <- healths },
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(monitor.Start(ctx)).To(Succeed())
		defer monitor.Stop()

		By("starting with every device healthy")
		Expect(monitor.Healths()).To(Equal(map[string]DeviceHealthState{
			"0001:00:00.0": healthyDeviceState,
			"0002:00:00.0": healthyDeviceState,
		}))

		By("receiving an unhealthy device")
		server.UpdateDeviceState([]SimplifiedDevice{
			{PciAddress: "0001:00:00.0", State: pb.DEVICE_STATE_ONLINE},
			{PciAddress: "0002:00:00.0", State: pb.DEVICE_STATE_IN_ERROR},
		})
		var healths map[string]DeviceHealthState
		Eventually(got, 5*time.Second).Should(Receive(&healths))
		Expect(healths).To(Equal(map[string]DeviceHealthState{
			"0001:00:00.0": healthyDeviceState,
			"0002:00:00.0": unhealthyDeviceState,
		}))
		Expect(monitor.Healths()).To(Equal(healths))
	})
})
