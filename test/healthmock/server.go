/*
 * +-------------------------------------------------------------------+
 * | (C) Copyright IBM Corp. 2025, 2026                                |
 * | SPDX-License-Identifier: Apache-2.0                               |
 * +-------------------------------------------------------------------+
 */

// Package healthmock is a TEST-ONLY mock of the spyre health service.
//
// It serves SpyreHealthService over mTLS on a unix socket, exactly where the
// driver's health client expects it, and lets a test change the reported state
// of a device through a small HTTP control API. It must never be deployed
// outside of test clusters.
package healthmock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	pb "github.com/ibm-aiu/spyre-health-checker/pkg/health/spyre"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	// SocketFileName is the unix socket created inside the mock directory.
	SocketFileName = "health.sock"

	statePrefix = "DEVICE_STATE_"
)

var (
	// ErrUnknownDevice is returned when a state is set for a device that has not been registered.
	ErrUnknownDevice = errors.New("unknown device")
	// ErrInvalidState is returned when a state name is not a valid DEVICE_STATE.
	ErrInvalidState = errors.New("invalid device state")
	// ErrInvalidAddress is returned when a PCI address does not look like one.
	ErrInvalidAddress = errors.New("invalid PCI address")

	pciAddressPattern = regexp.MustCompile(`^[0-9a-fA-F:.]{1,32}$`)
)

// Server is a mock spyre health service.
type Server struct {
	pb.UnimplementedSpyreHealthServiceServer

	dir string

	mu     sync.Mutex
	states map[string]pb.DEVICE_STATE
	types  map[string]pb.DEVICE_TYPE
	subs   map[chan struct{}]struct{}
}

// New returns a mock that keeps its socket and certificates in dir.
func New(dir string) *Server {
	return &Server{
		dir:    dir,
		states: map[string]pb.DEVICE_STATE{},
		types:  map[string]pb.DEVICE_TYPE{},
		subs:   map[chan struct{}]struct{}{},
	}
}

// SocketPath returns the path of the unix socket the mock listens on.
func (s *Server) SocketPath() string { return filepath.Join(s.dir, SocketFileName) }

// Serve generates the certificates, listens on the unix socket and serves
// until ctx is cancelled. The socket only appears once the certificates exist,
// so a ready socket implies usable certificates.
func (s *Server) Serve(ctx context.Context) error {
	if err := GenerateCertificates(s.dir); err != nil {
		return fmt.Errorf("generate certificates: %w", err)
	}
	creds, err := s.serverCredentials()
	if err != nil {
		return err
	}

	socket := s.SocketPath()
	if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale socket: %w", err)
	}
	lis, err := net.Listen("unix", socket)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", socket, err)
	}

	grpcServer := grpc.NewServer(grpc.Creds(creds))
	pb.RegisterSpyreHealthServiceServer(grpcServer, s)

	go func() {
		<-ctx.Done()
		grpcServer.Stop()
	}()
	slog.Info("health mock serving", "socket", socket)
	if err := grpcServer.Serve(lis); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}

// RegisterForSpyreDevicesEvents implements pb.SpyreHealthServiceServer.
func (s *Server) RegisterForSpyreDevicesEvents(_ *emptypb.Empty, stream grpc.ServerStreamingServer[pb.Devices]) error {
	return s.stream(stream)
}

// RegisterForSpyreDevicesEventsWithDevices implements pb.SpyreHealthServiceServer.
// Devices that are not known yet are tracked with the state they were registered with
// (ONLINE when unspecified); known devices keep their state across client reconnects.
func (s *Server) RegisterForSpyreDevicesEventsWithDevices(in *pb.Devices, stream grpc.ServerStreamingServer[pb.Devices]) error {
	s.track(in)
	return s.stream(stream)
}

func (s *Server) track(in *pb.Devices) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, device := range in.GetDevices() {
		addr := device.GetDeviceID().GetPCIAddress()
		if addr == "" {
			continue
		}
		if _, known := s.states[addr]; known {
			continue
		}
		state := device.GetDeviceState()
		if state == pb.DEVICE_STATE_DEVICE_STATE_UNSPECIFIED {
			state = pb.DEVICE_STATE_ONLINE
		}
		s.states[addr] = state
		s.types[addr] = device.GetDeviceType()
	}
	slog.Info("devices registered", "tracked", len(s.states))
	s.notifyLocked()
}

// stream sends the current snapshot, then a new one on every change.
func (s *Server) stream(stream grpc.ServerStreamingServer[pb.Devices]) error {
	ch := make(chan struct{}, 1)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.subs, ch)
		s.mu.Unlock()
	}()

	ch <- struct{}{} // initial snapshot
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-ch:
			if err := stream.Send(s.snapshot()); err != nil {
				return err
			}
		}
	}
}

func (s *Server) notifyLocked() {
	for ch := range s.subs {
		select {
		case ch <- struct{}{}:
		default: // a snapshot is already pending; it will carry the latest state
		}
	}
}

func (s *Server) snapshot() *pb.Devices {
	s.mu.Lock()
	defer s.mu.Unlock()
	addrs := make([]string, 0, len(s.states))
	for addr := range s.states {
		addrs = append(addrs, addr)
	}
	sort.Strings(addrs)
	devices := make([]*pb.Device, 0, len(addrs))
	for _, addr := range addrs {
		devices = append(devices, &pb.Device{
			DeviceID:    &pb.DeviceID{PCIAddress: addr},
			DeviceType:  s.types[addr],
			DeviceState: s.states[addr],
		})
	}
	return &pb.Devices{Devices: devices}
}

// Set changes the state reported for a registered device and notifies all clients.
func (s *Server) Set(addr string, state pb.DEVICE_STATE) error {
	if !pciAddressPattern.MatchString(addr) {
		return ErrInvalidAddress
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, known := s.states[addr]; !known {
		return fmt.Errorf("%w: %s", ErrUnknownDevice, addr)
	}
	s.states[addr] = state
	slog.Info("device state set", "pciAddress", addr, "state", StateName(state))
	s.notifyLocked()
	return nil
}

// States returns the state name of every registered device keyed by PCI address.
func (s *Server) States() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]string, len(s.states))
	for addr, state := range s.states {
		out[addr] = StateName(state)
	}
	return out
}

// StateName returns the short state name, e.g. "IN_ERROR".
func StateName(state pb.DEVICE_STATE) string {
	return strings.TrimPrefix(state.String(), statePrefix)
}

// ParseState parses a short state name such as "IN_ERROR" (case-insensitive).
func ParseState(name string) (pb.DEVICE_STATE, error) {
	value, ok := pb.DEVICE_STATE_value[strings.ToUpper(strings.TrimSpace(name))]
	if !ok || pb.DEVICE_STATE(value) == pb.DEVICE_STATE_DEVICE_STATE_UNSPECIFIED {
		return 0, fmt.Errorf("%w: %q", ErrInvalidState, name)
	}
	return pb.DEVICE_STATE(value), nil
}

// Handler returns the HTTP control API:
//
//	GET  /healthz                    liveness
//	GET  /state                      JSON object of PCI address -> state name
//	POST /state?addr=<pci>&state=<S> set the state of a registered device
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /state", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, s.States())
	})
	mux.HandleFunc("POST /state", func(w http.ResponseWriter, r *http.Request) {
		state, err := ParseState(r.URL.Query().Get("state"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		switch err := s.Set(r.URL.Query().Get("addr"), state); {
		case err == nil:
			writeJSON(w, http.StatusOK, s.States())
		case errors.Is(err, ErrUnknownDevice):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
	})
	return mux
}

// ServeControl serves the HTTP control API on addr until ctx is cancelled.
// A wildcard or empty host is rejected so the API is never exposed on all interfaces.
func (s *Server) ServeControl(ctx context.Context, addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid control address %q: %w", addr, err)
	}
	if ip := net.ParseIP(host); host == "" || (ip != nil && ip.IsUnspecified()) {
		return fmt.Errorf("control address %q must name a specific interface, not all interfaces", addr)
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	server := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	slog.Info("health mock control API serving", "addr", lis.Addr().String())
	if err := server.Serve(lis); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Ready reports whether the mock in dir is ready to accept clients: its socket
// exists and the certificates were written. Used by the container startup probe.
func Ready(dir string) error {
	info, err := os.Stat(filepath.Join(dir, SocketFileName))
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("%s is not a unix socket", filepath.Join(dir, SocketFileName))
	}
	for _, name := range []string{CertFileName, KeyFileName, CAFileName} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) serverCredentials() (credentials.TransportCredentials, error) {
	return ServerCredentials(s.dir)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
