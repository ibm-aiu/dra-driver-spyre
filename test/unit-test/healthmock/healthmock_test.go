/*
 * +-------------------------------------------------------------------+
 * | (C) Copyright IBM Corp. 2025, 2026                                |
 * | SPDX-License-Identifier: Apache-2.0                               |
 * +-------------------------------------------------------------------+
 */
package healthmock_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"github.com/ibm-aiu/dra-driver-spyre/internal/health"
	"github.com/ibm-aiu/dra-driver-spyre/internal/state"
	cst "github.com/ibm-aiu/dra-driver-spyre/pkg/const"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/types"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/utils"
	"github.com/ibm-aiu/dra-driver-spyre/test/healthmock"
	pb "github.com/ibm-aiu/spyre-health-checker/pkg/health/spyre"
	"github.com/jaypipes/ghw"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ParseState", func() {
	DescribeTable("accepts valid state names case-insensitively",
		func(in string, expected pb.DEVICE_STATE) {
			got, err := healthmock.ParseState(in)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal(expected))
		},
		Entry("upper", "IN_ERROR", pb.DEVICE_STATE_IN_ERROR),
		Entry("lower", "online", pb.DEVICE_STATE_ONLINE),
		Entry("padded", " OFFLINE ", pb.DEVICE_STATE_OFFLINE),
	)

	DescribeTable("rejects invalid state names",
		func(in string) {
			_, err := healthmock.ParseState(in)
			Expect(err).To(MatchError(healthmock.ErrInvalidState))
		},
		Entry("empty", ""),
		Entry("unknown", "BROKEN"),
		Entry("unspecified", "DEVICE_STATE_UNSPECIFIED"),
		Entry("unspecified short", "UNSPECIFIED"),
	)
})

var _ = Describe("health mock end to end with the driver's health pipeline", Serial, Ordered, func() {
	const (
		addr1 = "0001:00:00.0"
		addr2 = "0002:00:00.0"
	)

	var (
		dir         string
		ctx         context.Context
		cancel      context.CancelFunc
		mock        *healthmock.Server
		monitor     *health.DeviceHealthMonitor
		deviceState *state.DeviceState
		changed     chan bool
		handler     http.Handler
	)

	attributeOf := func(addr string) string {
		name := utils.PciAddressToDeviceName(addr)
		for _, device := range deviceState.Devices() {
			if device.Name == name {
				attr, ok := device.Attributes[types.DeviceStateAttribute]
				Expect(ok).To(BeTrue())
				Expect(attr.StringValue).NotTo(BeNil())
				return *attr.StringValue
			}
		}
		Fail("device not published: " + name)
		return ""
	}

	// post calls the HTTP control API and returns the status code.
	post := func(query string) int {
		req := httptest.NewRequest(http.MethodPost, "/state?"+query, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	BeforeAll(func() {
		var err error
		dir, err = os.MkdirTemp("", "healthmock-*")
		Expect(err).NotTo(HaveOccurred())

		_ = os.Setenv(cst.PseudoDeviceModeKey, cst.ModeEnabledValue)
		_ = os.Setenv(health.SpyreHealthSocketEnvKey, filepath.Join(dir, healthmock.SocketFileName))
		_ = os.Setenv(health.TLSCertPathEnvKey, filepath.Join(dir, healthmock.CertFileName))
		_ = os.Setenv(health.TLSKeyPathEnvKey, filepath.Join(dir, healthmock.KeyFileName))
		_ = os.Setenv(health.TLSCAPathEnvKey, filepath.Join(dir, healthmock.CAFileName))

		ctx, cancel = context.WithCancel(context.Background())
		mock = healthmock.New(dir)
		handler = mock.Handler()
		go func() {
			defer GinkgoRecover()
			Expect(mock.Serve(ctx)).To(Succeed())
		}()
		// The socket only appears once the certificates exist.
		Eventually(func() error { return healthmock.Ready(dir) }, 10*time.Second, 100*time.Millisecond).Should(Succeed())

		allocatable := types.NewAllocatableDevices([]*ghw.PCIDevice{
			types.GeneratePseudoDevice(addr1, types.ProductIDPf, 0),
			types.GeneratePseudoDevice(addr2, types.ProductIDPf, 0),
		}, nil, false)
		deviceState = &state.DeviceState{Allocatable: allocatable}
		changed = make(chan bool, 8)

		monitor, err = health.NewDeviceHealthMonitor(allocatable,
			func(_ context.Context, healths map[string]health.DeviceHealthState) {
				changed <- deviceState.UpdateDeviceStates(health.StateNames(healths))
			})
		Expect(err).NotTo(HaveOccurred())
		Expect(monitor.Start(ctx)).To(Succeed())
	})

	AfterAll(func() {
		monitor.Stop()
		cancel()
		_ = os.RemoveAll(dir)
		for _, key := range []string{cst.PseudoDeviceModeKey, health.SpyreHealthSocketEnvKey,
			health.TLSCertPathEnvKey, health.TLSKeyPathEnvKey, health.TLSCAPathEnvKey} {
			_ = os.Unsetenv(key)
		}
	})

	It("tracks the devices the driver registers, all ONLINE", func() {
		Eventually(mock.States, 5*time.Second).Should(Equal(map[string]string{
			addr1: "ONLINE",
			addr2: "ONLINE",
		}))
		Expect(attributeOf(addr1)).To(Equal(types.DeviceStateOnline))
		Expect(attributeOf(addr2)).To(Equal(types.DeviceStateOnline))
	})

	It("sets the device state attribute when the mock fails a device", func() {
		Expect(post("addr=" + addr2 + "&state=IN_ERROR")).To(Equal(http.StatusOK))

		Eventually(changed, 5*time.Second).Should(Receive(BeTrue()))
		Expect(attributeOf(addr1)).To(Equal(types.DeviceStateOnline))
		Expect(attributeOf(addr2)).To(Equal("IN_ERROR"))
	})

	It("restores the attribute when the mock recovers the device", func() {
		Expect(post("addr=" + addr2 + "&state=online")).To(Equal(http.StatusOK))

		Eventually(changed, 5*time.Second).Should(Receive(BeTrue()))
		Expect(attributeOf(addr2)).To(Equal(types.DeviceStateOnline))
	})

	It("reports the current states over the control API", func() {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/state", nil))
		Expect(rec.Code).To(Equal(http.StatusOK))
		var states map[string]string
		Expect(json.Unmarshal(rec.Body.Bytes(), &states)).To(Succeed())
		Expect(states).To(Equal(map[string]string{addr1: "ONLINE", addr2: "ONLINE"}))
	})

	DescribeTable("rejects bad control requests without changing anything",
		func(query string, status int) {
			Expect(post(query)).To(Equal(status))
			Consistently(changed, 500*time.Millisecond).ShouldNot(Receive())
			Expect(attributeOf(addr1)).To(Equal(types.DeviceStateOnline))
			Expect(attributeOf(addr2)).To(Equal(types.DeviceStateOnline))
		},
		Entry("unknown state", "addr="+addr1+"&state=BROKEN", http.StatusBadRequest),
		Entry("missing state", "addr="+addr1, http.StatusBadRequest),
		Entry("unknown device", "addr=0099:00:00.0&state=IN_ERROR", http.StatusNotFound),
		Entry("malformed address", "addr=../../etc&state=IN_ERROR", http.StatusBadRequest),
	)

	It("keeps the mock state when the driver reconnects", func() {
		Expect(post("addr=" + addr1 + "&state=OFFLINE")).To(Equal(http.StatusOK))
		Eventually(changed, 5*time.Second).Should(Receive(BeTrue()))
		Expect(attributeOf(addr1)).To(Equal("OFFLINE"))

		// A second client registering the same devices must not reset them to ONLINE.
		allocatable := types.NewAllocatableDevices([]*ghw.PCIDevice{
			types.GeneratePseudoDevice(addr1, types.ProductIDPf, 0),
		}, nil, false)
		second, err := health.NewDeviceHealthMonitor(allocatable, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(second.Start(ctx)).To(Succeed())
		defer second.Stop()

		Consistently(mock.States, 1*time.Second).Should(HaveKeyWithValue(addr1, "OFFLINE"))
	})
})

var _ = Describe("Ready", func() {
	It("fails before the mock has started", func() {
		Expect(healthmock.Ready(GinkgoT().TempDir())).NotTo(Succeed())
	})
})

var _ = Describe("ServeControl", func() {
	DescribeTable("refuses to listen on all interfaces",
		func(addr string) {
			err := healthmock.New(GinkgoT().TempDir()).ServeControl(context.Background(), addr)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("specific interface"))
		},
		Entry("empty host", ":9090"),
		Entry("ipv4 wildcard", "0.0.0.0:9090"),
		Entry("ipv6 wildcard", "[::]:9090"),
	)
})
