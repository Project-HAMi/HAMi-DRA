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
	"encoding/json"
	"strings"
	"testing"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Project-HAMi/HAMi-DRA/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleWarnsOnExtendedResourceConflict(t *testing.T) {
	sch := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(sch))
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default"},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: "c",
			Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
				"nvidia.com/gpu": resource.MustParse("1"),
			}},
		}}},
	}
	raw, err := json.Marshal(pod)
	require.NoError(t, err)
	req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Create,
		Namespace: "default",
		Object:    runtime.RawExtension{Raw: raw},
	}}

	for _, tc := range []struct {
		name         string
		extendedName string
		wantWarnings int
	}{
		{"conflict", "nvidia.com/gpu", 1},
		{"other resource", "example.com/gpu", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dc := &resourceapi.DeviceClass{
				ObjectMeta: metav1.ObjectMeta{Name: "gpu"},
				Spec:       resourceapi.DeviceClassSpec{ExtendedResourceName: &tc.extendedName},
			}
			a := &MutatingAdmission{
				Decoder:      admission.NewDecoder(sch),
				Client:       fake.NewClientBuilder().WithScheme(sch).WithObjects(dc).Build(),
				DeviceConfig: defaultNvidiaDeviceConfig(),
			}
			resp := a.Handle(context.Background(), req)
			require.True(t, resp.Allowed)
			assert.NotEmpty(t, resp.Patches)
			assert.Len(t, resp.Warnings, tc.wantWarnings)
		})
	}
}

func TestExtendedResourceWarningsCoversCoreAndMemory(t *testing.T) {
	sch := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(sch))
	cfg := defaultNvidiaDeviceConfig()
	limits := func(names ...string) []corev1.Container {
		l := corev1.ResourceList{}
		for _, n := range names {
			l[corev1.ResourceName(n)] = resource.MustParse("1")
		}
		return []corev1.Container{{Resources: corev1.ResourceRequirements{Limits: l}}}
	}
	for _, tc := range []struct {
		name       string
		containers []corev1.Container
		want       int
	}{
		{"memory stripped with count", limits(cfg.ResourceCountName, cfg.ResourceMemoryName), 1},
		{"core stripped with count", limits(cfg.ResourceCountName, cfg.ResourceCoreName), 1},
		{"memory kept without count", limits(cfg.ResourceMemoryName), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			extended := cfg.ResourceMemoryName
			if strings.Contains(tc.name, "core") {
				extended = cfg.ResourceCoreName
			}
			dc := &resourceapi.DeviceClass{
				ObjectMeta: metav1.ObjectMeta{Name: "gpu"},
				Spec:       resourceapi.DeviceClassSpec{ExtendedResourceName: &extended},
			}
			c := fake.NewClientBuilder().WithScheme(sch).WithObjects(dc).Build()
			got := ExtendedResourceWarnings(context.Background(), c, []*config.DRADeviceConfig{cfg}, tc.containers)
			assert.Len(t, got, tc.want)
		})
	}
}

func TestHandleDoesNotBlockWhenDeviceClassListHangs(t *testing.T) {
	sch := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(sch))
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default"},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: "c",
			Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
				"nvidia.com/gpu": resource.MustParse("1"),
			}},
		}}},
	}
	raw, err := json.Marshal(pod)
	require.NoError(t, err)
	req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Create,
		Namespace: "default",
		Object:    runtime.RawExtension{Raw: raw},
	}}
	// Like an informer that never syncs: List returns only when ctx is done.
	hang := interceptor.Funcs{List: func(ctx context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
		<-ctx.Done()
		return ctx.Err()
	}}
	a := &MutatingAdmission{
		Decoder:      admission.NewDecoder(sch),
		Client:       fake.NewClientBuilder().WithScheme(sch).WithInterceptorFuncs(hang).Build(),
		DeviceConfig: defaultNvidiaDeviceConfig(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	resp := a.Handle(ctx, req)
	assert.Less(t, time.Since(start), 4*time.Second)
	require.True(t, resp.Allowed)
	assert.NotEmpty(t, resp.Patches)
	assert.Empty(t, resp.Warnings)
}
