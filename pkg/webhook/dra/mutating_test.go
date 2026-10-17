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
	draCEL "k8s.io/dynamic-resource-allocation/cel"
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
				`!has(device.attributes["hami-core-gpu.project-hami.io"].uuid) || !(device.attributes["hami-core-gpu.project-hami.io"].uuid in ["gpu-999","gpu-888"])`,
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
				`!has(device.attributes["hami-core-gpu.project-hami.io"].productName) || !(device.attributes["hami-core-gpu.project-hami.io"].productName in ["A100","H100"])`,
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
				`!has(device.attributes["hami-core-gpu.project-hami.io"].uuid) || !(device.attributes["hami-core-gpu.project-hami.io"].uuid in ["gpu-999"])`,
				`device.attributes["hami-core-gpu.project-hami.io"].productName in ["A100"]`,
				`!has(device.attributes["hami-core-gpu.project-hami.io"].productName) || !(device.attributes["hami-core-gpu.project-hami.io"].productName in ["H100"])`,
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

func TestHandleDryRunSkipsPersistentWrites(t *testing.T) {
	tests := []struct {
		name     string
		template bool
	}{
		{name: "template", template: true},
		{name: "claim", template: false},
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
				ResourceClaimTemplate: tt.template,
			}

			req := newGPUPodCreateRequest(t, nil)
			dryRun := true
			req.DryRun = &dryRun
			resp := a.Handle(context.Background(), req)
			require.True(t, resp.Allowed, "unexpected rejection: %v", resp.Result)
			assert.NotEmpty(t, resp.Patches, "dry run should still show the mutated pod")

			const name = "default-trainer-worker-nvidia"
			podClaims := patchedPodResourceClaims(t, resp)
			require.Len(t, podClaims, 1)
			assert.Equal(t, name, podClaims[0].Name)
			if tt.template {
				require.NotNil(t, podClaims[0].ResourceClaimTemplateName)
				assert.Equal(t, name, *podClaims[0].ResourceClaimTemplateName)
			} else {
				require.NotNil(t, podClaims[0].ResourceClaimName)
				assert.Equal(t, name, *podClaims[0].ResourceClaimName)
			}

			key := client.ObjectKey{Namespace: "default", Name: name}
			assert.True(t, apierrors.IsNotFound(fakeClient.Get(context.Background(), key, &resourceapi.ResourceClaimTemplate{})))
			assert.True(t, apierrors.IsNotFound(fakeClient.Get(context.Background(), key, &resourceapi.ResourceClaim{})))
		})
	}
}

func TestHandleDryRunDoesNotDeleteExistingClaims(t *testing.T) {
	deviceConfigs, err := (&config.Config{}).DRADevices([]string{config.VendorNvidia, config.VendorHygon})
	require.NoError(t, err)

	sch := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(sch))
	const nvidiaName = "default-trainer-worker-nvidia"
	existing := &resourceapi.ResourceClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: nvidiaName, Namespace: "default", Labels: map[string]string{constants.DraLabel: "true"}},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(sch).WithObjects(existing).Build()
	a := &MutatingAdmission{
		Decoder:               admission.NewDecoder(sch),
		Client:                fakeClient,
		DeviceConfigs:         deviceConfigs,
		ResourceClaimTemplate: true,
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "trainer", Namespace: "default"},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: "worker",
			Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
				corev1.ResourceName("nvidia.com/gpu"):     resource.MustParse("1"),
				corev1.ResourceName("hygon.com/hcunum"):   resource.MustParse("1"),
				corev1.ResourceName("hygon.com/hcucores"): resource.MustParse("50"),
			}},
		}}},
	}
	raw, err := json.Marshal(pod)
	require.NoError(t, err)
	dryRun := true
	resp := a.Handle(context.Background(), admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Namespace: "default",
		Operation: admissionv1.Create,
		DryRun:    &dryRun,
		Object:    runtime.RawExtension{Raw: raw},
	}})
	assert.False(t, resp.Allowed, "invalid hygon core conversion should still reject the request")

	require.NoError(t, fakeClient.Get(context.Background(),
		client.ObjectKey{Namespace: "default", Name: nvidiaName}, &resourceapi.ResourceClaimTemplate{}),
		"dry-run must not delete an existing template")
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

func TestHandleContainerAMD(t *testing.T) {
	deviceConfigs, err := (&config.Config{Amd: config.AmdConfig{ReferenceComputeUnits: 304}}).DRADevices([]string{config.VendorAMD})
	require.NoError(t, err)
	sch := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(sch))
	admission := &MutatingAdmission{
		Client:        fake.NewClientBuilder().WithScheme(sch).Build(),
		DeviceConfigs: deviceConfigs,
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: "amd", Namespace: "default",
		Annotations: map[string]string{
			constants.AmdUseTypeAnnotation:   "AMD Instinct MI300X",
			constants.AmdNoUseUUIDAnnotation: "GPU-bad",
		},
	}}
	container := &corev1.Container{
		Name: "rocm",
		Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
			corev1.ResourceName("amd.com/gpu"):      resource.MustParse("2"),
			corev1.ResourceName("amd.com/gpumem"):   resource.MustParse("4096"),
			corev1.ResourceName("amd.com/gpucores"): resource.MustParse("25"),
		}},
	}

	names, err := admission.handleContainer(context.Background(), container, pod, nil, false)
	require.NoError(t, err)
	require.Equal(t, []string{"default-amd-rocm-amd"}, names)
	assert.Empty(t, container.Resources.Limits)

	claim := &resourceapi.ResourceClaim{}
	require.NoError(t, admission.Client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: names[0]}, claim))
	exactly := claim.Spec.Devices.Requests[0].Exactly
	assert.Equal(t, "gpu.amd.com", exactly.DeviceClassName)
	assert.Equal(t, int64(2), exactly.Count)
	mem, cus := exactly.Capacity.Requests["memory"], exactly.Capacity.Requests["computeUnits"]
	assert.Equal(t, int64(4096)*1024*1024, mem.Value())
	assert.Equal(t, int64(76), cus.Value())
	_, hasCores := exactly.Capacity.Requests["cores"]
	assert.False(t, hasCores, "the AMD driver has no cores capacity")
	require.Len(t, exactly.Selectors, 3)
	assert.Equal(t, `device.driver == "gpu.amd.com" && device.attributes["gpu.amd.com"].type == "amdgpu"`, exactly.Selectors[0].CEL.Expression)
	assert.Equal(t, `!has(device.attributes["gpu.amd.com"].uuid) || !(device.attributes["gpu.amd.com"].uuid in ["GPU-bad"])`, exactly.Selectors[1].CEL.Expression)
	assert.Equal(t, `has(device.attributes["gpu.amd.com"].productName) && ["amd_instinct_mi300x"].exists(t, device.attributes["gpu.amd.com"].productName.lowerAscii().contains(t))`, exactly.Selectors[2].CEL.Expression)
}

func TestHandleContainerAMDNeedsReferenceComputeUnitsForCores(t *testing.T) {
	deviceConfigs, err := (&config.Config{}).DRADevices([]string{config.VendorAMD})
	require.NoError(t, err)
	sch := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(sch))
	admission := &MutatingAdmission{Client: fake.NewClientBuilder().WithScheme(sch).Build(), DeviceConfigs: deviceConfigs}
	container := &corev1.Container{Name: "rocm", Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
		corev1.ResourceName("amd.com/gpu"):      resource.MustParse("1"),
		corev1.ResourceName("amd.com/gpucores"): resource.MustParse("25"),
	}}}
	_, err = admission.handleContainer(context.Background(), container, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default"}}, nil, false)
	assert.ErrorContains(t, err, "referenceComputeUnits")
	claims := &resourceapi.ResourceClaimList{}
	require.NoError(t, admission.Client.List(context.Background(), claims))
	assert.Empty(t, claims.Items, "a failed conversion must not leave a claim behind")
}

func TestHandleUpdateStripsReappliedResources(t *testing.T) {
	a := &MutatingAdmission{DeviceConfig: defaultNvidiaDeviceConfig()}
	gpu := corev1.ResourceList{"nvidia.com/gpu": resource.MustParse("1"), "nvidia.com/gpumem": resource.MustParse("1000")}
	pod := &corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{
		{
			Name: "c",
			Resources: corev1.ResourceRequirements{
				Limits: gpu.DeepCopy(), Requests: gpu.DeepCopy(),
				Claims: []corev1.ResourceClaim{{Name: "claim"}},
			},
		},
		{
			Name:      "no-claims",
			Resources: corev1.ResourceRequirements{Limits: gpu.DeepCopy(), Requests: gpu.DeepCopy()},
		},
	}}}

	raw, err := json.Marshal(pod)
	require.NoError(t, err)
	req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: raw}}}
	resp := a.handleUpdate(req, pod)
	require.True(t, resp.Allowed)
	assert.NotEmpty(t, resp.Patches)
	assert.Empty(t, pod.Spec.Containers[0].Resources.Limits)
	assert.Empty(t, pod.Spec.Containers[0].Resources.Requests)
	assert.Equal(t, gpu, pod.Spec.Containers[1].Resources.Limits, "container without claims is left alone")
	assert.Equal(t, gpu, pod.Spec.Containers[1].Resources.Requests)

	raw, err = json.Marshal(pod)
	require.NoError(t, err)
	req.Object.Raw = raw
	resp = a.handleUpdate(req, pod)
	require.True(t, resp.Allowed)
	assert.Empty(t, resp.Patches, "nothing to strip on a plain update")
}

// The AMD driver publishes neither uuid nor, on some cards, productName. An
// exclusion must still match such a device instead of failing on the lookup.
func TestExclusionSelectorsMatchDevicesWithoutTheAttribute(t *testing.T) {
	cfgs, err := (&config.Config{}).DRADevices([]string{config.VendorAMD})
	require.NoError(t, err)
	claim := &resourceapi.ResourceClaim{Spec: resourceapi.ResourceClaimSpec{Devices: resourceapi.DeviceClaim{
		Requests: []resourceapi.DeviceRequest{cfgs[0].NewPrimaryDeviceRequest()},
	}}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
		constants.AmdNoUseUUIDAnnotation: "GPU-bad",
		constants.AmdNoUseTypeAnnotation: "MI300X",
	}}}
	require.NoError(t, (&MutatingAdmission{}).addAnnotationSelectors(claim, pod, cfgs[0]))

	str := func(v string) resourceapi.DeviceAttribute { return resourceapi.DeviceAttribute{StringValue: &v} }
	matches := func(attrs map[resourceapi.QualifiedName]resourceapi.DeviceAttribute) bool {
		for _, sel := range claim.Spec.Devices.Requests[0].Exactly.Selectors {
			result := draCEL.GetCompiler(draCEL.Features{}).CompileCELExpression(sel.CEL.Expression, draCEL.Options{})
			require.Nil(t, result.Error, sel.CEL.Expression)
			ok, _, err := result.DeviceMatches(context.Background(), draCEL.Device{Driver: "gpu.amd.com", Attributes: attrs})
			require.NoError(t, err, sel.CEL.Expression)
			if !ok {
				return false
			}
		}
		return true
	}

	assert.True(t, matches(map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{"type": str("amdgpu")}))
	assert.True(t, matches(map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{"type": str("amdgpu"), "uuid": str("GPU-ok"), "productName": str("RX9060XT")}))
	assert.False(t, matches(map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{"type": str("amdgpu"), "uuid": str("GPU-bad")}))
	assert.False(t, matches(map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{"type": str("amdgpu"), "productName": str("MI300X")}))
}

func amdStr(v string) resourceapi.DeviceAttribute {
	return resourceapi.DeviceAttribute{StringValue: &v}
}

// selectorsMatch evaluates selectors with the scheduler's CEL environment.
func selectorsMatch(t *testing.T, selectors []resourceapi.DeviceSelector, dev draCEL.Device) bool {
	t.Helper()
	for _, sel := range selectors {
		result := draCEL.GetCompiler(draCEL.Features{}).CompileCELExpression(sel.CEL.Expression, draCEL.Options{})
		require.Nil(t, result.Error, sel.CEL.Expression)
		ok, _, err := result.DeviceMatches(context.Background(), dev)
		require.NoError(t, err, sel.CEL.Expression)
		if !ok {
			return false
		}
	}
	return true
}

// HAMi matches AMD types as case-insensitive parts of the name; the driver
// publishes the name with "_" for spaces.
func TestAMDTypeAnnotationsMatchPartsOfTheName(t *testing.T) {
	cfgs, err := (&config.Config{}).DRADevices([]string{config.VendorAMD})
	require.NoError(t, err)
	matches := func(annotations map[string]string, productName string) bool {
		claim := &resourceapi.ResourceClaim{Spec: resourceapi.ResourceClaimSpec{Devices: resourceapi.DeviceClaim{
			Requests: []resourceapi.DeviceRequest{cfgs[0].NewPrimaryDeviceRequest()},
		}}}
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: annotations}}
		require.NoError(t, (&MutatingAdmission{}).addAnnotationSelectors(claim, pod, cfgs[0]))
		attrs := map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{"type": amdStr("amdgpu")}
		if productName != "" {
			attrs["productName"] = amdStr(productName)
		}
		return selectorsMatch(t, claim.Spec.Devices.Requests[0].Exactly.Selectors, draCEL.Device{Driver: "gpu.amd.com", Attributes: attrs})
	}
	use := func(v string) map[string]string { return map[string]string{constants.AmdUseTypeAnnotation: v} }
	nouse := func(v string) map[string]string { return map[string]string{constants.AmdNoUseTypeAnnotation: v} }

	assert.True(t, matches(use("RX 9060"), "AMD_Radeon_RX_9060_XT"))
	assert.True(t, matches(use("AMD Radeon RX 9060 XT"), "AMD_Radeon_RX_9060_XT"))
	assert.True(t, matches(use("mi300, rx 9060"), "AMD_Radeon_RX_9060_XT"))
	assert.True(t, matches(use("MI300"), "AMD_Instinct_MI300X"))
	assert.False(t, matches(use("MI300"), "AMD_Radeon_RX_9060_XT"))
	assert.False(t, matches(use("RX 9060"), ""), "a device without a name is not chosen")
	assert.False(t, matches(nouse("rx 9060"), "AMD_Radeon_RX_9060_XT"))
	assert.True(t, matches(nouse("MI300"), "AMD_Radeon_RX_9060_XT"))
	assert.True(t, matches(nouse("MI300"), ""))
}

func TestHandleContainerAMDMemoryPercentage(t *testing.T) {
	cfgs, err := (&config.Config{Amd: config.AmdConfig{ReferenceComputeUnits: 32}}).DRADevices([]string{config.VendorAMD})
	require.NoError(t, err)
	sch := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(sch))
	device := func(name, typ string, mem int64) resourceapi.Device {
		return resourceapi.Device{
			Name:       name,
			Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{"type": amdStr(typ)},
			Capacity:   map[resourceapi.QualifiedName]resourceapi.DeviceCapacity{"memory": {Value: *resource.NewQuantity(mem, resource.BinarySI)}},
		}
	}
	slices := []client.Object{
		&resourceapi.ResourceSlice{ObjectMeta: metav1.ObjectMeta{Name: "a"}, Spec: resourceapi.ResourceSliceSpec{
			Driver: "gpu.amd.com", Pool: resourceapi.ResourcePool{Name: "a"},
			Devices: []resourceapi.Device{device("gpu-1", "amdgpu", 16304<<20), device("gpu-2", "amdgpu", 16304<<20), device("part-1", "amdgpu-partition", 1000<<20)},
		}},
		&resourceapi.ResourceSlice{ObjectMeta: metav1.ObjectMeta{Name: "b"}, Spec: resourceapi.ResourceSliceSpec{
			Driver: "gpu.amd.com", Pool: resourceapi.ResourcePool{Name: "b"},
			Devices: []resourceapi.Device{device("gpu-1", "amdgpu", 196592<<20)},
		}},
		&resourceapi.ResourceSlice{ObjectMeta: metav1.ObjectMeta{Name: "c"}, Spec: resourceapi.ResourceSliceSpec{
			Driver: "gpu.nvidia.com", Pool: resourceapi.ResourcePool{Name: "c"},
			Devices: []resourceapi.Device{device("gpu-0", "amdgpu", 24576<<20)},
		}},
	}
	newAdmission := func(objs ...client.Object) *MutatingAdmission {
		return &MutatingAdmission{Client: fake.NewClientBuilder().WithScheme(sch).WithObjects(objs...).Build(), DeviceConfigs: cfgs}
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pct", Namespace: "default"}}
	container := func(limits corev1.ResourceList) *corev1.Container {
		return &corev1.Container{Name: "c", Resources: corev1.ResourceRequirements{Limits: limits}}
	}

	a := newAdmission(slices...)
	c := container(corev1.ResourceList{
		"amd.com/gpu":               resource.MustParse("1"),
		"amd.com/gpumem-percentage": resource.MustParse("25"),
		"amd.com/gpucores":          resource.MustParse("50"),
	})
	names, err := a.handleContainer(context.Background(), c, pod, nil, false)
	require.NoError(t, err)
	assert.Empty(t, c.Resources.Limits)
	claim := &resourceapi.ResourceClaim{}
	require.NoError(t, a.Client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: names[0]}, claim))
	req := claim.Spec.Devices.Requests[0]
	assert.Nil(t, req.Exactly)
	require.Len(t, req.FirstAvailable, 2, "one subrequest per memory size of an amdgpu device")
	for i, want := range []int64{4076, 49148} {
		sub := req.FirstAvailable[i]
		mem, cus := sub.Capacity.Requests["memory"], sub.Capacity.Requests["computeUnits"]
		assert.Equal(t, want<<20, mem.Value())
		assert.Equal(t, int64(16), cus.Value())
		assert.Equal(t, int64(1), sub.Count)
		assert.Equal(t, "gpu.amd.com", sub.DeviceClassName)
	}
	dev := func(mem int64) draCEL.Device {
		return draCEL.Device{
			Driver:     "gpu.amd.com",
			Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{"type": amdStr("amdgpu")},
			Capacity:   map[resourceapi.QualifiedName]resourceapi.DeviceCapacity{"memory": {Value: *resource.NewQuantity(mem, resource.BinarySI)}},
		}
	}
	assert.True(t, selectorsMatch(t, req.FirstAvailable[0].Selectors, dev(16304<<20)))
	assert.False(t, selectorsMatch(t, req.FirstAvailable[0].Selectors, dev(196592<<20)))
	assert.True(t, selectorsMatch(t, req.FirstAvailable[1].Selectors, dev(196592<<20)))

	// An absolute request wins, as in HAMi.
	c = container(corev1.ResourceList{
		"amd.com/gpu":               resource.MustParse("1"),
		"amd.com/gpumem":            resource.MustParse("2000"),
		"amd.com/gpumem-percentage": resource.MustParse("25"),
	})
	names, err = newAdmission(slices...).handleContainer(context.Background(), c, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "abs", Namespace: "default"}}, nil, false)
	require.NoError(t, err)
	assert.Empty(t, c.Resources.Limits)
	assert.Len(t, names, 1)

	for _, tc := range []struct {
		pct  string
		objs []client.Object
		err  string
	}{
		{"0", slices, "must be between 1 and 100"},
		{"101", slices, "must be between 1 and 100"},
		{"50", nil, "no gpu.amd.com devices are published"},
	} {
		c = container(corev1.ResourceList{"amd.com/gpu": resource.MustParse("1"), "amd.com/gpumem-percentage": resource.MustParse(tc.pct)})
		_, err = newAdmission(tc.objs...).handleContainer(context.Background(), c, pod, nil, false)
		assert.ErrorContains(t, err, tc.err, tc.pct)
	}
}
