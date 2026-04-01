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
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"
	"k8s.io/dynamic-resource-allocation/resourceslice"
	"k8s.io/klog/v2"
)

const defaultDriverName = "fake.dra.hami.io"

type Options struct {
	NodeName                  string
	DriverName                string
	KubeConfig                string
	ConfigMapNamespace        string
	ConfigMapName             string
	ConfigMapKey              string
	KubeletRegistrarDirectory string
	KubeletPluginsDirectory   string
}

type RuntimeConfig struct {
	options       Options
	coreClient    coreclientset.Interface
	driverDevices map[string]resourceapi.Device
	resources     resourceslice.DriverResources
	cancelMainCtx func(error)
}

func (c RuntimeConfig) DriverPluginPath() string {
	return filepath.Join(c.options.KubeletPluginsDirectory, c.options.DriverName)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	opts := Options{
		DriverName:                defaultDriverName,
		ConfigMapNamespace:        "default",
		ConfigMapKey:              "config.yaml",
		KubeletRegistrarDirectory: kubeletplugin.KubeletRegistryDir,
		KubeletPluginsDirectory:   kubeletplugin.KubeletPluginsDir,
	}

	klog.InitFlags(nil)
	flag.StringVar(&opts.NodeName, "node-name", "", "Node name for the current kubelet plugin instance.")
	flag.StringVar(&opts.DriverName, "driver-name", opts.DriverName, "DRA driver name.")
	flag.StringVar(&opts.KubeConfig, "kubeconfig", "", "Path to kubeconfig. Leave empty when running in cluster.")
	flag.StringVar(&opts.ConfigMapNamespace, "configmap-namespace", opts.ConfigMapNamespace, "Namespace of the ConfigMap that stores fake device config.")
	flag.StringVar(&opts.ConfigMapName, "configmap-name", "", "Name of the ConfigMap that stores fake device config.")
	flag.StringVar(&opts.ConfigMapKey, "configmap-key", opts.ConfigMapKey, "Key in the ConfigMap data that contains the YAML config.")
	flag.StringVar(&opts.KubeletRegistrarDirectory, "kubelet-registrar-directory-path", opts.KubeletRegistrarDirectory, "Absolute path to kubelet plugin registration directory.")
	flag.StringVar(&opts.KubeletPluginsDirectory, "kubelet-plugins-directory-path", opts.KubeletPluginsDirectory, "Absolute path to kubelet plugin data directory.")
	flag.Parse()

	if flag.NArg() > 0 {
		return fmt.Errorf("arguments not supported: %v", flag.Args())
	}
	if opts.NodeName == "" {
		return errors.New("--node-name is required")
	}
	if opts.ConfigMapName == "" {
		return errors.New("--configmap-name is required")
	}

	return runPlugin(context.Background(), opts)
}

func runPlugin(parent context.Context, opts Options) error {
	client, err := newCoreClient(opts.KubeConfig)
	if err != nil {
		return err
	}

	node, err := client.CoreV1().Nodes().Get(parent, opts.NodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get node %q: %w", opts.NodeName, err)
	}

	cfg, err := loadDriverConfig(parent, client, opts.ConfigMapNamespace, opts.ConfigMapName, opts.ConfigMapKey)
	if err != nil {
		return err
	}

	devices, err := cfg.DevicesForNode(node)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(parent, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()
	ctx, cancel := context.WithCancelCause(ctx)

	runtimeConfig := &RuntimeConfig{
		options:       opts,
		coreClient:    client,
		driverDevices: indexDevices(devices),
		resources:     buildDriverResources(node.Name, devices),
		cancelMainCtx: cancel,
	}

	if err := os.MkdirAll(runtimeConfig.DriverPluginPath(), 0750); err != nil {
		return fmt.Errorf("create plugin directory: %w", err)
	}

	driver, err := NewDriver(ctx, runtimeConfig)
	if err != nil {
		return err
	}

	logger := klog.FromContext(ctx)
	logger.Info("fake driver started", "node", node.Name, "devices", len(devices), "configMap", fmt.Sprintf("%s/%s", opts.ConfigMapNamespace, opts.ConfigMapName))

	<-ctx.Done()
	stop()
	if cause := context.Cause(ctx); cause != nil && !errors.Is(cause, context.Canceled) {
		logger.Error(cause, "driver exited with error")
	}

	return driver.Shutdown()
}

func newCoreClient(kubeconfig string) (coreclientset.Interface, error) {
	restConfig, err := newRestConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	client, err := coreclientset.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}
	return client, nil
}

func newRestConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig == "" {
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("create in-cluster config: %w", err)
		}
		return config, nil
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("create kubeconfig client: %w", err)
	}
	return config, nil
}

func buildDriverResources(nodeName string, devices []resourceapi.Device) resourceslice.DriverResources {
	return resourceslice.DriverResources{
		Pools: map[string]resourceslice.Pool{
			nodeName: {
				Slices: []resourceslice.Slice{
					{
						Devices: devices,
					},
				},
			},
		},
	}
}

func indexDevices(devices []resourceapi.Device) map[string]resourceapi.Device {
	index := make(map[string]resourceapi.Device, len(devices))
	for _, device := range devices {
		index[device.Name] = device
	}
	return index
}

func nodeLabels(node *corev1.Node) map[string]string {
	if node == nil {
		return nil
	}
	return node.Labels
}
