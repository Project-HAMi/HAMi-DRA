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
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
	// ResourceClaimTemplate makes every converted Pod reference a ResourceClaimTemplate.
	// Kubernetes then generates the ResourceClaim for that Pod.
	ResourceClaimTemplate bool
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
	pod := &corev1.Pod{}
	err := a.Decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	klog.V(5).Infof("Mutating Pod(%s/%s) for request: %s", req.Namespace, pod.Name, req.Operation)
	if req.Operation == admissionv1.Update {
		return a.handleUpdate(req, pod)
	}
	needPatch := false
	rcNameList := []string{}
	asTemplate := a.ResourceClaimTemplate
	warnings := ExtendedResourceWarnings(ctx, a.Client, a.configs(), pod.Spec.Containers)
	for _, w := range warnings {
		klog.Warningf("Pod(%s/%s): %s", req.Namespace, pod.Name, w)
	}

	for i := range pod.Spec.Containers {
		container := &pod.Spec.Containers[i]
		rcNames, err := a.handleContainer(ctx, container, pod, rcNameList, asTemplate)
		if err != nil {
			a.deleteResourceClaims(ctx, pod.Namespace, append(rcNameList, rcNames...), asTemplate)
			return admission.Errored(http.StatusInternalServerError, err)
		}
		for _, rcName := range rcNames {
			needPatch = true
			rcNameList = append(rcNameList, rcName)
			container.Resources.Claims = append(container.Resources.Claims, corev1.ResourceClaim{Name: rcName})
			podClaim := corev1.PodResourceClaim{Name: rcName}
			if asTemplate {
				podClaim.ResourceClaimTemplateName = &rcName
			} else {
				podClaim.ResourceClaimName = &rcName
			}
			pod.Spec.ResourceClaims = append(pod.Spec.ResourceClaims, podClaim)
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
		a.deleteResourceClaims(ctx, pod.Namespace, rcNameList, asTemplate)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledBytes).WithWarnings(warnings...)
}

// handleUpdate strips the converted resources again when a pod that already
// got its claims is re-applied, since pod resources can't change after create.
func (a *MutatingAdmission) handleUpdate(req admission.Request, pod *corev1.Pod) admission.Response {
	if !stripConverted(pod.Spec.Containers, a.configs()) {
		return admission.Allowed("")
	}
	marshaledBytes, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledBytes)
}

// stripConverted removes the resources this webhook converts to claims from
// containers that already hold claims, and reports whether any were found.
func stripConverted(containers []corev1.Container, cfgs []*config.DRADeviceConfig) bool {
	found := false
	for i := range containers {
		res := &containers[i].Resources
		if len(res.Claims) == 0 {
			continue
		}
		for _, cfg := range cfgs {
			for _, name := range []string{cfg.ResourceCountName, cfg.ResourceCoreName, cfg.ResourceMemoryName, cfg.ResourceMemoryPercentageName} {
				if name == "" {
					continue
				}
				_, inLimits := res.Limits[corev1.ResourceName(name)]
				_, inRequests := res.Requests[corev1.ResourceName(name)]
				if inLimits || inRequests {
					delete(res.Limits, corev1.ResourceName(name))
					delete(res.Requests, corev1.ResourceName(name))
					found = true
				}
			}
		}
	}
	return found
}

// newClaimObject wraps spec in a ResourceClaim, or in a ResourceClaimTemplate when asTemplate is set.
// Templates carry the DRA label so that Pod deletion only removes templates this webhook created.
func newClaimObject(name, namespace string, spec resourceapi.ResourceClaimSpec, asTemplate bool) client.Object {
	meta := metav1.ObjectMeta{Name: name, Namespace: namespace}
	if asTemplate {
		meta.Labels = map[string]string{constants.DraLabel: "true"}
		return &resourceapi.ResourceClaimTemplate{
			ObjectMeta: meta,
			Spec:       resourceapi.ResourceClaimTemplateSpec{Spec: spec},
		}
	}
	return &resourceapi.ResourceClaim{ObjectMeta: meta, Spec: spec}
}

func claimKind(asTemplate bool) string {
	if asTemplate {
		return "ResourceClaimTemplate"
	}
	return "ResourceClaim"
}

func (a *MutatingAdmission) deleteResourceClaims(ctx context.Context, namespace string, rcNames []string, asTemplate bool) {
	for _, rcName := range rcNames {
		obj := newClaimObject(rcName, namespace, resourceapi.ResourceClaimSpec{}, asTemplate)
		if deletionErr := a.Client.Delete(ctx, obj); deletionErr != nil {
			klog.V(5).Infof("Failed to delete %s(%s/%s) after an error occurs", claimKind(asTemplate), namespace, rcName)
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

func (a *MutatingAdmission) handleContainer(ctx context.Context, container *corev1.Container, pod *corev1.Pod, createdClaims []string, asTemplate bool) ([]string, error) {
	var rcNames []string
	cleanup := func() {
		a.deleteResourceClaims(ctx, pod.Namespace, append(createdClaims, rcNames...), asTemplate)
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
			resourceclaim.Spec.Devices.Requests[0].Exactly.Capacity.Requests[resourceapi.QualifiedName(cfg.EffectiveCoreCapacityName())] = converted
			a.removeResource(container, corev1.ResourceName(cfg.ResourceCoreName))
		}
		_, hasMem := container.Resources.Limits[corev1.ResourceName(cfg.ResourceMemoryName)]
		if memQty, ok := container.Resources.Limits[corev1.ResourceName(cfg.ResourceMemoryName)]; ok {
			memory, err := cfg.ConvertMemory(memQty)
			if err != nil {
				cleanup()
				return nil, err
			}
			resourceclaim.Spec.Devices.Requests[0].Exactly.Capacity.Requests["memory"] = memory
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

		if err := a.addAnnotationSelectors(resourceclaim, pod, cfg); err != nil {
			cleanup()
			return nil, err
		}
		if pctQty != nil {
			if err := a.requestMemoryPercentage(ctx, resourceclaim, cfg, *pctQty); err != nil {
				cleanup()
				return nil, err
			}
		}

		obj := newClaimObject(rcName, pod.Namespace, resourceclaim.Spec, asTemplate)
		if err := a.Client.Create(ctx, obj); err != nil {
			cleanup()
			return nil, fmt.Errorf("failed to create %s %s/%s: %w", claimKind(asTemplate), pod.Namespace, rcName, err)
		}
		klog.V(4).Infof("Successfully created %s %s/%s", claimKind(asTemplate), pod.Namespace, rcName)
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
		attr := fmt.Sprintf(`device.attributes[%q].%s`, draDriverName, sel.field)
		match := fmt.Sprintf(`%s in [%s]`, attr, strings.Join(literals, ","))
		substring := sel.field == "productName" && cfg.TypeSubstringMatch
		if substring {
			var parts []string
			for _, v := range strings.Split(raw, ",") {
				if v = strings.TrimSpace(v); v != "" {
					parts = append(parts, strconv.Quote(typeNamePart(v)))
				}
			}
			match = fmt.Sprintf(`[%s].exists(t, %s.lowerAscii().contains(t))`, strings.Join(parts, ","), attr)
		}
		expr := match
		switch {
		case sel.negate:
			// A device without the attribute is not on the exclusion list; looking
			// it up anyway is a CEL runtime error that blocks the allocation.
			expr = "!has(" + attr + ") || !(" + match + ")"
		case substring:
			expr = "has(" + attr + ") && " + match
		}
		exactly.Selectors = append(exactly.Selectors, resourceapi.DeviceSelector{
			CEL: &resourceapi.CELDeviceSelector{Expression: expr},
		})
	}
	return nil
}

// typeNamePart turns a type annotation into the form the AMD driver
// publishes productName in, lower-cased: spaces become "_" and parentheses
// are dropped, so "Radeon RX 9060" finds "AMD_Radeon_RX_9060_XT".
func typeNamePart(raw string) string {
	return strings.NewReplacer(" ", "_", "(", "", ")", "").Replace(strings.ToLower(raw))
}

// requestMemoryPercentage turns a share of each device's memory into one
// subrequest per memory size the driver publishes, since a claim can only
// request an absolute capacity. The scheduler uses the first subrequest a
// device fits, so each device gets the share of its own memory.
func (a *MutatingAdmission) requestMemoryPercentage(ctx context.Context, claim *resourceapi.ResourceClaim, cfg *config.DRADeviceConfig, pctQty resource.Quantity) error {
	pct := pctQty.Value()
	if pct < 1 || pct > 100 {
		return fmt.Errorf("%s must be between 1 and 100, got %s", cfg.ResourceMemoryPercentageName, pctQty.String())
	}
	sizes, err := a.deviceMemorySizes(ctx, cfg)
	if err != nil {
		return err
	}
	if len(sizes) == 0 {
		return fmt.Errorf("no %s devices are published, so %s cannot be sized", cfg.EffectiveDraDriverName(), cfg.ResourceMemoryPercentageName)
	}
	if len(sizes) > resourceapi.FirstAvailableDeviceRequestMaxSize {
		return fmt.Errorf("%s devices come in %d memory sizes, %s supports at most %d", cfg.EffectiveDraDriverName(), len(sizes), cfg.ResourceMemoryPercentageName, resourceapi.FirstAvailableDeviceRequestMaxSize)
	}

	req := &claim.Spec.Devices.Requests[0]
	exactly := req.Exactly
	for i, size := range sizes {
		// HAMi sizes the share in whole MiB, at least 1.
		mib := max((size>>20)*pct/100, 1)
		requests := maps.Clone(exactly.Capacity.Requests)
		requests["memory"] = *resource.NewQuantity(mib<<20, resource.BinarySI)
		req.FirstAvailable = append(req.FirstAvailable, resourceapi.DeviceSubRequest{
			Name:            fmt.Sprintf("mem-%d", i),
			DeviceClassName: exactly.DeviceClassName,
			Selectors: append(slices.Clone(exactly.Selectors), resourceapi.DeviceSelector{CEL: &resourceapi.CELDeviceSelector{
				Expression: fmt.Sprintf(`device.capacity[%q].memory.compareTo(quantity(%q)) == 0`, cfg.EffectiveDraDriverName(), strconv.FormatInt(size, 10)),
			}}),
			AllocationMode: exactly.AllocationMode,
			Count:          exactly.Count,
			Tolerations:    exactly.Tolerations,
			Capacity:       &resourceapi.CapacityRequirements{Requests: requests},
		})
	}
	req.Exactly = nil
	return nil
}

// deviceMemorySizes returns the distinct memory capacities, in bytes, of the
// devices of cfg's driver and type in the published ResourceSlices.
func (a *MutatingAdmission) deviceMemorySizes(ctx context.Context, cfg *config.DRADeviceConfig) ([]int64, error) {
	// The cached List waits for the informer to sync; bound it like
	// ExtendedResourceWarnings does so admission can't hang on it.
	listCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	list := &resourceapi.ResourceSliceList{}
	if err := a.Client.List(listCtx, list); err != nil {
		return nil, fmt.Errorf("listing ResourceSlices to size %s: %w", cfg.ResourceMemoryPercentageName, err)
	}
	seen := map[int64]bool{}
	var sizes []int64
	for _, rs := range list.Items {
		if rs.Spec.Driver != cfg.EffectiveDraDriverName() {
			continue
		}
		for _, d := range rs.Spec.Devices {
			if t := d.Attributes["type"].StringValue; t == nil || *t != cfg.DeviceType {
				continue
			}
			if c, ok := d.Capacity["memory"]; ok && !seen[c.Value.Value()] {
				seen[c.Value.Value()] = true
				sizes = append(sizes, c.Value.Value())
			}
		}
	}
	slices.Sort(sizes)
	return sizes, nil
}
