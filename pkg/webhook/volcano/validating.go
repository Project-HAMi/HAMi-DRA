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
	"net/http"
	"strings"

	"github.com/Project-HAMi/HAMi-DRA/pkg/constants"
	resourceapi "k8s.io/api/resource/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	vcv1alpha1 "volcano.sh/apis/pkg/apis/batch/v1alpha1"
)

// ValidatingAdmission validates API request when creating/updating/deleting.
type ValidatingAdmission struct {
	Decoder admission.Decoder
	Client  client.Client
	// Reader reads ResourceClaimTemplates directly from the API server to avoid a cluster-wide informer.
	Reader client.Reader
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}

// Handle deletes the ResourceClaim when a DRA-managed Volcano job is deleted.
func (v *ValidatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.DryRun != nil && *req.DryRun {
		// Send the claim writes as dry run too, so a dry run changes nothing.
		dry := *v
		dry.Client = client.NewDryRunClient(v.Client)
		v = &dry
	}
	job := &vcv1alpha1.Job{}

	if err := json.Unmarshal(req.OldObject.Raw, job); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	if _, ok := job.Labels[constants.DraLabel]; !ok {
		return admission.Allowed("")
	}
	klog.V(5).Infof("Validating Job(%s/%s) for request: %s", req.Namespace, job.Name, req.Operation)

	for _, name := range getResourceClaimTemplateName(job) {
		v.deleteResourceClaimTemplate(ctx, job, name)
	}

	return admission.Allowed("")
}

// deleteResourceClaimTemplate deletes the template only if the mutating webhook
// created it for this job, so user-managed templates and those of other jobs
// that the job merely references are left alone.
func (v *ValidatingAdmission) deleteResourceClaimTemplate(ctx context.Context, job *vcv1alpha1.Job, name string) {
	template := &resourceapi.ResourceClaimTemplate{}
	if err := v.Reader.Get(ctx, client.ObjectKey{Namespace: job.Namespace, Name: name}, template); err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Warningf("Failed to get ResourceClaimTemplate %s/%s: %v", job.Namespace, name, err)
		}
		return
	}
	if _, ok := template.Labels[constants.DraLabel]; !ok || !createdFor(template.Annotations[constants.DraOwnerAnnotation], job) {
		return
	}
	err := v.Client.Delete(ctx, template, client.Preconditions{UID: &template.UID})
	if err != nil && !apierrors.IsNotFound(err) {
		klog.Warningf("Failed to delete ResourceClaimTemplate %s/%s: %v", job.Namespace, name, err)
	}
}

// createdFor reports whether owner, recorded at admission, is this job: its
// name, or the generateName prefix it was created with.
func createdFor(owner string, job *vcv1alpha1.Job) bool {
	if owner == "" {
		return false
	}
	return owner == job.Name || (owner == job.GenerateName && strings.HasPrefix(job.Name, owner))
}

func getResourceClaimTemplateName(job *vcv1alpha1.Job) []string {
	var names []string
	for _, task := range job.Spec.Tasks {
		for _, rc := range task.Template.Spec.ResourceClaims {
			if rc.ResourceClaimTemplateName != nil {
				names = append(names, *rc.ResourceClaimTemplateName)
			}
		}
	}
	return names
}
