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
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	vcv1alpha1 "volcano.sh/apis/pkg/apis/batch/v1alpha1"

	"github.com/Project-HAMi/HAMi-DRA/pkg/config"
	"github.com/Project-HAMi/HAMi-DRA/pkg/constants"
	"github.com/Project-HAMi/HAMi-DRA/pkg/webhook/dra"
)

// MutatingAdmission mutates API request if necessary.
type MutatingAdmission struct {
	Decoder admission.Decoder
	Client  client.Client
	// Reader reads a same-named template directly from the API server to check
	// whether it can be reused, without a cluster-wide informer.
	Reader        client.Reader
	DeviceConfig  *config.DRADeviceConfig
	DeviceConfigs []*config.DRADeviceConfig
}

// Check if our MutatingAdmission implements necessary interface
var _ admission.Handler = &MutatingAdmission{}

// Handle yields a response to an AdmissionRequest.
func (a *MutatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.DryRun != nil && *req.DryRun {
		// Send the claim writes as dry run too, so a dry run changes nothing.
		dry := *a
		dry.Client = client.NewDryRunClient(a.Client)
		a = &dry
	}
	job := &vcv1alpha1.Job{}
	err := a.Decoder.Decode(req, job)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	klog.V(5).Infof("Mutating volcano job(%s/%s) for request: %s", req.Namespace, job.Name, req.Operation)
	var containers []corev1.Container
	for _, task := range job.Spec.Tasks {
		containers = append(containers, task.Template.Spec.Containers...)
	}
	warnings := dra.ExtendedResourceWarnings(ctx, a.Client, a.configs(), containers)
	for _, w := range warnings {
		klog.Warningf("Job(%s/%s): %s", req.Namespace, job.Name, w)
	}
	// A re-applied job gets its device resources back, and since jobs are
	// patched as JSON merge patches it loses the claims added at creation.
	// Converting it again reuses the job's templates and restores the claims.
	rctNameList, created, err := a.convertJob(ctx, job)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	klog.V(5).InfoS("Job after patching", "job", job)
	if len(rctNameList) == 0 {
		klog.V(5).Infof("No need to patch Job(%s/%s) for request: %s", req.Namespace, job.Name, req.Operation)
		return admission.Allowed("")
	}

	marshaledBytes, err := json.Marshal(job)
	if err != nil {
		a.deleteResourceClaimTemplates(ctx, job.Namespace, created)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledBytes).WithWarnings(warnings...)
}

// convertJob moves the device resources of every task to templates and
// returns their names and those it created. A failure removes only the
// templates this request created; reused ones may belong to a running job.
func (a *MutatingAdmission) convertJob(ctx context.Context, job *vcv1alpha1.Job) (rctNameList, createdList []string, err error) {
	for i := range job.Spec.Tasks {
		task := &job.Spec.Tasks[i]
		rctNames, created, err := a.handleTask(ctx, task, job)
		createdList = append(createdList, created...)
		if err != nil {
			a.deleteResourceClaimTemplates(ctx, job.Namespace, createdList)
			return nil, nil, err
		}
		for _, rctName := range rctNames {
			rctNameList = append(rctNameList, rctName)
			if !slices.ContainsFunc(task.Template.Spec.ResourceClaims, func(c corev1.PodResourceClaim) bool { return c.Name == rctName }) {
				task.Template.Spec.ResourceClaims = append(task.Template.Spec.ResourceClaims, corev1.PodResourceClaim{
					Name:                      rctName,
					ResourceClaimTemplateName: &rctName,
				})
			}
		}
	}
	if len(rctNameList) > 0 {
		if job.Labels == nil {
			job.Labels = make(map[string]string)
		}
		job.Labels[constants.DraLabel] = "true"
	}
	return rctNameList, createdList, nil
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

func (a *MutatingAdmission) configs() []*config.DRADeviceConfig {
	if len(a.DeviceConfigs) > 0 {
		return a.DeviceConfigs
	}
	if a.DeviceConfig != nil {
		return []*config.DRADeviceConfig{a.DeviceConfig}
	}
	return nil
}

// handleTask processes every container in the task and returns the names of
// the ResourceClaimTemplates they use and of those it created.
func (a *MutatingAdmission) handleTask(ctx context.Context, task *vcv1alpha1.TaskSpec, job *vcv1alpha1.Job) (rctNames, created []string, err error) {
	// Task names repeat across jobs, so the template name must include the job.
	jobName := job.Name
	if jobName == "" {
		jobName = rand.String(5)
	}
	for i := range task.Template.Spec.Containers {
		container := &task.Template.Spec.Containers[i]
		names, newNames, err := a.handleContainerTemplate(ctx, container, job.Namespace, jobName+"-"+task.Name, jobOwner(job), task.Template.Annotations)
		rctNames = append(rctNames, names...)
		created = append(created, newNames...)
		if err != nil {
			return rctNames, created, err
		}
	}
	return rctNames, created, nil
}

// podPath runs the conversion steps shared with the Pod path.
func (a *MutatingAdmission) podPath() *dra.MutatingAdmission {
	return &dra.MutatingAdmission{Client: a.Client, DeviceConfig: a.DeviceConfig, DeviceConfigs: a.DeviceConfigs}
}

// createTemplate creates want and reports whether it did. A job rejected after
// this webhook (for example by Volcano's own validation) leaves its templates
// behind under the names it gets again when resubmitted; a leftover created for
// the same job and asking for the same devices is reused instead of failing on the name.
func (a *MutatingAdmission) createTemplate(ctx context.Context, want *resourceapi.ResourceClaimTemplate, owner string) (bool, error) {
	err := a.Client.Create(ctx, want)
	if !apierrors.IsAlreadyExists(err) {
		return err == nil, err
	}
	reader := a.Reader
	if reader == nil {
		reader = a.Client
	}
	existing := &resourceapi.ResourceClaimTemplate{}
	if getErr := reader.Get(ctx, client.ObjectKeyFromObject(want), existing); getErr == nil && existing.DeletionTimestamp == nil &&
		existing.Labels[constants.DraLabel] != "" && existing.Annotations[constants.DraOwnerAnnotation] == owner &&
		equality.Semantic.DeepEqual(existing.Spec.Spec, want.Spec.Spec) {
		klog.V(4).Infof("Reusing ResourceClaimTemplate %s left by an earlier admission", client.ObjectKeyFromObject(want))
		return false, nil
	}
	return false, fmt.Errorf("%w; it belongs to another job or asks for other devices, delete it to admit the job", err)
}

// jobOwner identifies the job a template is created for. A job created with
// generateName has no name yet at admission, so its prefix stands in.
func jobOwner(job *vcv1alpha1.Job) string {
	if job.Name != "" {
		return job.Name
	}
	return job.GenerateName
}

func (a *MutatingAdmission) handleContainerTemplate(ctx context.Context, container *corev1.Container, namespace, name, owner string, annotations map[string]string) (rctNames, created []string, err error) {
	for _, cfg := range a.configs() {
		dra.ApplyTaskPriority(container, cfg)
		countResourceName := corev1.ResourceName(cfg.ResourceCountName)
		countQty, ok := container.Resources.Limits[countResourceName]
		// As with the device plugin, a zero count asks for no device. A claim
		// with count 0 would be defaulted to one whole device.
		if !ok || countQty.Sign() <= 0 {
			continue
		}
		if countQty.MilliValue()%1000 != 0 {
			// The API server rejects a fractional extended resource, but only
			// after it is stripped here, so a claim would round it up.
			return rctNames, created, fmt.Errorf("%s must be a whole number, got %s", countResourceName, countQty.String())
		}

		raw := fmt.Sprintf("%s-%s-%s", namespace, name, container.Name)
		if cfg.ClaimNameSuffix() != "" {
			raw = fmt.Sprintf("%s-%s", raw, cfg.ClaimNameSuffix())
		}
		rctName := truncateDNS1123Label(raw)
		resourceclaimtemplate := a.buildResourceClaimTemplate(rctName, namespace, owner, cfg)

		resourceclaimtemplate.Spec.Spec.Devices.Requests[0].Exactly.Count = countQty.Value()

		a.removeResource(container, countResourceName)

		if coreQty, ok := container.Resources.Limits[corev1.ResourceName(cfg.ResourceCoreName)]; ok {
			converted, err := a.podPath().CoreCapacity(ctx, cfg, coreQty)
			if err != nil {
				return rctNames, created, err
			}
			resourceclaimtemplate.Spec.Spec.Devices.Requests[0].Exactly.Capacity.Requests[resourceapi.QualifiedName(cfg.EffectiveCoreCapacityName())] = converted
			a.removeResource(container, corev1.ResourceName(cfg.ResourceCoreName))
		}
		_, hasMem := container.Resources.Limits[corev1.ResourceName(cfg.ResourceMemoryName)]
		if memQty, ok := container.Resources.Limits[corev1.ResourceName(cfg.ResourceMemoryName)]; ok {
			memory, err := cfg.ConvertMemory(memQty)
			if err != nil {
				return rctNames, created, err
			}
			resourceclaimtemplate.Spec.Spec.Devices.Requests[0].Exactly.Capacity.Requests["memory"] = memory
			a.removeResource(container, corev1.ResourceName(cfg.ResourceMemoryName))
		}
		var pctQty *resource.Quantity
		if pctName := corev1.ResourceName(cfg.ResourceMemoryPercentageName); pctName != "" {
			if q, ok := container.Resources.Limits[pctName]; ok {
				a.removeResource(container, pctName)
				// As in HAMi, an absolute memory request wins over a percentage.
				if !hasMem {
					pctQty = &q
				}
			}
		}
		if err := a.podPath().CompleteClaimSpec(ctx, &resourceclaimtemplate.Spec.Spec, annotations, cfg, pctQty); err != nil {
			return rctNames, created, err
		}

		isNew, err := a.createTemplate(ctx, resourceclaimtemplate, owner)
		if err != nil {
			return rctNames, created, fmt.Errorf("failed to create ResourceClaimTemplate %s/%s: %w", namespace, rctName, err)
		}
		if isNew {
			created = append(created, rctName)
		}

		if !slices.ContainsFunc(container.Resources.Claims, func(c corev1.ResourceClaim) bool { return c.Name == rctName }) {
			container.Resources.Claims = append(container.Resources.Claims, corev1.ResourceClaim{Name: rctName})
		}

		klog.V(4).Infof("Successfully created ResourceClaimTemplate %s/%s", namespace, rctName)
		rctNames = append(rctNames, rctName)
	}
	return rctNames, created, nil
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

func (a *MutatingAdmission) buildResourceClaimTemplate(name, namespace, owner string, cfg *config.DRADeviceConfig) *resourceapi.ResourceClaimTemplate {
	if cfg == nil {
		cfg = a.DeviceConfig
	}
	return &resourceapi.ResourceClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			// Job deletion removes only templates this webhook created for that job.
			Labels:      map[string]string{constants.DraLabel: "true"},
			Annotations: map[string]string{constants.DraOwnerAnnotation: owner},
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
