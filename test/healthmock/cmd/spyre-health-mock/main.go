/*
 * +-------------------------------------------------------------------+
 * | (C) Copyright IBM Corp. 2025, 2026                                |
 * | SPDX-License-Identifier: Apache-2.0                               |
 * +-------------------------------------------------------------------+
 */

// Command spyre-health-mock runs the TEST-ONLY mock of the spyre health service.
//
//	spyre-health-mock serve [--dir DIR] [--control-addr HOST:PORT]
//	spyre-health-mock ready [--dir DIR]
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ibm-aiu/dra-driver-spyre/test/healthmock"
)

const defaultDir = "/var/run/spyre-health"

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))

	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "serve":
		os.Exit(serve(os.Args[2:]))
	case "ready":
		os.Exit(ready(os.Args[2:]))
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: spyre-health-mock serve|ready [flags]")
	os.Exit(2)
}

func serve(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	dir := fs.String("dir", defaultDir, "directory for the unix socket and the generated certificates")
	controlAddr := fs.String("control-addr", "127.0.0.1:9090", "HTTP control API address (must not be a wildcard address)")
	_ = fs.Parse(args)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	mock := healthmock.New(*dir)
	errc := make(chan error, 2)
	go func() { errc <- mock.ServeControl(ctx, *controlAddr) }()
	go func() { errc <- mock.Serve(ctx) }()

	select {
	case <-ctx.Done():
		return 0
	case err := <-errc:
		if err != nil {
			slog.Error("health mock failed", "error", err.Error())
			return 1
		}
		return 0
	}
}

func ready(args []string) int {
	fs := flag.NewFlagSet("ready", flag.ExitOnError)
	dir := fs.String("dir", defaultDir, "directory of the unix socket and certificates")
	_ = fs.Parse(args)

	if err := healthmock.Ready(*dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
