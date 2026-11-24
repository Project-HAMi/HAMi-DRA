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
	"testing"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	vcv1alpha1 "volcano.sh/apis/pkg/apis/batch/v1alpha1"

	"github.com/Project-HAMi/HAMi-DRA/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleTaskZeroCountRequestsNoDevice(t *testing.T) {
	sch := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(sch))
	fakeClient := fake.NewClientBuilder().WithScheme(sch).Build()
	admission := &MutatingAdmission{
		Client: fakeClient,
		DeviceConfig: &config.DRADeviceConfig{
			ResourceCountName: "nvidia.com/gpu",
			RequestName:       "gpu",
		},
	}
	task := &vcv1alpha1.TaskSpec{
		Name: "t",
		Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: "c",
			Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
				corev1.ResourceName("nvidia.com/gpu"): resource.MustParse("0"),
			}},
		}}}},
	}
	job := &vcv1alpha1.Job{ObjectMeta: metav1.ObjectMeta{Name: "j", Namespace: "default"}}

	names, _, err := admission.handleTask(context.Background(), task, job)
	require.NoError(t, err)
	assert.Empty(t, names)
	assert.Empty(t, task.Template.Spec.Containers[0].Resources.Claims)
	templates := &resourceapi.ResourceClaimTemplateList{}
	require.NoError(t, fakeClient.List(context.Background(), templates))
	assert.Empty(t, templates.Items)
}

func TestHandleTaskRejectsFractionalCount(t *testing.T) {
	sch := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(sch))
	admission := &MutatingAdmission{
		Client:       fake.NewClientBuilder().WithScheme(sch).Build(),
		DeviceConfig: &config.DRADeviceConfig{ResourceCountName: "nvidia.com/gpu", RequestName: "gpu"},
	}
	task := &vcv1alpha1.TaskSpec{
		Name: "t",
		Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name:      "c",
			Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{"nvidia.com/gpu": resource.MustParse("1500m")}},
		}}}},
	}
	job := &vcv1alpha1.Job{ObjectMeta: metav1.ObjectMeta{Name: "j", Namespace: "default"}}

	_, _, err := admission.handleTask(context.Background(), task, job)
	require.ErrorContains(t, err, "nvidia.com/gpu must be a whole number")
}
