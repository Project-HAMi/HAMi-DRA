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

	"github.com/Project-HAMi/HAMi-DRA/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// ValidatingAdmission validates API request when creating/updating/deleting.
type ValidatingAdmission struct {
	Decoder admission.Decoder
	Client  client.Client
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}

// This is temporary solution to delete ResourceClaim when Pod is deleted. And it will be replaced in the future.
func (v *ValidatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}

	if err := json.Unmarshal(req.OldObject.Raw, pod); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	if _, ok := pod.Labels[constants.DraLabel]; !ok {
		return admission.Allowed("")
	}
	klog.V(5).Infof("Validating Pod(%s/%s) for request: %s", req.Namespace, pod.Name, req.Operation)

	for _, container := range pod.Spec.Containers {
		err := v.Client.Delete(ctx, &resourceapi.ResourceClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s-%s", pod.Namespace, pod.Name, container.Name),
				Namespace: pod.Namespace,
			},
		})
		if err != nil && !apierrors.IsNotFound(err) {
			klog.Warningf("Failed to delete ResourceClaim %s/%s: %v", pod.Namespace, fmt.Sprintf("%s-%s-%s", pod.Namespace, pod.Name, container.Name), err)
			continue
		}
	}

	return admission.Allowed("")
}
