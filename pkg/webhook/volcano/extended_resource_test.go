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
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	vcv1alpha1 "volcano.sh/apis/pkg/apis/batch/v1alpha1"

	"github.com/Project-HAMi/HAMi-DRA/pkg/config"
	"github.com/Project-HAMi/HAMi-DRA/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleWarnsOnExtendedResourceConflict(t *testing.T) {
	sch := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(sch))
	require.NoError(t, vcv1alpha1.AddToScheme(sch))
	jobRaw, err := json.Marshal(quickstartJob)
	require.NoError(t, err)
	req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Create,
		Namespace: quickstartJob.Namespace,
		Object:    runtime.RawExtension{Raw: jobRaw},
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
				Decoder: admission.NewDecoder(sch),
				Client:  fake.NewClientBuilder().WithScheme(sch).WithObjects(dc).Build(),
				DeviceConfig: &config.DRADeviceConfig{
					ResourceCountName:  "nvidia.com/gpu",
					ResourceMemoryName: "nvidia.com/gpumem",
					ResourceCoreName:   "nvidia.com/gpucores",
					RequestName:        "gpu",
					DeviceType:         constants.NvidiaDeviceType,
				},
			}
			resp := a.Handle(context.Background(), req)
			require.True(t, resp.Allowed)
			assert.NotEmpty(t, resp.Patches)
			assert.Len(t, resp.Warnings, tc.wantWarnings)
		})
	}
}
