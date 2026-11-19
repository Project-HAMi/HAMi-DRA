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

package volcano

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
	vcv1alpha1 "volcano.sh/apis/pkg/apis/batch/v1alpha1"

	"github.com/Project-HAMi/HAMi-DRA/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string { return &s }

func ownedTemplate(name string) *resourceapi.ResourceClaimTemplate {
	return &resourceapi.ResourceClaimTemplate{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: "default", Labels: map[string]string{constants.DraLabel: "true"},
	}}
}

func jobDeleteRequest(t *testing.T, claims []corev1.PodResourceClaim, dryRun bool) admission.Request {
	t.Helper()
	job := &vcv1alpha1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "j", Namespace: "default", Labels: map[string]string{constants.DraLabel: "true"}},
		Spec: vcv1alpha1.JobSpec{Tasks: []vcv1alpha1.TaskSpec{{
			Name:     "worker",
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{ResourceClaims: claims}},
		}}},
	}
	raw, err := json.Marshal(job)
	require.NoError(t, err)
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Delete,
		Namespace: "default",
		OldObject: runtime.RawExtension{Raw: raw},
		DryRun:    &dryRun,
	}}
}

func newValidating(t *testing.T, objs ...client.Object) (*ValidatingAdmission, client.Client) {
	t.Helper()
	sch := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(sch))
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(objs...).Build()
	return &ValidatingAdmission{Client: c, Reader: c}, c
}

func templateExists(t *testing.T, c client.Client, name string) bool {
	t.Helper()
	err := c.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &resourceapi.ResourceClaimTemplate{})
	if apierrors.IsNotFound(err) {
		return false
	}
	require.NoError(t, err)
	return true
}

func TestValidatingHandleDryRunKeepsTemplates(t *testing.T) {
	v, c := newValidating(t, ownedTemplate("default-j-main"))
	req := jobDeleteRequest(t, []corev1.PodResourceClaim{
		{Name: "default-j-main", ResourceClaimTemplateName: strPtr("default-j-main")},
	}, true)

	assert.True(t, v.Handle(context.Background(), req).Allowed)
	assert.True(t, templateExists(t, c, "default-j-main"))
}

func TestValidatingHandleDeletesOnlyOwnTemplates(t *testing.T) {
	// A user template whose name happens to start with "<namespace>-<job>-".
	userPrefixed := &resourceapi.ResourceClaimTemplate{ObjectMeta: metav1.ObjectMeta{Name: "default-j-user", Namespace: "default"}}
	shared := &resourceapi.ResourceClaimTemplate{ObjectMeta: metav1.ObjectMeta{Name: "team-shared", Namespace: "default"}}
	v, c := newValidating(t, ownedTemplate("default-j-main"), userPrefixed, shared, ownedTemplate("default-j-claim"))
	req := jobDeleteRequest(t, []corev1.PodResourceClaim{
		{Name: "default-j-main", ResourceClaimTemplateName: strPtr("default-j-main")},
		{Name: "default-j-user", ResourceClaimTemplateName: strPtr("default-j-user")},
		{Name: "team-shared", ResourceClaimTemplateName: strPtr("team-shared")},
		// The entry refers to a ResourceClaim, so the template sharing its name is not ours to delete.
		{Name: "default-j-claim", ResourceClaimName: strPtr("some-claim")},
	}, false)

	require.True(t, v.Handle(context.Background(), req).Allowed)

	assert.False(t, templateExists(t, c, "default-j-main"), "webhook-generated template should be deleted")
	assert.True(t, templateExists(t, c, "default-j-user"), "unlabeled template must survive job deletion")
	assert.True(t, templateExists(t, c, "team-shared"), "user-managed template must survive job deletion")
	assert.True(t, templateExists(t, c, "default-j-claim"), "template referenced only by entry name must survive")
}
