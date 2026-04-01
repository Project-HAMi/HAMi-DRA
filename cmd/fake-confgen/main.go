/*
Copyright 2026 The HAMi Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Project-HAMi/HAMi-DRA/internal/configmapgen"
	"k8s.io/klog/v2"
)

type options struct {
	ListenAddress string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	opts := options{
		ListenAddress: ":8080",
	}

	klog.InitFlags(nil)
	flag.StringVar(&opts.ListenAddress, "listen-address", opts.ListenAddress, "HTTP listen address.")
	flag.Parse()
	if flag.NArg() > 0 {
		return fmt.Errorf("arguments not supported: %v", flag.Args())
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	server, err := configmapgen.StartServer(ctx, opts.ListenAddress)
	if err != nil {
		return err
	}

	klog.InfoS("fake confgen started", "addr", opts.ListenAddress)
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
