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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	vcv1alpha1 "volcano.sh/apis/pkg/apis/batch/v1alpha1"

	"github.com/Project-HAMi/HAMi-DRA/pkg/config"
	"github.com/Project-HAMi/HAMi-DRA/pkg/constants"
)

// MutatingAdmission mutates API request if necessary.
type MutatingAdmission struct {
	Decoder       admission.Decoder
	Client        client.Client
	DeviceConfig  *config.DRADeviceConfig
	DeviceConfigs []*config.DRADeviceConfig
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
		rctNames, err := a.handleTask(ctx, task, job)
		if err != nil {
			a.deleteResourceClaimTemplates(ctx, job.Namespace, append(rctNameList, rctNames...))
			return admission.Errored(http.StatusInternalServerError, err)
		}
		for _, rctName := range rctNames {
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
		a.deleteResourceClaimTemplates(ctx, job.Namespace, rctNameList)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledBytes)
}

// deleteResourceClaimTemplates removes templates created earlier in the same
// request after a later step fails.
func (a *MutatingAdmission) deleteResourceClaimTemplates(ctx context.Context, namespace string, names []string) {
	for _, name := range names {
		if deletionErr := a.Client.Delete(ctx, &resourceapi.ResourceClaimTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}); deletionErr != nil {
			klog.Errorf("Failed to delete ResourceClaimTemplate(%s/%s), it may need manual cleanup: %v", namespace, name, deletionErr)
		}
	}
}

// handleTask processes every container in the task and returns the names of
// the ResourceClaimTemplates created for them.
func (a *MutatingAdmission) configs() []*config.DRADeviceConfig {
	if len(a.DeviceConfigs) > 0 {
		return a.DeviceConfigs
	}
	if a.DeviceConfig != nil {
		return []*config.DRADeviceConfig{a.DeviceConfig}
	}
	return nil
}

func (a *MutatingAdmission) handleTask(ctx context.Context, task *vcv1alpha1.TaskSpec, job *vcv1alpha1.Job) ([]string, error) {
	var rctNames []string
	// Task names repeat across jobs, so the template name must include the job.
	jobName := job.Name
	if jobName == "" {
		jobName = rand.String(5)
	}
	for i := range task.Template.Spec.Containers {
		container := &task.Template.Spec.Containers[i]
		names, err := a.handleContainerTemplate(ctx, container, job.Namespace, jobName+"-"+task.Name)
		rctNames = append(rctNames, names...)
		if err != nil {
			return rctNames, err
		}
	}
	return rctNames, nil
}

func (a *MutatingAdmission) handleContainerTemplate(ctx context.Context, container *corev1.Container, namespace, name string) ([]string, error) {
	var rctNames []string
	for _, cfg := range a.configs() {
		countResourceName := corev1.ResourceName(cfg.ResourceCountName)
		countQty, ok := container.Resources.Limits[countResourceName]
		if !ok {
			continue
		}

		raw := fmt.Sprintf("%s-%s-%s", namespace, name, container.Name)
		if cfg.ClaimNameSuffix() != "" {
			raw = fmt.Sprintf("%s-%s", raw, cfg.ClaimNameSuffix())
		}
		rctName := truncateDNS1123Label(raw)
		resourceclaimtemplate := a.buildResourceClaimTemplate(rctName, namespace, cfg)

		resourceclaimtemplate.Spec.Spec.Devices.Requests[0].Exactly.Count = countQty.Value()

		a.removeResource(container, countResourceName)

		if coreQty, ok := container.Resources.Limits[corev1.ResourceName(cfg.ResourceCoreName)]; ok {
			converted, err := cfg.ConvertCores(coreQty)
			if err != nil {
				return rctNames, err
			}
			resourceclaimtemplate.Spec.Spec.Devices.Requests[0].Exactly.Capacity.Requests["cores"] = converted
			a.removeResource(container, corev1.ResourceName(cfg.ResourceCoreName))
		}
		if memQty, ok := container.Resources.Limits[corev1.ResourceName(cfg.ResourceMemoryName)]; ok {
			resourceclaimtemplate.Spec.Spec.Devices.Requests[0].Exactly.Capacity.Requests["memory"] = cfg.ConvertMemory(memQty)
			a.removeResource(container, corev1.ResourceName(cfg.ResourceMemoryName))
		}

		if err := a.Client.Create(ctx, resourceclaimtemplate); err != nil {
			return rctNames, fmt.Errorf("failed to create ResourceClaimTemplate %s/%s: %w", namespace, rctName, err)
		}

		container.Resources.Claims = append(container.Resources.Claims, corev1.ResourceClaim{Name: rctName})

		klog.V(4).Infof("Successfully created ResourceClaimTemplate %s/%s", namespace, rctName)
		rctNames = append(rctNames, rctName)
	}
	return rctNames, nil
}

const dns1123LabelMaxLength = 63

func truncateDNS1123Label(name string) string {
	if len(name) <= dns1123LabelMaxLength {
		return name
	}
	h := sha256.Sum256([]byte(name))
	suffix := fmt.Sprintf("-%x", h[:4])
	prefixLen := dns1123LabelMaxLength - len(suffix)
	prefix := strings.TrimRight(name[:prefixLen], "-")
	if prefix == "" {
		return strings.TrimLeft(suffix, "-")
	}
	return prefix + suffix
}

func (a *MutatingAdmission) buildResourceClaimTemplate(name, namespace string, cfg *config.DRADeviceConfig) *resourceapi.ResourceClaimTemplate {
	if cfg == nil {
		cfg = a.DeviceConfig
	}
	return &resourceapi.ResourceClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: resourceapi.ResourceClaimTemplateSpec{
			Spec: resourceapi.ResourceClaimSpec{
				Devices: resourceapi.DeviceClaim{
					Requests: []resourceapi.DeviceRequest{
						cfg.NewPrimaryDeviceRequest(),
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
