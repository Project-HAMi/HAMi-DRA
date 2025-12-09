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
	"net/http"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Project-HAMi/HAMi-DRA/pkg/config"
	"github.com/Project-HAMi/HAMi-DRA/pkg/constants"
)

// MutatingAdmission mutates API request if necessary.
type MutatingAdmission struct {
	Decoder      admission.Decoder
	Client       client.Client
	DeviceConfig *config.NvidiaConfig
}

// Check if our MutatingAdmission implements necessary interface
var _ admission.Handler = &MutatingAdmission{}

// Handle yields a response to an AdmissionRequest.
func (a *MutatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}

	err := a.Decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	klog.V(5).Infof("Mutating Pod(%s/%s) for request: %s", req.Namespace, pod.Name, req.Operation)
	needPatch := false

	for i := range pod.Spec.Containers {
		container := &pod.Spec.Containers[i]
		rcName, err := a.handelContainer(ctx, container, pod)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if rcName != "" {
			needPatch = true
		}
		container.Resources.Claims = []corev1.ResourceClaim{{Name: rcName}}
		pod.Spec.ResourceClaims = append(pod.Spec.ResourceClaims, corev1.PodResourceClaim{
			Name:              rcName,
			ResourceClaimName: &rcName,
		})
	}

	klog.V(5).InfoS("Pod after patching", "pod", pod)
	if !needPatch {
		klog.V(5).Infof("No need to patch Pod(%s/%s) for request: %s", req.Namespace, pod.Name, req.Operation)
		return admission.Allowed("")
	}

	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels[constants.DraLabel] = "true"

	marshaledBytes, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledBytes)
}

func (a *MutatingAdmission) handelContainer(ctx context.Context, container *corev1.Container, pod *corev1.Pod) (string, error) {
	if _, ok := container.Resources.Limits[corev1.ResourceName(a.DeviceConfig.ResourceCountName)]; !ok {
		return "", nil
	}
	rcName := fmt.Sprintf("%s-%s-%s", pod.Namespace, pod.Name, container.Name)
	resourceclaim := &resourceapi.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rcName,
			Namespace: pod.Namespace,
		},
		Spec: resourceapi.ResourceClaimSpec{
			Devices: resourceapi.DeviceClaim{
				Requests: []resourceapi.DeviceRequest{
					{
						Name: "gpu",
						Exactly: &resourceapi.ExactDeviceRequest{
							AllocationMode: resourceapi.DeviceAllocationModeExactCount,
							Capacity: &resourceapi.CapacityRequirements{
								Requests: map[resourceapi.QualifiedName]resource.Quantity{},
							},
							DeviceClassName: constants.NvidiaDraDriver,
							Selectors: []resourceapi.DeviceSelector{
								{
									CEL: &resourceapi.CELDeviceSelector{
										Expression: fmt.Sprintf(`device.attributes["%s"].type == "%s"`, constants.NvidiaDraDriver, constants.NvidiaDeviceType),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	q, ok := container.Resources.Limits[corev1.ResourceName(a.DeviceConfig.ResourceCountName)]
	if !ok {
		klog.Errorf("Resource %s not found in Pod(%s/%s)", a.DeviceConfig.ResourceCountName, pod.Namespace, pod.Name)
		return "", fmt.Errorf("resource %s not found in Pod(%s/%s)", a.DeviceConfig.ResourceCountName, pod.Namespace, pod.Name)
	}
	resourceclaim.Spec.Devices.Requests[0].Exactly.Count = q.Value()
	delete(container.Resources.Requests, corev1.ResourceName(a.DeviceConfig.ResourceCountName))
	delete(container.Resources.Limits, corev1.ResourceName(a.DeviceConfig.ResourceCountName))
	c, ok := container.Resources.Limits[corev1.ResourceName(a.DeviceConfig.ResourceCoreName)]
	if ok {
		delete(container.Resources.Requests, corev1.ResourceName(a.DeviceConfig.ResourceCoreName))
		delete(container.Resources.Limits, corev1.ResourceName(a.DeviceConfig.ResourceCoreName))
		resourceclaim.Spec.Devices.Requests[0].Exactly.Capacity.Requests["cores"] = c
	}
	m, ok := container.Resources.Limits[corev1.ResourceName(a.DeviceConfig.ResourceMemoryName)]
	if ok {
		delete(container.Resources.Requests, corev1.ResourceName(a.DeviceConfig.ResourceMemoryName))
		delete(container.Resources.Limits, corev1.ResourceName(a.DeviceConfig.ResourceMemoryName))
		resourceclaim.Spec.Devices.Requests[0].Exactly.Capacity.Requests["memory"] = m
	}
	if uuid, ok := pod.Annotations[constants.UseUUIDAnnotation]; ok {
		resourceclaim.Spec.Devices.Requests[0].Exactly.Selectors = append(resourceclaim.Spec.Devices.Requests[0].Exactly.Selectors, resourceapi.DeviceSelector{
			CEL: &resourceapi.CELDeviceSelector{
				Expression: fmt.Sprintf(`device.attributes["%s"].uuid == "%s"`, constants.NvidiaDraDriver, uuid),
			},
		})
	}
	if deviceType, ok := pod.Annotations[constants.UseTypeAnnotation]; ok {
		resourceclaim.Spec.Devices.Requests[0].Exactly.Selectors = append(resourceclaim.Spec.Devices.Requests[0].Exactly.Selectors, resourceapi.DeviceSelector{
			CEL: &resourceapi.CELDeviceSelector{
				Expression: fmt.Sprintf(`device.attributes["%s"].productName == "%s"`, constants.NvidiaDraDriver, deviceType),
			},
		})
	}

	err := a.Client.Create(ctx, resourceclaim)
	if err != nil {
		return "", fmt.Errorf("failed to create ResourceClaim %s/%s: %w", pod.Namespace, rcName, err)
	}
	klog.V(4).Infof("Successfully created ResourceClaim %s/%s", pod.Namespace, rcName)
	return rcName, nil
}
