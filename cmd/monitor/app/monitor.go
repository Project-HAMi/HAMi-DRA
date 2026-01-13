/*
Copyright 2025 The HAMi Authors.

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

package app

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/klog/v2"

	"github.com/Project-HAMi/HAMi-DRA/cmd/monitor/app/options"
	"github.com/Project-HAMi/HAMi-DRA/pkg/cache"
	"github.com/Project-HAMi/HAMi-DRA/pkg/metrics"
	"github.com/Project-HAMi/HAMi-DRA/pkg/version"
)

// NewMonitorCommand creates a *cobra.Command object with default parameters
func NewMonitorCommand(ctx context.Context) *cobra.Command {
	logConfig := logsv1.NewLoggingConfiguration()
	fss := cliflag.NamedFlagSets{}

	logsFlagSet := fss.FlagSet("logs")
	logs.AddFlags(logsFlagSet, logs.SkipLoggingConfigurationFlags())
	logsv1.AddFlags(logConfig, logsFlagSet)

	genericFlagSet := fss.FlagSet("generic")
	opts := options.NewOptions()
	genericFlagSet.AddGoFlagSet(flag.CommandLine)
	opts.AddFlags(genericFlagSet)

	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "A Kubernetes monitor server for DRA resources",
		Long: `The monitor server collects metrics about Dynamic Resource Allocation (DRA) resources
and exposes them via Prometheus metrics endpoint.`,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if err := logsv1.ValidateAndApply(logConfig, nil); err != nil {
				return err
			}
			logs.InitLogs()
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			// validate options
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := Run(ctx, opts); err != nil {
				return err
			}
			return nil
		},
		Args: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if len(arg) > 0 {
					return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), args)
				}
			}
			return nil
		},
	}

	cmd.Flags().AddFlagSet(genericFlagSet)
	cmd.Flags().AddFlagSet(logsFlagSet)

	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		return nil
	})
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n", cmd.Long)
		fmt.Fprintf(cmd.OutOrStdout(), "Usage:\n  %s\n\n", cmd.UseLine())
		fmt.Fprintf(cmd.OutOrStdout(), "Flags:\n%s\n", cmd.Flags().FlagUsages())
	})
	return cmd
}

// Run runs the monitor server with options. This should never exit.
func Run(ctx context.Context, opts *options.Options) error {
	klog.Infof("hami-dra-monitor version: %s", version.Get())
	klog.InfoS("Golang settings", "GOGC", os.Getenv("GOGC"), "GOMAXPROCS", os.Getenv("GOMAXPROCS"), "GOTRACEBACK", os.Getenv("GOTRACEBACK"))

	// Initialize cache
	klog.Info("Initializing cache...")
	cacheInstance := cache.NewCache()
	if err := cacheInstance.Start(); err != nil {
		klog.Errorf("Failed to start cache: %v", err)
		return err
	}
	defer cacheInstance.Stop()

	// Wait for cache to be ready
	for !cacheInstance.IsReady() {
		time.Sleep(100 * time.Millisecond)
	}
	klog.Info("Cache is ready")

	// Create a registry that only contains our custom metrics
	// This prevents exposing Go runtime metrics (go_*, process_*, etc.)
	customRegistry := prometheus.NewRegistry()

	// Create metrics collector and register to registry
	collector := metrics.NewCollector(cacheInstance)
	klog.Info("Registering metrics collector to registry")
	customRegistry.MustRegister(collector)

	// Create HTTP server for metrics with registry
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.HandlerFor(customRegistry, promhttp.HandlerOpts{}))
	metricsServer := &http.Server{
		Addr:    opts.MetricsBindAddress,
		Handler: metricsMux,
	}
	go func() {
		klog.Infof("Starting metrics server on %s", opts.MetricsBindAddress)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Errorf("Metrics server failed: %v", err)
		}
	}()

	// Create HTTP server for health probes
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			klog.Errorf("Failed to write healthz response: %v", err)
		}
	})
	healthMux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if cacheInstance.IsReady() {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("ready")); err != nil {
				klog.Errorf("Failed to write readyz response: %v", err)
			}
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			if _, err := w.Write([]byte("not ready")); err != nil {
				klog.Errorf("Failed to write readyz response: %v", err)
			}
		}
	})
	healthServer := &http.Server{
		Addr:    opts.HealthProbeBindAddress,
		Handler: healthMux,
	}
	go func() {
		klog.Infof("Starting health probe server on %s", opts.HealthProbeBindAddress)
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Errorf("Health probe server failed: %v", err)
		}
	}()

	// Create monitor
	monitor := NewMonitor(collector, opts.CollectInterval)

	// Start monitor in a goroutine
	go func() {
		klog.Infof("Starting monitor with collect interval: %v", opts.CollectInterval)
		monitor.Run(ctx)
	}()

	// Wait for context to be done
	<-ctx.Done()
	klog.Info("Shutting down servers...")

	// Shutdown servers gracefully
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		klog.Errorf("Error shutting down metrics server: %v", err)
	}

	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		klog.Errorf("Error shutting down health probe server: %v", err)
	}

	return nil
}

type Monitor struct {
	collector *metrics.Collector
	interval  time.Duration
}

func NewMonitor(collector *metrics.Collector, interval time.Duration) *Monitor {
	return &Monitor{
		collector: collector,
		interval:  interval,
	}
}

func (m *Monitor) Run(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	// Note: Prometheus will automatically call Collect() when scraping metrics
	// We don't need to manually trigger collection here, but we can log that we're ready
	klog.Info("Monitor is running, metrics will be collected on demand by Prometheus")

	for {
		select {
		case <-ticker.C:
			// Prometheus handles collection automatically, but we can use this ticker for other purposes
			// For now, just log that we're alive
			klog.V(5).Info("Monitor tick")
		case <-ctx.Done():
			klog.Info("Monitor stopped")
			return
		}
	}
}
