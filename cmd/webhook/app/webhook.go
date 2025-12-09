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
	"crypto/tls"
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/flowcontrol"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Project-HAMi/HAMi-DRA/cmd/webhook/app/options"
	"github.com/Project-HAMi/HAMi-DRA/pkg/config"
	"github.com/Project-HAMi/HAMi-DRA/pkg/webhook/dra"
)

// NewWebhookCommand creates a *cobra.Command object with default parameters
func NewWebhookCommand(ctx context.Context) *cobra.Command {
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
		Use:   "webhook",
		Short: "A Kubernetes webhook server template",
		Long: `The webhook server starts a webhook server and manages policies about how to mutate and validate
Kubernetes resources.`,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if err := logsv1.ValidateAndApply(logConfig, nil); err != nil {
				return err
			}
			logs.InitLogs()

			// Starting from version 0.15.0, controller-runtime expects its consumers to set a logger through log.SetLogger.
			controllerruntime.SetLogger(klog.Background())
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

// Run runs the webhook server with options. This should never exit.
func Run(ctx context.Context, opts *options.Options) error {
	klog.Infof("webhook server version: %s", "v1.0.0")
	deviceConfigBytes, err := os.ReadFile(opts.DeviceConfigFile)
	if err != nil {
		klog.Errorf("Failed to read device config file: %v", err)
		return err
	}
	deviceConfig, err := config.Unmarshal(deviceConfigBytes)
	if err != nil {
		klog.Errorf("Failed to unmarshal device config: %v", err)
		return err
	}
	// Create a new scheme and add default Kubernetes schemes
	sch := runtime.NewScheme()
	_ = scheme.AddToScheme(sch)

	config, err := controllerruntime.GetConfig()
	if err != nil {
		panic(err)
	}
	config.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(opts.KubeAPIQPS, opts.KubeAPIBurst)

	hookManager, err := controllerruntime.NewManager(config, controllerruntime.Options{
		Logger: klog.Background(),
		Scheme: sch,
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:     opts.BindAddress,
			Port:     opts.SecurePort,
			CertDir:  opts.CertDir,
			CertName: opts.CertName,
			KeyName:  opts.KeyName,
			TLSOpts: []func(*tls.Config){
				func(config *tls.Config) {
					// Just transform the valid options as opts.TLSMinVersion
					// can only accept "1.0", "1.1", "1.2", "1.3" and has default
					// value,
					switch opts.TLSMinVersion {
					case "1.0":
						config.MinVersion = tls.VersionTLS10
					case "1.1":
						config.MinVersion = tls.VersionTLS11
					case "1.2":
						config.MinVersion = tls.VersionTLS12
					case "1.3":
						config.MinVersion = tls.VersionTLS13
					}
				},
			},
		}),
		LeaderElection:         false,
		Metrics:                metricsserver.Options{BindAddress: opts.MetricsBindAddress},
		HealthProbeBindAddress: opts.HealthProbeBindAddress,
	})
	if err != nil {
		klog.Errorf("Failed to build webhook server: %v", err)
		return err
	}

	decoder := admission.NewDecoder(hookManager.GetScheme())

	klog.Info("Registering webhooks to the webhook server")
	hookServer := hookManager.GetWebhookServer()

	mutatingAdmission := &dra.MutatingAdmission{}
	mutatingAdmission.Decoder = decoder
	mutatingAdmission.Client = hookManager.GetClient()
	mutatingAdmission.DeviceConfig = deviceConfig
	hookServer.Register("/mutate", &webhook.Admission{Handler: mutatingAdmission})
	validatingAdmission := &dra.ValidatingAdmission{}
	validatingAdmission.Decoder = decoder
	validatingAdmission.Client = hookManager.GetClient()
	hookServer.Register("/validate", &webhook.Admission{Handler: validatingAdmission})
	// blocks until the context is done.
	if err := hookManager.Start(ctx); err != nil {
		klog.Errorf("webhook server exits unexpectedly: %v", err)
		return err
	}

	// never reach here
	return nil
}
