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

func TestValidatingHandleDryRunKeepsTemplates(t *testing.T) {
	sch := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(sch))
	existing := &resourceapi.ResourceClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "default-worker-main", Namespace: "default"},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(sch).WithObjects(existing).Build()
	job := &vcv1alpha1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "j", Namespace: "default", Labels: map[string]string{constants.DraLabel: "true"}},
		Spec: vcv1alpha1.JobSpec{Tasks: []vcv1alpha1.TaskSpec{{
			Name: "worker",
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
				ResourceClaims: []corev1.PodResourceClaim{{Name: "default-worker-main"}},
			}},
		}}},
	}
	raw, err := json.Marshal(job)
	require.NoError(t, err)
	dryRun := true
	req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Delete,
		Namespace: "default",
		OldObject: runtime.RawExtension{Raw: raw},
		DryRun:    &dryRun,
	}}

	resp := (&ValidatingAdmission{Client: fakeClient}).Handle(context.Background(), req)
	assert.True(t, resp.Allowed)
	assert.NoError(t, fakeClient.Get(context.Background(),
		client.ObjectKey{Namespace: "default", Name: "default-worker-main"}, &resourceapi.ResourceClaimTemplate{}))
}
