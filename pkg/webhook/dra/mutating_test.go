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
	"context"
	"encoding/json"
	"fmt"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Project-HAMi/HAMi-DRA/pkg/config"
	"github.com/Project-HAMi/HAMi-DRA/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func defaultNvidiaDeviceConfig() *config.DRADeviceConfig {
	cfgs, _ := (&config.Config{}).DRADevices([]string{config.VendorNvidia})
	return cfgs[0]
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
				`!(device.attributes["hami-core-gpu.project-hami.io"].uuid in ["gpu-999","gpu-888"])`,
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
				`!(device.attributes["hami-core-gpu.project-hami.io"].productName in ["A100","H100"])`,
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
				`!(device.attributes["hami-core-gpu.project-hami.io"].uuid in ["gpu-999"])`,
				`device.attributes["hami-core-gpu.project-hami.io"].productName in ["A100"]`,
				`!(device.attributes["hami-core-gpu.project-hami.io"].productName in ["H100"])`,
			},
		},
		{
			name:           "no annotations",
			podAnnotations: map[string]string{},
			wantSelectors:  []string{},
		},
		{
			name: "values are trimmed and empty elements dropped",
			podAnnotations: map[string]string{
				constants.UseUUIDAnnotation: " gpu-123, gpu-456,",
			},
			wantSelectors: []string{
				`device.attributes["hami-core-gpu.project-hami.io"].uuid in ["gpu-123","gpu-456"]`,
			},
		},
		{
			name: "quote and backslash are escaped as a single literal",
			podAnnotations: map[string]string{
				constants.UseTypeAnnotation: `A100"x,B\C`,
			},
			wantSelectors: []string{
				`device.attributes["hami-core-gpu.project-hami.io"].productName in ["A100\"x","B\\C"]`,
			},
		},
		{
			name: "structure injection stays a single literal",
			podAnnotations: map[string]string{
				constants.UseUUIDAnnotation: `"] || true || ["`,
			},
			wantSelectors: []string{
				`device.attributes["hami-core-gpu.project-hami.io"].uuid in ["\"] || true || [\""]`,
			},
		},
		{
			name: "empty annotation fails closed",
			podAnnotations: map[string]string{
				constants.UseUUIDAnnotation: "",
			},
			wantErr: true,
		},
		{
			name: "annotation with only separators fails closed",
			podAnnotations: map[string]string{
				constants.NoUseTypeAnnotation: " , ,",
			},
			wantErr: true,
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

			err := admission.addAnnotationSelectors(claim, pod, admission.DeviceConfig)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

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
	cfgs, err := (&config.Config{}).DRADevices([]string{config.VendorHygon})
	assert.NoError(t, err)
	cfg := cfgs[0]

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constants.HygonUseUUIDAnnotation: "HCU-123",
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
						Name: "hcu",
						Exactly: &resourceapi.ExactDeviceRequest{
							Selectors: []resourceapi.DeviceSelector{},
						},
					},
				},
			},
		},
	}

	require.NoError(t, admission.addAnnotationSelectors(claim, pod, cfg))
	selectors := claim.Spec.Devices.Requests[0].Exactly.Selectors
	assert.Len(t, selectors, 2)
	assert.Equal(t, `device.attributes["dra.hygon.com"].uuid in ["HCU-123"]`, selectors[0].CEL.Expression)
	assert.Equal(t, `device.attributes["dra.hygon.com"].productName in ["K100"]`, selectors[1].CEL.Expression)
}

func TestBuildResourceClaimUsesConfiguredDriver(t *testing.T) {
	deviceConfigs, err := (&config.Config{
		Nvidia: config.NvidiaConfig{
			DeviceClassName: "fake-gpu.project-hami.io",
			DraDriverName:   "fake.dra.hami.io",
		},
	}).DRADevices([]string{config.VendorNvidia})
	require.NoError(t, err)
	deviceConfig := deviceConfigs[0]

	admission := &MutatingAdmission{
		DeviceConfig: deviceConfig,
	}

	claim := admission.buildResourceClaim("test-claim", "default", deviceConfig)
	exactly := claim.Spec.Devices.Requests[0].Exactly

	assert.Equal(t, "fake-gpu.project-hami.io", exactly.DeviceClassName)
	assert.Len(t, exactly.Selectors, 1)
	assert.Equal(t,
		`device.attributes["fake.dra.hami.io"].type == "hami-gpu"`,
		exactly.Selectors[0].CEL.Expression,
	)
}

func TestAddAnnotationSelectorsAscend(t *testing.T) {
	cfgs, err := (&config.Config{}).DRADevices([]string{config.VendorAscend})
	assert.NoError(t, err)
	var cfg *config.DRADeviceConfig
	for _, c := range cfgs {
		if c.CommonWord == "Ascend310P" {
			cfg = c
			break
		}
	}
	require.NotNil(t, cfg)
	assert.Equal(t, "hami.io/use-Ascend310P-uuid", cfg.UseUUIDAnnotation)
	assert.Equal(t, "huawei.com/Ascend310P", cfg.ResourceCountName)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				cfg.UseUUIDAnnotation:             "node-4",
				constants.AscendUseTypeAnnotation: "Ascend310P",
			},
		},
	}

	admission := &MutatingAdmission{DeviceConfig: cfg}
	claim := admission.buildResourceClaim("npu-claim", "default", cfg)
	require.NoError(t, admission.addAnnotationSelectors(claim, pod, cfg))
	selectors := claim.Spec.Devices.Requests[0].Exactly.Selectors
	assert.Len(t, selectors, 3)
	assert.Equal(t,
		`device.driver == "ascend.project-hami.io" && device.attributes["ascend.project-hami.io"].type == "HAMivNPUCore"`,
		selectors[0].CEL.Expression,
	)
	assert.Equal(t, `device.attributes["ascend.project-hami.io"].uuid in ["node-4"]`, selectors[1].CEL.Expression)
	assert.Equal(t, `device.attributes["ascend.project-hami.io"].productName in ["Ascend310P"]`, selectors[2].CEL.Expression)
}

func TestResourceClaimNameDNS1123Label(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "very-long-namespace-name-for-testing",
			Name:      "very-long-pod-name-that-would-exceed-the-label-limit",
		},
	}
	cfg := &config.DRADeviceConfig{CommonWord: "Ascend910B4-1"}
	name := resourceClaimName(pod, "very-long-container-name", cfg)
	assert.LessOrEqual(t, len(name), dns1123LabelMaxLength)
	assert.NotContains(t, name, "--")
	short := resourceClaimName(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "pod"}}, "ctr", nil)
	assert.Equal(t, "ns-pod-ctr", short)

	nvidia := defaultNvidiaDeviceConfig()
	hygonConfigs, err := (&config.Config{}).DRADevices([]string{config.VendorHygon})
	require.NoError(t, err)
	assert.Equal(t, "ns-pod-ctr-nvidia", resourceClaimName(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "pod"}}, "ctr", nvidia))
	assert.Equal(t, "ns-pod-ctr-hygon", resourceClaimName(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "pod"}}, "ctr", hygonConfigs[0]))
}

func TestHandleContainerMultipleVendors(t *testing.T) {
	deviceConfigs, err := (&config.Config{Ascend: config.AscendConfig{
		Devices: []config.AscendVNPUConfig{{
			CommonWord:         "Ascend310P",
			ResourceName:       "huawei.com/Ascend310P",
			ResourceMemoryName: "huawei.com/Ascend310P-memory",
			ResourceCoreName:   "huawei.com/Ascend310P-core",
		}},
	}}).DRADevices([]string{config.VendorNvidia, config.VendorHygon, config.VendorAscend})
	require.NoError(t, err)

	sch := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(sch))
	admission := &MutatingAdmission{
		Client:        fake.NewClientBuilder().WithScheme(sch).Build(),
		DeviceConfigs: deviceConfigs,
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "mixed", Namespace: "default"}}
	container := &corev1.Container{
		Name: "workload",
		Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
			corev1.ResourceName("nvidia.com/gpu"):        resource.MustParse("1"),
			corev1.ResourceName("hygon.com/hcunum"):      resource.MustParse("1"),
			corev1.ResourceName("huawei.com/Ascend310P"): resource.MustParse("1"),
		}},
	}

	names, err := admission.handleContainer(context.Background(), container, pod, nil, false)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		"default-mixed-workload-nvidia",
		"default-mixed-workload-hygon",
		"default-mixed-workload-ascend310p",
	}, names)
	assert.Empty(t, container.Resources.Limits)

	for i, name := range names {
		claim := &resourceapi.ResourceClaim{}
		require.NoError(t, admission.Client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, claim))
		assert.Equal(t, deviceConfigs[i].EffectiveDeviceClassName(), claim.Spec.Devices.Requests[0].Exactly.DeviceClassName)
	}
}

func newGPUPodCreateRequest(t *testing.T, labels map[string]string) admission.Request {
	t.Helper()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "trainer", Namespace: "default", Labels: labels},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: "worker",
			Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
				corev1.ResourceName("nvidia.com/gpu"):    resource.MustParse("1"),
				corev1.ResourceName("nvidia.com/gpumem"): resource.MustParse("1024"),
			}},
		}}},
	}
	raw, err := json.Marshal(pod)
	require.NoError(t, err)
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Namespace: "default",
		Operation: admissionv1.Create,
		Object:    runtime.RawExtension{Raw: raw},
	}}
}

// patchedPodResourceClaims extracts spec.resourceClaims from the JSON patch returned by Handle.
func patchedPodResourceClaims(t *testing.T, resp admission.Response) []corev1.PodResourceClaim {
	t.Helper()
	for _, op := range resp.Patches {
		if op.Path != "/spec/resourceClaims" {
			continue
		}
		raw, err := json.Marshal(op.Value)
		require.NoError(t, err)
		var claims []corev1.PodResourceClaim
		require.NoError(t, json.Unmarshal(raw, &claims))
		return claims
	}
	t.Fatalf("no /spec/resourceClaims patch in response: %+v", resp.Patches)
	return nil
}

func TestHandleResourceClaimTemplate(t *testing.T) {
	tests := []struct {
		name         string
		enabled      bool
		wantTemplate bool
	}{
		{name: "enabled uses template", enabled: true, wantTemplate: true},
		{name: "disabled keeps claim", enabled: false, wantTemplate: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sch := runtime.NewScheme()
			require.NoError(t, scheme.AddToScheme(sch))
			fakeClient := fake.NewClientBuilder().WithScheme(sch).Build()
			a := &MutatingAdmission{
				Decoder:               admission.NewDecoder(sch),
				Client:                fakeClient,
				DeviceConfig:          defaultNvidiaDeviceConfig(),
				ResourceClaimTemplate: tt.enabled,
			}

			resp := a.Handle(context.Background(), newGPUPodCreateRequest(t, nil))
			require.True(t, resp.Allowed, "unexpected rejection: %v", resp.Result)

			const name = "default-trainer-worker-nvidia"
			podClaims := patchedPodResourceClaims(t, resp)
			require.Len(t, podClaims, 1)
			assert.Equal(t, name, podClaims[0].Name)

			key := client.ObjectKey{Namespace: "default", Name: name}
			template := &resourceapi.ResourceClaimTemplate{}
			claim := &resourceapi.ResourceClaim{}
			if tt.wantTemplate {
				require.NotNil(t, podClaims[0].ResourceClaimTemplateName)
				assert.Equal(t, name, *podClaims[0].ResourceClaimTemplateName)
				assert.Nil(t, podClaims[0].ResourceClaimName)

				require.NoError(t, fakeClient.Get(context.Background(), key, template))
				assert.Equal(t, "true", template.Labels[constants.DraLabel])
				request := template.Spec.Spec.Devices.Requests[0].Exactly
				assert.Equal(t, int64(1), request.Count)
				assert.Equal(t, resource.MustParse("1073741824"), request.Capacity.Requests["memory"])
				assert.True(t, apierrors.IsNotFound(fakeClient.Get(context.Background(), key, claim)))
			} else {
				require.NotNil(t, podClaims[0].ResourceClaimName)
				assert.Equal(t, name, *podClaims[0].ResourceClaimName)
				assert.Nil(t, podClaims[0].ResourceClaimTemplateName)

				require.NoError(t, fakeClient.Get(context.Background(), key, claim))
				assert.True(t, apierrors.IsNotFound(fakeClient.Get(context.Background(), key, template)))
			}
		})
	}
}

func TestHandleResourceClaimTemplateRollsBackOnFailure(t *testing.T) {
	deviceConfigs, err := (&config.Config{}).DRADevices([]string{config.VendorNvidia, config.VendorHygon})
	require.NoError(t, err)

	sch := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(sch))
	conflicting := &resourceapi.ResourceClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "default-trainer-worker-hygon", Namespace: "default"},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(sch).WithObjects(conflicting).Build()
	a := &MutatingAdmission{
		Decoder:               admission.NewDecoder(sch),
		Client:                fakeClient,
		DeviceConfigs:         deviceConfigs,
		ResourceClaimTemplate: true,
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "trainer",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: "worker",
			Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
				corev1.ResourceName("nvidia.com/gpu"):   resource.MustParse("1"),
				corev1.ResourceName("hygon.com/hcunum"): resource.MustParse("1"),
			}},
		}}},
	}
	raw, err := json.Marshal(pod)
	require.NoError(t, err)

	resp := a.Handle(context.Background(), admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Namespace: "default",
		Operation: admissionv1.Create,
		Object:    runtime.RawExtension{Raw: raw},
	}})
	assert.False(t, resp.Allowed)

	err = fakeClient.Get(context.Background(),
		client.ObjectKey{Namespace: "default", Name: "default-trainer-worker-nvidia"}, &resourceapi.ResourceClaimTemplate{})
	assert.True(t, apierrors.IsNotFound(err), "template created before the failure should be rolled back, got: %v", err)
	require.NoError(t, fakeClient.Get(context.Background(),
		client.ObjectKey{Namespace: "default", Name: "default-trainer-worker-hygon"}, &resourceapi.ResourceClaimTemplate{}),
		"pre-existing template must not be deleted by the rollback")
}
