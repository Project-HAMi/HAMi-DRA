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
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Project-HAMi/HAMi-DRA/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDeleteRequest(t *testing.T, pod *corev1.Pod) admission.Request {
	t.Helper()
	raw, err := json.Marshal(pod)
	require.NoError(t, err)
	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Namespace: pod.Namespace,
			Operation: admissionv1.Delete,
			OldObject: runtime.RawExtension{Raw: raw},
		},
	}
}

func podResourceClaim(name, rcName string) corev1.PodResourceClaim {
	return corev1.PodResourceClaim{Name: name, ResourceClaimName: &rcName}
}

func TestValidatingHandle_SkipsPodsWithoutDraLabel(t *testing.T) {
	sch := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(sch))

	existing := &resourceapi.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "default-untouched-gpu", Namespace: "default"},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(sch).WithObjects(existing).Build()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "untouched", Namespace: "default"},
		Spec: corev1.PodSpec{
			ResourceClaims: []corev1.PodResourceClaim{podResourceClaim("gpu", "default-untouched-gpu")},
		},
	}

	v := &ValidatingAdmission{Client: fakeClient}
	resp := v.Handle(context.Background(), newDeleteRequest(t, pod))

	assert.True(t, resp.Allowed, "pods without the DRA label should always be allowed")

	err := fakeClient.Get(context.Background(),
		client.ObjectKey{Namespace: "default", Name: "default-untouched-gpu"},
		&resourceapi.ResourceClaim{})
	assert.NoError(t, err, "ResourceClaim should be left alone when the pod has no DRA label")
}

func TestValidatingHandle_DeletesResourceClaims(t *testing.T) {
	sch := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(sch))

	claimOne := &resourceapi.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "default-multi-gpu-one", Namespace: "default"},
	}
	claimTwo := &resourceapi.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "default-multi-gpu-two", Namespace: "default"},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(sch).WithObjects(claimOne, claimTwo).Build()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multi",
			Namespace: "default",
			Labels:    map[string]string{constants.DraLabel: "true"},
		},
		Spec: corev1.PodSpec{
			ResourceClaims: []corev1.PodResourceClaim{
				podResourceClaim("gpu-one", "default-multi-gpu-one"),
				podResourceClaim("gpu-two", "default-multi-gpu-two"),
			},
		},
	}

	v := &ValidatingAdmission{Client: fakeClient}
	resp := v.Handle(context.Background(), newDeleteRequest(t, pod))

	require.True(t, resp.Allowed, "pod deletion should always be allowed")

	for _, name := range []string{"default-multi-gpu-one", "default-multi-gpu-two"} {
		err := fakeClient.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &resourceapi.ResourceClaim{})
		assert.True(t, apierrors.IsNotFound(err), "ResourceClaim %s should have been deleted, got: %v", name, err)
	}
}

func TestValidatingHandle_MissingResourceClaimIsIgnored(t *testing.T) {
	sch := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(sch))
	fakeClient := fake.NewClientBuilder().WithScheme(sch).Build()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "already-gone",
			Namespace: "default",
			Labels:    map[string]string{constants.DraLabel: "true"},
		},
		Spec: corev1.PodSpec{
			ResourceClaims: []corev1.PodResourceClaim{podResourceClaim("gpu", "default-already-gone-gpu")},
		},
	}

	v := &ValidatingAdmission{Client: fakeClient}
	resp := v.Handle(context.Background(), newDeleteRequest(t, pod))

	assert.True(t, resp.Allowed, "a ResourceClaim that is already gone should not block pod deletion")
}

func TestValidatingHandle_MalformedOldObject(t *testing.T) {
	v := &ValidatingAdmission{}
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Namespace: "default",
			Operation: admissionv1.Delete,
			OldObject: runtime.RawExtension{Raw: []byte("not-json")},
		},
	}

	resp := v.Handle(context.Background(), req)

	assert.False(t, resp.Allowed)
	assert.Equal(t, int32(400), resp.Result.Code)
}

func TestGetResourceClaimName(t *testing.T) {
	rcName := "default-pod-gpu"
	templateOnly := "default-pod-template"
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			ResourceClaims: []corev1.PodResourceClaim{
				{Name: "gpu", ResourceClaimName: &rcName},
				{Name: "templated", ResourceClaimTemplateName: &templateOnly},
				{Name: "empty"},
			},
		},
	}

	names := getResourceClaimName(pod)

	assert.Equal(t, []string{"default-pod-gpu"}, names, "only entries with a resolved ResourceClaimName should be returned")
}
