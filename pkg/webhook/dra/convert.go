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

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Project-HAMi/HAMi-DRA/pkg/config"
)

// The functions below expose the conversion steps of the Pod path, so the
// Volcano path builds the same claim for the same request.

// CoreCapacity converts a core request like the Pod path does, deriving the
// reference compute units from the published devices when they are unset.
func (a *MutatingAdmission) CoreCapacity(ctx context.Context, cfg *config.DRADeviceConfig, cores resource.Quantity) (resource.Quantity, error) {
	coreCfg, err := a.withReferenceComputeUnits(ctx, cfg)
	if err != nil {
		return resource.Quantity{}, err
	}
	return coreCfg.ConvertCores(cores)
}

// CompleteClaimSpec adds the selectors of the device annotations and, when
// memoryPercentage is set, sizes the request by the published device memory.
func (a *MutatingAdmission) CompleteClaimSpec(ctx context.Context, spec *resourceapi.ResourceClaimSpec, annotations map[string]string, cfg *config.DRADeviceConfig, memoryPercentage *resource.Quantity) error {
	claim := &resourceapi.ResourceClaim{Spec: *spec}
	if err := a.addAnnotationSelectors(claim, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: annotations}}, cfg); err != nil {
		return err
	}
	if memoryPercentage != nil {
		if err := a.requestMemoryPercentage(ctx, claim, cfg, *memoryPercentage); err != nil {
			return err
		}
	}
	*spec = claim.Spec
	return nil
}
