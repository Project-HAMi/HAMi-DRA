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
	"fmt"

	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"
	"k8s.io/klog/v2"
)

type driver struct {
	driverName string
	devices    map[string]resourceapi.Device
	helper     *kubeletplugin.Helper
	cancelCtx  func(error)
}

func NewDriver(ctx context.Context, config *RuntimeConfig) (*driver, error) {
	driver := &driver{
		driverName: config.options.DriverName,
		devices:    config.driverDevices,
		cancelCtx:  config.cancelMainCtx,
	}

	helper, err := kubeletplugin.Start(
		ctx,
		driver,
		kubeletplugin.KubeClient(config.coreClient),
		kubeletplugin.NodeName(config.options.NodeName),
		kubeletplugin.DriverName(config.options.DriverName),
		kubeletplugin.RegistrarDirectoryPath(config.options.KubeletRegistrarDirectory),
		kubeletplugin.PluginDataDirectoryPath(config.DriverPluginPath()),
	)
	if err != nil {
		return nil, err
	}
	driver.helper = helper

	if err := helper.PublishResources(ctx, config.resources); err != nil {
		helper.Stop()
		return nil, fmt.Errorf("publish resources: %w", err)
	}

	return driver, nil
}

func (d *driver) Shutdown() error {
	if d.helper != nil {
		d.helper.Stop()
	}
	return nil
}

func (d *driver) PrepareResourceClaims(ctx context.Context, claims []*resourceapi.ResourceClaim) (map[types.UID]kubeletplugin.PrepareResult, error) {
	logger := klog.FromContext(ctx)
	logger.Info("PrepareResourceClaims is called", "numClaims", len(claims))
	result := make(map[types.UID]kubeletplugin.PrepareResult)

	for _, claim := range claims {
		result[claim.UID] = d.prepareResourceClaim(ctx, claim)
	}

	return result, nil
}

func (d *driver) prepareResourceClaim(ctx context.Context, claim *resourceapi.ResourceClaim) kubeletplugin.PrepareResult {
	logger := klog.FromContext(ctx)
	logger.Info("Preparing claim", "uid", claim.UID, "namespace", claim.Namespace, "name", claim.Name)

	if claim.Status.Allocation == nil {
		err := fmt.Errorf("claim has no allocation")
		logger.Error(err, "Error preparing devices for claim", "uid", claim.UID)
		return kubeletplugin.PrepareResult{
			Err: fmt.Errorf("error preparing devices for claim %v: %w", claim.UID, err),
		}
	}

	var prepared []kubeletplugin.Device
	for _, result := range claim.Status.Allocation.Devices.Results {
		if result.Driver != d.driverName {
			continue
		}
		if _, exists := d.devices[result.Device]; !exists {
			err := fmt.Errorf("allocated device %q is not published on this node", result.Device)
			logger.Error(err, "Error preparing devices for claim", "uid", claim.UID)
			return kubeletplugin.PrepareResult{
				Err: fmt.Errorf("error preparing devices for claim %v: %w", claim.UID, err),
			}
		}

		prepared = append(prepared, kubeletplugin.Device{
			Requests:   []string{result.Request},
			PoolName:   result.Pool,
			DeviceName: result.Device,
		})
	}

	logger.Info("Returning newly prepared devices for claim", "uid", claim.UID, "devices", prepared)
	return kubeletplugin.PrepareResult{Devices: prepared}
}

func (d *driver) UnprepareResourceClaims(ctx context.Context, claims []kubeletplugin.NamespacedObject) (map[types.UID]error, error) {
	logger := klog.FromContext(ctx)
	logger.Info("UnprepareResourceClaims is called", "numClaims", len(claims))
	result := make(map[types.UID]error)

	for _, claim := range claims {
		result[claim.UID] = d.unprepareResourceClaim(ctx, claim)
	}

	return result, nil
}

func (d *driver) unprepareResourceClaim(_ context.Context, claim kubeletplugin.NamespacedObject) error {
	return nil
}

func (d *driver) HandleError(ctx context.Context, err error, msg string) {
	utilruntime.HandleErrorWithContext(ctx, err, msg)
	if !errors.Is(err, kubeletplugin.ErrRecoverable) && d.cancelCtx != nil {
		d.cancelCtx(fmt.Errorf("fatal background error: %w", err))
	}
}
