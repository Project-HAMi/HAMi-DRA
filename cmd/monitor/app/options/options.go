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

package options

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/errors"
)

const (
	defaultCollectInterval = 30 * time.Second
)

// Options contains everything necessary to create and run monitor server.
type Options struct {
	// KubeAPIQPS is the QPS to use while talking with kube-apiserver.
	KubeAPIQPS float32
	// KubeAPIBurst is the burst to allow while talking with kube-apiserver.
	KubeAPIBurst int
	// MetricsBindAddress is the TCP address that the controller should bind to
	// for serving prometheus metrics.
	// It can be set to "0" to disable the metrics serving.
	// Defaults to ":8080".
	MetricsBindAddress string
	// HealthProbeBindAddress is the TCP address that the controller should bind to
	// for serving health probes
	// Defaults to ":8000".
	HealthProbeBindAddress string
	// CollectInterval is the interval at which metrics are collected.
	// Defaults to 30s.
	CollectInterval time.Duration
}

// NewOptions builds an empty options.
func NewOptions() *Options {
	return &Options{}
}

// AddFlags adds flags to the specified FlagSet.
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	flags.Float32Var(&o.KubeAPIQPS, "kube-api-qps", 40.0, "QPS to use while talking with kube-apiserver.")
	flags.IntVar(&o.KubeAPIBurst, "kube-api-burst", 60, "Burst to use while talking with kube-apiserver.")
	flags.StringVar(&o.MetricsBindAddress, "metrics-bind-address", ":8080", "The TCP address that the controller should bind to for serving prometheus metrics(e.g. 127.0.0.1:8080, :8080). It can be set to \"0\" to disable the metrics serving.")
	flags.StringVar(&o.HealthProbeBindAddress, "health-probe-bind-address", ":8000", "The TCP address that the controller should bind to for serving health probes(e.g. 127.0.0.1:8000, :8000)")
	flags.DurationVar(&o.CollectInterval, "collect-interval", defaultCollectInterval, "The interval at which metrics are collected.")
}

// Validate validates the options and returns aggregated errors.
func (o *Options) Validate() error {
	var errs []error

	if o.KubeAPIQPS <= 0 {
		errs = append(errs, fmt.Errorf("--kube-api-qps must be greater than 0"))
	}

	if o.KubeAPIBurst <= 0 {
		errs = append(errs, fmt.Errorf("--kube-api-burst must be greater than 0"))
	}

	if o.CollectInterval <= 0 {
		errs = append(errs, fmt.Errorf("--collect-interval must be greater than 0"))
	}

	return errors.NewAggregate(errs)
}
