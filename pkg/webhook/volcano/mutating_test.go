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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	vcv1alpha1 "volcano.sh/apis/pkg/apis/batch/v1alpha1"
	busv1alpha1 "volcano.sh/apis/pkg/apis/bus/v1alpha1"

	"github.com/Project-HAMi/HAMi-DRA/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var quickstartJob = &vcv1alpha1.Job{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "quickstart-job",
		Namespace: "default",
	},
	Spec: vcv1alpha1.JobSpec{
		MinAvailable:  3,
		SchedulerName: "volcano",
		Policies: []vcv1alpha1.LifecyclePolicy{
			{
				Event:  busv1alpha1.PodFailedEvent,
				Action: busv1alpha1.RestartJobAction,
			},
		},
		Tasks: []vcv1alpha1.TaskSpec{
			{
				Replicas: 3,
				Name:     "completion-task",
				Policies: []vcv1alpha1.LifecyclePolicy{
					{
						Event:  busv1alpha1.TaskCompletedEvent,
						Action: busv1alpha1.CompleteJobAction,
					},
				},
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:    "busybox-container",
								Image:   "busybox:latest",
								Command: []string{"sh", "-c", `echo "Job is running and will complete!"; sleep 100; echo "Job done!"`},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:                       resource.MustParse("1"),
										corev1.ResourceName("nvidia.com/gpu"):    resource.MustParse("1"),
										corev1.ResourceName("nvidia.com/gpumem"): resource.MustParse("1000"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:                       resource.MustParse("1"),
										corev1.ResourceName("nvidia.com/gpu"):    resource.MustParse("1"),
										corev1.ResourceName("nvidia.com/gpumem"): resource.MustParse("1000"),
									},
								},
							},
						},
						RestartPolicy: corev1.RestartPolicyNever,
					},
				},
			},
		},
	},
}

func TestMutatingAdmission_Handle(t *testing.T) {
	sch := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(sch))
	require.NoError(t, vcv1alpha1.AddToScheme(sch))

	decoder := admission.NewDecoder(sch)
	fakeClient := fake.NewClientBuilder().WithScheme(sch).Build()

	deviceConfig := &config.NvidiaConfig{
		ResourceCountName:  "nvidia.com/gpu",
		ResourceMemoryName: "nvidia.com/gpumem",
		ResourceCoreName:   "nvidia.com/gpucores",
	}

	tests := []struct {
		name        string
		job         *vcv1alpha1.Job
		wantPatched bool
	}{
		{
			name:        "quickstart-job from vcjob-quickstart.yaml",
			job:         quickstartJob.DeepCopy(),
			wantPatched: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobRaw, err := json.Marshal(tt.job)
			require.NoError(t, err)

			req := admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Namespace: tt.job.Namespace,
					Operation: admissionv1.Create,
					Object: runtime.RawExtension{
						Raw: jobRaw,
					},
				},
			}

			admission := &MutatingAdmission{
				Decoder:      decoder,
				Client:       fakeClient,
				DeviceConfig: deviceConfig,
			}

			resp := admission.Handle(context.Background(), req)

			if tt.wantPatched {
				assert.True(t, resp.Allowed, "expected patch to be allowed")
				assert.NotEmpty(t, resp.Patches, "expected non-empty patch")
			} else {
				assert.True(t, resp.Allowed, "expected allowed response")
				if resp.Patch != nil {
					assert.Empty(t, resp.Patches, "expected no patch when no GPU count resource")
				}
			}
		})
	}
}

func TestBuildResourceClaimTemplateUsesConfiguredDriver(t *testing.T) {
	admission := &MutatingAdmission{
		DeviceConfig: &config.NvidiaConfig{
			DeviceClassName: "fake-gpu.project-hami.io",
			DraDriverName:   "fake.dra.hami.io",
		},
	}

	template := admission.buildResourceClaimTemplate("test-template", "default")
	exactly := template.Spec.Spec.Devices.Requests[0].Exactly

	assert.Equal(t, "fake-gpu.project-hami.io", exactly.DeviceClassName)
	assert.Len(t, exactly.Selectors, 1)
	assert.Equal(t,
		`device.attributes["fake.dra.hami.io"].type == "hami-gpu"`,
		exactly.Selectors[0].CEL.Expression,
	)
}
