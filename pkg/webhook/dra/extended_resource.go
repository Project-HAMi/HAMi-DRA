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

package dra

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Project-HAMi/HAMi-DRA/pkg/config"
)

// ExtendedResourceWarnings warns when a resource the webhooks strip is also
// claimed by a DeviceClass through extendedResourceName (KEP-5004), because the
// scheduler then no longer sees the extended resource request.
func ExtendedResourceWarnings(ctx context.Context, c client.Reader, cfgs []*config.DRADeviceConfig, containers []corev1.Container) []string {
	used := map[string]bool{}
	for _, container := range containers {
		for _, cfg := range cfgs {
			if _, ok := container.Resources.Limits[corev1.ResourceName(cfg.ResourceCountName)]; ok {
				used[cfg.ResourceCountName] = true
			}
		}
	}
	if len(used) == 0 {
		return nil
	}

	// The cached List waits for the informer to sync, which never happens
	// without RBAC on deviceclasses. Bound it so a warning can't block admission.
	listCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	classes := &resourceapi.DeviceClassList{}
	if err := c.List(listCtx, classes); err != nil {
		klog.Warningf("Failed to list DeviceClasses: %v", err)
		return nil
	}
	var warnings []string
	for _, dc := range classes.Items {
		if name := dc.Spec.ExtendedResourceName; name != nil && used[*name] {
			warnings = append(warnings, fmt.Sprintf("resource %s is moved to a DRA claim by HAMi-DRA, but DeviceClass %s also uses it as extendedResourceName", *name, dc.Name))
		}
	}
	return warnings
}
