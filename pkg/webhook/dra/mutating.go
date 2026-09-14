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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

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
		rcNames, err := a.handleContainer(ctx, container, pod, rcNameList)
		if err != nil {
			a.deleteResourceClaims(ctx, pod.Namespace, append(rcNameList, rcNames...))
			return admission.Errored(http.StatusInternalServerError, err)
		}
		for _, rcName := range rcNames {
			needPatch = true
			rcNameList = append(rcNameList, rcName)
			container.Resources.Claims = append(container.Resources.Claims, corev1.ResourceClaim{Name: rcName})
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
		a.deleteResourceClaims(ctx, pod.Namespace, rcNameList)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledBytes)
}

func (a *MutatingAdmission) deleteResourceClaims(ctx context.Context, namespace string, rcNames []string) {
	for _, rcName := range rcNames {
		if deletionErr := a.Client.Delete(ctx, &resourceapi.ResourceClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      rcName,
				Namespace: namespace,
			},
		}); deletionErr != nil {
			klog.V(5).Infof("Failed to delete ResourceClaim(%s/%s) after an error occurs", namespace, rcName)
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

func (a *MutatingAdmission) handleContainer(ctx context.Context, container *corev1.Container, pod *corev1.Pod, createdClaims []string) ([]string, error) {
	var rcNames []string
	cleanup := func() {
		a.deleteResourceClaims(ctx, pod.Namespace, append(createdClaims, rcNames...))
	}

	for _, cfg := range a.configs() {
		countResourceName := corev1.ResourceName(cfg.ResourceCountName)
		countQty, ok := container.Resources.Limits[countResourceName]
		if !ok {
			continue
		}

		rcName := resourceClaimName(pod, container.Name, cfg)
		resourceclaim := a.buildResourceClaim(rcName, pod.Namespace, cfg)
		resourceclaim.Spec.Devices.Requests[0].Exactly.Count = countQty.Value()

		a.removeResource(container, countResourceName)

		if coreQty, ok := container.Resources.Limits[corev1.ResourceName(cfg.ResourceCoreName)]; ok {
			converted, err := cfg.ConvertCores(coreQty)
			if err != nil {
				cleanup()
				return nil, err
			}
			resourceclaim.Spec.Devices.Requests[0].Exactly.Capacity.Requests["cores"] = converted
			a.removeResource(container, corev1.ResourceName(cfg.ResourceCoreName))
		}
		if memQty, ok := container.Resources.Limits[corev1.ResourceName(cfg.ResourceMemoryName)]; ok {
			resourceclaim.Spec.Devices.Requests[0].Exactly.Capacity.Requests["memory"] = cfg.ConvertMemory(memQty)
			a.removeResource(container, corev1.ResourceName(cfg.ResourceMemoryName))
		}

		if err := a.addAnnotationSelectors(resourceclaim, pod, cfg); err != nil {
			cleanup()
			return nil, err
		}

		if err := a.Client.Create(ctx, resourceclaim); err != nil {
			cleanup()
			return nil, fmt.Errorf("failed to create ResourceClaim %s/%s: %w", pod.Namespace, rcName, err)
		}

		klog.V(4).Infof("Successfully created ResourceClaim %s/%s", pod.Namespace, rcName)
		rcNames = append(rcNames, rcName)
	}
	return rcNames, nil
}

// dns1123LabelMaxLength is the Kubernetes DNS-1123 label limit, which applies
// to PodResourceClaim.Name (and remains valid for ResourceClaim object names).
const dns1123LabelMaxLength = 63

func resourceClaimName(pod *corev1.Pod, containerName string, cfg *config.DRADeviceConfig) string {
	rcName := fmt.Sprintf("%s-%s-%s", pod.Namespace, pod.Name, containerName)
	if pod.Name == "" {
		rcName = fmt.Sprintf("%s-%s-%s", pod.Namespace, rand.String(5), containerName)
	}
	if cfg != nil && cfg.ClaimNameSuffix() != "" {
		rcName = fmt.Sprintf("%s-%s", rcName, cfg.ClaimNameSuffix())
	}
	return truncateDNS1123Label(rcName)
}

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

// buildResourceClaim creates a ResourceClaim with default selectors.
func (a *MutatingAdmission) buildResourceClaim(name, namespace string, cfg *config.DRADeviceConfig) *resourceapi.ResourceClaim {
	if cfg == nil {
		cfg = a.DeviceConfig
	}
	return &resourceapi.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: resourceapi.ResourceClaimSpec{
			Devices: resourceapi.DeviceClaim{
				Requests: []resourceapi.DeviceRequest{
					cfg.NewPrimaryDeviceRequest(),
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

// celStringList converts a comma-separated annotation value into CEL string
// literals, trimming whitespace and dropping empty elements.
func celStringList(raw string) []string {
	var literals []string
	for _, v := range strings.Split(raw, ",") {
		if v = strings.TrimSpace(v); v != "" {
			literals = append(literals, strconv.Quote(v))
		}
	}
	return literals
}

// addAnnotationSelectors adds device selectors based on pod annotations.
// It fails closed: an annotation that yields no usable value is an error,
// so an allow-list that resolved to empty cannot silently match any device.
func (a *MutatingAdmission) addAnnotationSelectors(resourceclaim *resourceapi.ResourceClaim, pod *corev1.Pod, cfg *config.DRADeviceConfig) error {
	exactly := resourceclaim.Spec.Devices.Requests[0].Exactly
	if cfg == nil {
		cfg = a.DeviceConfig
	}
	draDriverName := cfg.EffectiveDraDriverName()

	if pod == nil || pod.Annotations == nil {
		return nil
	}

	for _, sel := range []struct {
		annotation string
		field      string
		negate     bool
	}{
		{cfg.UseUUIDAnnotation, "uuid", false},
		{cfg.NoUseUUIDAnnotation, "uuid", true},
		{cfg.UseTypeAnnotation, "productName", false},
		{cfg.NoUseTypeAnnotation, "productName", true},
	} {
		raw, ok := pod.Annotations[sel.annotation]
		if !ok {
			continue
		}
		literals := celStringList(raw)
		if len(literals) == 0 {
			return fmt.Errorf("annotation %s has no usable value: %q", sel.annotation, raw)
		}
		expr := fmt.Sprintf(`device.attributes[%q].%s in [%s]`, draDriverName, sel.field, strings.Join(literals, ","))
		if sel.negate {
			expr = "!(" + expr + ")"
		}
		exactly.Selectors = append(exactly.Selectors, resourceapi.DeviceSelector{
			CEL: &resourceapi.CELDeviceSelector{Expression: expr},
		})
	}
	return nil
}
