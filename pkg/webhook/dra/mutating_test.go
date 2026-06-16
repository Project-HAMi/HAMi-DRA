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

package dra

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/Project-HAMi/HAMi-DRA/pkg/config"
	"github.com/Project-HAMi/HAMi-DRA/pkg/constants"
	"github.com/stretchr/testify/assert"
)

func defaultNvidiaDeviceConfig() *config.DRADeviceConfig {
	cfg, _ := (&config.Config{}).DRADevice(config.VendorNvidia)
	return cfg
}

func TestAddAnnotationSelectors(t *testing.T) {
	tests := []struct {
		name           string
		podAnnotations map[string]string
		wantSelectors  []string
		wantErr        bool
	}{
		{
			name: "single uuid selector",
			podAnnotations: map[string]string{
				constants.UseUUIDAnnotation: "gpu-123",
			},
			wantSelectors: []string{
				`device.attributes["hami-core-gpu.project-hami.io"].uuid in ["gpu-123"]`,
			},
		},

		{
			name: "multiple uuids selector",
			podAnnotations: map[string]string{
				constants.UseUUIDAnnotation: "gpu-123,gpu-456,gpu-789",
			},
			wantSelectors: []string{
				`device.attributes["hami-core-gpu.project-hami.io"].uuid in ["gpu-123","gpu-456","gpu-789"]`,
			},
		},

		{
			name: "exclude uuids selector",
			podAnnotations: map[string]string{
				constants.NoUseUUIDAnnotation: "gpu-999,gpu-888",
			},
			wantSelectors: []string{
				`device.attributes["hami-core-gpu.project-hami.io"].uuid not in ["gpu-999","gpu-888"]`,
			},
		},

		{
			name: "single device type selector",
			podAnnotations: map[string]string{
				constants.UseTypeAnnotation: "A100",
			},
			wantSelectors: []string{
				`device.attributes["hami-core-gpu.project-hami.io"].productName in ["A100"]`,
			},
		},

		{
			name: "multiple device types case insensitive",
			podAnnotations: map[string]string{
				constants.UseTypeAnnotation: "A100,H100",
			},
			wantSelectors: []string{
				`device.attributes["hami-core-gpu.project-hami.io"].productName in ["A100","H100"]`,
			},
		},

		{
			name: "exclude device types",
			podAnnotations: map[string]string{
				constants.NoUseTypeAnnotation: "A100,H100",
			},
			wantSelectors: []string{
				`device.attributes["hami-core-gpu.project-hami.io"].productName not in ["A100","H100"]`,
			},
		},

		{
			name: "combined selectors",
			podAnnotations: map[string]string{
				constants.UseUUIDAnnotation:   "gpu-123,gpu-456",
				constants.NoUseUUIDAnnotation: "gpu-999",
				constants.UseTypeAnnotation:   "A100",
				constants.NoUseTypeAnnotation: "H100",
			},
			wantSelectors: []string{
				`device.attributes["hami-core-gpu.project-hami.io"].uuid in ["gpu-123","gpu-456"]`,
				`device.attributes["hami-core-gpu.project-hami.io"].uuid not in ["gpu-999"]`,
				`device.attributes["hami-core-gpu.project-hami.io"].productName in ["A100"]`,
				`device.attributes["hami-core-gpu.project-hami.io"].productName not in ["H100"]`,
			},
		},
		{
			name:           "no annotations",
			podAnnotations: map[string]string{},
			wantSelectors:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-pod",
					Namespace:   "default",
					Annotations: tt.podAnnotations,
				},
			}

			admission := &MutatingAdmission{DeviceConfig: defaultNvidiaDeviceConfig()}
			claim := &resourceapi.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-1",
					Namespace: "default",
				},
				Spec: resourceapi.ResourceClaimSpec{
					Devices: resourceapi.DeviceClaim{
						Requests: []resourceapi.DeviceRequest{
							{
								Name: "gpu",
								Exactly: &resourceapi.ExactDeviceRequest{
									AllocationMode: resourceapi.DeviceAllocationModeExactCount,
									Capacity: &resourceapi.CapacityRequirements{
										Requests: make(map[resourceapi.QualifiedName]resource.Quantity),
									},
									DeviceClassName: constants.NvidiaDraDriver,
									Selectors:       []resourceapi.DeviceSelector{},
								},
							},
						},
					},
				},
			}

			admission.addAnnotationSelectors(claim, pod)

			selectors := claim.Spec.Devices.Requests[0].Exactly.Selectors
			for i, selector := range selectors {
				t.Logf("Selector %d: %v", i, selector.CEL.Expression)
			}
			assert.Equal(t, len(tt.wantSelectors), len(selectors), "selector nums not match")

			for i, wantExpr := range tt.wantSelectors {
				if i < len(selectors) {
					assert.NotNil(t, selectors[i].CEL, "CEL selector is nil")
					assert.Equal(t, wantExpr, selectors[i].CEL.Expression,
						fmt.Sprintf("selector expression is not match\nexpect: %s\nreal: %s",
							wantExpr, selectors[i].CEL.Expression))
				}
			}
		})
	}
}

func TestAddAnnotationSelectorsHygon(t *testing.T) {
	cfg, err := (&config.Config{}).DRADevice(config.VendorHygon)
	assert.NoError(t, err)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constants.HygonUseUUIDAnnotation: "DCU-123",
				constants.HygonUseTypeAnnotation: "K100",
			},
		},
	}

	admission := &MutatingAdmission{DeviceConfig: cfg}
	claim := &resourceapi.ResourceClaim{
		Spec: resourceapi.ResourceClaimSpec{
			Devices: resourceapi.DeviceClaim{
				Requests: []resourceapi.DeviceRequest{
					{
						Name: "dcu",
						Exactly: &resourceapi.ExactDeviceRequest{
							Selectors: []resourceapi.DeviceSelector{},
						},
					},
				},
			},
		},
	}

	admission.addAnnotationSelectors(claim, pod)
	selectors := claim.Spec.Devices.Requests[0].Exactly.Selectors
	assert.Len(t, selectors, 2)
	assert.Equal(t, `device.attributes["dra.hygon.com"].uuid in ["DCU-123"]`, selectors[0].CEL.Expression)
	assert.Equal(t, `device.attributes["dra.hygon.com"].productName in ["K100"]`, selectors[1].CEL.Expression)
}

func TestBuildResourceClaimUsesConfiguredDriver(t *testing.T) {
	cfg, err := (&config.Config{
		Nvidia: config.NvidiaConfig{
			DeviceClassName: "fake-gpu.project-hami.io",
			DraDriverName:   "fake.dra.hami.io",
		},
	}).DRADevice(config.VendorNvidia)
	assert.NoError(t, err)

	admission := &MutatingAdmission{DeviceConfig: cfg}
	claim := admission.buildResourceClaim("test-claim", "default", nil)
	exactly := claim.Spec.Devices.Requests[0].Exactly

	assert.Equal(t, "fake-gpu.project-hami.io", exactly.DeviceClassName)
	assert.Equal(t, "gpu", claim.Spec.Devices.Requests[0].Name)
	assert.Len(t, exactly.Selectors, 1)
	assert.Equal(t,
		`device.attributes["fake.dra.hami.io"].type == "hami-gpu"`,
		exactly.Selectors[0].CEL.Expression,
	)
}

func TestBuildResourceClaimHygon(t *testing.T) {
	cfg, err := (&config.Config{}).DRADevice(config.VendorHygon)
	assert.NoError(t, err)

	admission := &MutatingAdmission{DeviceConfig: cfg}
	claim := admission.buildResourceClaim("test-claim", "default", nil)
	exactly := claim.Spec.Devices.Requests[0].Exactly

	assert.Equal(t, constants.HygonDraDriver, exactly.DeviceClassName)
	assert.Equal(t, "dcu", claim.Spec.Devices.Requests[0].Name)
	assert.Equal(t,
		`device.driver == "dra.hygon.com" && device.attributes["dra.hygon.com"].type == "dcu"`,
		exactly.Selectors[0].CEL.Expression,
	)
}

func TestBuildResourceClaimWithOwnerReferences(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       types.UID("pod-uid-123"),
		},
	}

	admission := &MutatingAdmission{DeviceConfig: defaultNvidiaDeviceConfig()}
	claim := admission.buildResourceClaim("test-claim", "default", pod)
	// Verify OwnerReference exists
	assert.Equal(t, 1, len(claim.ObjectMeta.OwnerReferences))
	assert.Equal(t, "Pod", claim.ObjectMeta.OwnerReferences[0].Kind)
	assert.Equal(t, "test-pod", claim.ObjectMeta.OwnerReferences[0].Name)
	assert.Equal(t, types.UID("pod-uid-123"), claim.ObjectMeta.OwnerReferences[0].UID)
	assert.True(t, *claim.ObjectMeta.OwnerReferences[0].Controller)
}

func TestBuildResourceClaimSkipsOwnerReferenceWithoutUID(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	admission := &MutatingAdmission{DeviceConfig: defaultNvidiaDeviceConfig()}
	claim := admission.buildResourceClaim("test-claim", "default", pod)
	assert.Empty(t, claim.ObjectMeta.OwnerReferences)
}
