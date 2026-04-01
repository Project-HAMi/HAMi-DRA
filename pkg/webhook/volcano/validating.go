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

	"github.com/Project-HAMi/HAMi-DRA/pkg/constants"
	resourceapi "k8s.io/api/resource/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	vcv1alpha1 "volcano.sh/apis/pkg/apis/batch/v1alpha1"
)

// ValidatingAdmission validates API request when creating/updating/deleting.
type ValidatingAdmission struct {
	Decoder admission.Decoder
	Client  client.Client
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}

// Handle deletes the ResourceClaim when a DRA-managed Volcano job is deleted.
func (v *ValidatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	job := &vcv1alpha1.Job{}

	if err := json.Unmarshal(req.OldObject.Raw, job); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	if _, ok := job.Labels[constants.DraLabel]; !ok {
		return admission.Allowed("")
	}
	klog.V(5).Infof("Validating Job(%s/%s) for request: %s", req.Namespace, job.Name, req.Operation)

	resourceClaimNameList := getResourceClaimName(job)
	for _, resourceClaimName := range resourceClaimNameList {
		err := v.Client.Delete(ctx, &resourceapi.ResourceClaimTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceClaimName,
				Namespace: job.Namespace,
			},
		})
		if err != nil && !apierrors.IsNotFound(err) {
			klog.Warningf("Failed to delete ResourceClaim %s/%s: %v", job.Namespace, resourceClaimName, err)
			continue
		}
	}

	return admission.Allowed("")
}

func getResourceClaimName(job *vcv1alpha1.Job) []string {
	resourceClaimNameList := []string{}
	for _, task := range job.Spec.Tasks {
		for _, resourceClaim := range task.Template.Spec.ResourceClaims {
			resourceClaimNameList = append(resourceClaimNameList, resourceClaim.Name)
		}
	}
	return resourceClaimNameList
}
