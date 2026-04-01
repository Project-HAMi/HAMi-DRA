// Package volcano contains webhook logic for Volcano jobs.
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
package volcano

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
	vcv1alpha1 "volcano.sh/apis/pkg/apis/batch/v1alpha1"

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
	job := &vcv1alpha1.Job{}
	err := a.Decoder.Decode(req, job)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	klog.V(5).Infof("Mutating volcano job(%s/%s) for request: %s", req.Namespace, job.Name, req.Operation)
	needPatch := false
	rctNameList := []string{}

	for i := range job.Spec.Tasks {
		task := &job.Spec.Tasks[i]
		rctName, err := a.handelTask(ctx, task, job)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if rctName != "" {
			needPatch = true
			rctNameList = append(rctNameList, rctName)
			task.Template.Spec.ResourceClaims = append(task.Template.Spec.ResourceClaims, corev1.PodResourceClaim{
				Name:                      rctName,
				ResourceClaimTemplateName: &rctName,
			})
		}
	}

	klog.V(5).InfoS("Job after patching", "job", job)
	if !needPatch {
		klog.V(5).Infof("No need to patch Job(%s/%s) for request: %s", req.Namespace, job.Name, req.Operation)
		return admission.Allowed("")
	}

	if job.Labels == nil {
		job.Labels = make(map[string]string)
	}
	job.Labels[constants.DraLabel] = "true"

	marshaledBytes, err := json.Marshal(job)
	if err != nil {
		// Cleanup the ResourceClaims created for this job
		for _, rctName := range rctNameList {
			deletionErr := a.Client.Delete(ctx, &resourceapi.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      rctName,
					Namespace: job.Namespace,
				},
			})
			if deletionErr != nil {
				klog.V(5).Infof("Failed to delete ResourceClaim(%s/%s) for request: %s after an error occurs", job.Namespace, rctName, req.Operation)
			}
		}
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledBytes)
}

func (a *MutatingAdmission) handelTask(ctx context.Context, task *vcv1alpha1.TaskSpec, job *vcv1alpha1.Job) (string, error) {
	for i := range task.Template.Spec.Containers {
		container := &task.Template.Spec.Containers[i]
		rctName, err := a.handelContainerTemplate(ctx, container, job.Namespace, task.Name)
		if err != nil {
			return "", err
		}
		if rctName != "" {
			return rctName, nil
		}
	}
	return "", nil
}

func (a *MutatingAdmission) handelContainerTemplate(ctx context.Context, container *corev1.Container, namespace, name string) (string, error) {
	countResourceName := corev1.ResourceName(a.DeviceConfig.ResourceCountName)
	countQty, ok := container.Resources.Limits[countResourceName]
	if !ok {
		return "", nil
	}

	// TODO: refactor the name generator to avoid too long name and avoid empty name for generated pod.
	rctName := fmt.Sprintf("%s-%s-%s", namespace, name, container.Name)
	resourceclaimtemplate := a.buildResourceClaimTemplate(rctName, namespace)

	resourceclaimtemplate.Spec.Spec.Devices.Requests[0].Exactly.Count = countQty.Value()

	// Remove count resource from container
	a.removeResource(container, countResourceName)

	if coreQty, ok := container.Resources.Limits[corev1.ResourceName(a.DeviceConfig.ResourceCoreName)]; ok {
		resourceclaimtemplate.Spec.Spec.Devices.Requests[0].Exactly.Capacity.Requests["cores"] = coreQty
		a.removeResource(container, corev1.ResourceName(a.DeviceConfig.ResourceCoreName))
	}
	if memQty, ok := container.Resources.Limits[corev1.ResourceName(a.DeviceConfig.ResourceMemoryName)]; ok {
		mem := resource.MustParse(fmt.Sprintf("%d", memQty.Value()*1024*1024))
		resourceclaimtemplate.Spec.Spec.Devices.Requests[0].Exactly.Capacity.Requests["memory"] = mem
		a.removeResource(container, corev1.ResourceName(a.DeviceConfig.ResourceMemoryName))
	}

	if err := a.Client.Create(ctx, resourceclaimtemplate); err != nil {
		return "", fmt.Errorf("failed to create ResourceClaimTemplate %s/%s: %w", namespace, rctName, err)
	}

	container.Resources.Claims = append(container.Resources.Claims, corev1.ResourceClaim{Name: rctName})

	klog.V(4).Infof("Successfully created ResourceClaimTemplate %s/%s", namespace, rctName)
	return rctName, nil
}

func (a *MutatingAdmission) buildResourceClaimTemplate(name, namespace string) *resourceapi.ResourceClaimTemplate {
	deviceClassName := a.DeviceConfig.EffectiveDeviceClassName()
	draDriverName := a.DeviceConfig.EffectiveDraDriverName()

	return &resourceapi.ResourceClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: resourceapi.ResourceClaimTemplateSpec{
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
								DeviceClassName: deviceClassName,
								Selectors: []resourceapi.DeviceSelector{
									{
										CEL: &resourceapi.CELDeviceSelector{
											Expression: fmt.Sprintf(`device.attributes["%s"].type == "%s"`, draDriverName, constants.NvidiaDeviceType),
										},
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
