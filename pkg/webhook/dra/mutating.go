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
	rcNameList := []string{}

	for i := range pod.Spec.Containers {
		container := &pod.Spec.Containers[i]
		rcName, err := a.handelContainer(ctx, container, pod)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if rcName != "" {
			needPatch = true
			rcNameList = append(rcNameList, rcName)
			container.Resources.Claims = []corev1.ResourceClaim{{Name: rcName}}
			pod.Spec.ResourceClaims = append(pod.Spec.ResourceClaims, corev1.PodResourceClaim{
				Name:              rcName,
				ResourceClaimName: &rcName,
			})
		}
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
		// Cleanup the ResourceClaims created for this pod
		for _, rcName := range rcNameList {
			deletionErr := a.Client.Delete(ctx, &resourceapi.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      rcName,
					Namespace: pod.Namespace,
				},
			})
			if deletionErr != nil {
				klog.V(5).Infof("Failed to delete ResourceClaim(%s/%s) for request: %s after an error occurs", pod.Namespace, pod.Name, req.Operation)
			}
		}
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledBytes)
}

func (a *MutatingAdmission) handelContainer(ctx context.Context, container *corev1.Container, pod *corev1.Pod) (string, error) {
	countResourceName := corev1.ResourceName(a.DeviceConfig.ResourceCountName)
	countQty, ok := container.Resources.Limits[countResourceName]
	if !ok {
		return "", nil
	}

	rcName := fmt.Sprintf("%s-%s-%s", pod.Namespace, pod.Name, container.Name)
	resourceclaim := a.buildResourceClaim(rcName, pod.Namespace)

	resourceclaim.Spec.Devices.Requests[0].Exactly.Count = countQty.Value()

	// Remove count resource from container
	a.removeResource(container, countResourceName)

	if coreQty, ok := container.Resources.Limits[corev1.ResourceName(a.DeviceConfig.ResourceCoreName)]; ok {
		resourceclaim.Spec.Devices.Requests[0].Exactly.Capacity.Requests["cores"] = coreQty
		a.removeResource(container, corev1.ResourceName(a.DeviceConfig.ResourceCoreName))
	}
	if memQty, ok := container.Resources.Limits[corev1.ResourceName(a.DeviceConfig.ResourceMemoryName)]; ok {
		resourceclaim.Spec.Devices.Requests[0].Exactly.Capacity.Requests["memory"] = memQty
		a.removeResource(container, corev1.ResourceName(a.DeviceConfig.ResourceMemoryName))
	}

	a.addAnnotationSelectors(resourceclaim, pod)

	if err := a.Client.Create(ctx, resourceclaim); err != nil {
		return "", fmt.Errorf("failed to create ResourceClaim %s/%s: %w", pod.Namespace, rcName, err)
	}

	klog.V(4).Infof("Successfully created ResourceClaim %s/%s", pod.Namespace, rcName)
	return rcName, nil
}

// buildResourceClaim creates a ResourceClaim with default selectors.
func (a *MutatingAdmission) buildResourceClaim(name, namespace string) *resourceapi.ResourceClaim {
	return &resourceapi.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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
}

// removeResource removes a resource from both Requests and Limits
func (a *MutatingAdmission) removeResource(container *corev1.Container, resourceName corev1.ResourceName) {
	if container.Resources.Requests != nil {
		delete(container.Resources.Requests, resourceName)
	}
	if container.Resources.Limits != nil {
		delete(container.Resources.Limits, resourceName)
	}
}

// addAnnotationSelectors adds device selectors based on pod annotations.
func (a *MutatingAdmission) addAnnotationSelectors(resourceclaim *resourceapi.ResourceClaim, pod *corev1.Pod) {
	exactly := resourceclaim.Spec.Devices.Requests[0].Exactly

	if uuid, ok := pod.Annotations[constants.UseUUIDAnnotation]; ok {
		exactly.Selectors = append(exactly.Selectors, resourceapi.DeviceSelector{
			CEL: &resourceapi.CELDeviceSelector{
				Expression: fmt.Sprintf(`device.attributes["%s"].uuid == "%s"`, constants.NvidiaDraDriver, uuid),
			},
		})
	}

	if deviceType, ok := pod.Annotations[constants.UseTypeAnnotation]; ok {
		exactly.Selectors = append(exactly.Selectors, resourceapi.DeviceSelector{
			CEL: &resourceapi.CELDeviceSelector{
				Expression: fmt.Sprintf(`device.attributes["%s"].productName == "%s"`, constants.NvidiaDraDriver, deviceType),
			},
		})
	}
}
