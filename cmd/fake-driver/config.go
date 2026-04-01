/*
Copyright 2026 The HAMi Authors.

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

package main

import (
	"context"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	coreclientset "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"
)

type DriverConfig struct {
	NodeSelector *metav1.LabelSelector `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	Groups       []DeviceGroup         `json:"groups,omitempty" yaml:"groups,omitempty"`
	Nodes        map[string]NodeConfig `json:"nodes,omitempty" yaml:"nodes,omitempty"`
}

type DeviceGroup struct {
	Name     string                `json:"name,omitempty" yaml:"name,omitempty"`
	Selector *metav1.LabelSelector `json:"selector,omitempty" yaml:"selector,omitempty"`
	Devices  []FakeDevice          `json:"devices" yaml:"devices"`
}

type NodeConfig struct {
	Devices []FakeDevice `json:"devices" yaml:"devices"`
}

type FakeDevice struct {
	Name                     string                        `json:"name" yaml:"name"`
	AllowMultipleAllocations *bool                         `json:"allowMultipleAllocations,omitempty" yaml:"allowMultipleAllocations,omitempty"`
	Attributes               map[string]FakeAttributeValue `json:"attributes,omitempty" yaml:"attributes,omitempty"`
	Capacity                 map[string]FakeCapacity       `json:"capacity,omitempty" yaml:"capacity,omitempty"`
}

type FakeAttributeValue struct {
	Int     *int64  `json:"int,omitempty" yaml:"int,omitempty"`
	Bool    *bool   `json:"bool,omitempty" yaml:"bool,omitempty"`
	String  *string `json:"string,omitempty" yaml:"string,omitempty"`
	Version *string `json:"version,omitempty" yaml:"version,omitempty"`
}

type FakeCapacity struct {
	Value         string                     `json:"value" yaml:"value"`
	RequestPolicy *FakeCapacityRequestPolicy `json:"requestPolicy,omitempty" yaml:"requestPolicy,omitempty"`
}

type FakeCapacityRequestPolicy struct {
	Default     *string                  `json:"default,omitempty" yaml:"default,omitempty"`
	ValidValues []string                 `json:"validValues,omitempty" yaml:"validValues,omitempty"`
	ValidRange  *FakeCapacityPolicyRange `json:"validRange,omitempty" yaml:"validRange,omitempty"`
}

type FakeCapacityPolicyRange struct {
	Min  *string `json:"min,omitempty" yaml:"min,omitempty"`
	Max  *string `json:"max,omitempty" yaml:"max,omitempty"`
	Step *string `json:"step,omitempty" yaml:"step,omitempty"`
}

func loadDriverConfig(ctx context.Context, client coreclientset.Interface, namespace, name, key string) (*DriverConfig, error) {
	configMap, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get configmap %s/%s: %w", namespace, name, err)
	}

	raw, ok := configMap.Data[key]
	if !ok {
		return nil, fmt.Errorf("configmap %s/%s is missing data key %q", namespace, name, key)
	}

	cfg := &DriverConfig{}
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		return nil, fmt.Errorf("parse configmap %s/%s key %q: %w", namespace, name, key, err)
	}
	return cfg, nil
}

func (c *DriverConfig) DevicesForNode(node *corev1.Node) ([]resourceapi.Device, error) {
	if node == nil {
		return nil, fmt.Errorf("node is required")
	}

	match, err := c.matchesNodeSelector(node, c.NodeSelector)
	if err != nil {
		return nil, err
	}
	if !match {
		return nil, nil
	}

	devicesByName := map[string]FakeDevice{}
	for _, group := range c.Groups {
		match, err := c.matchesNodeSelector(node, group.Selector)
		if err != nil {
			return nil, fmt.Errorf("match group %q selector: %w", group.Name, err)
		}
		if !match {
			continue
		}
		for _, device := range group.Devices {
			devicesByName[device.Name] = devicesByName[device.Name].Merge(device)
		}
	}

	if nodeConfig, ok := c.Nodes[node.Name]; ok {
		for _, device := range nodeConfig.Devices {
			devicesByName[device.Name] = devicesByName[device.Name].Merge(device)
		}
	}

	names := make([]string, 0, len(devicesByName))
	for name := range devicesByName {
		names = append(names, name)
	}
	sort.Strings(names)

	devices := make([]resourceapi.Device, 0, len(names))
	for _, name := range names {
		device, err := devicesByName[name].ToResourceDevice()
		if err != nil {
			return nil, fmt.Errorf("build device %q: %w", name, err)
		}
		devices = append(devices, device)
	}
	return devices, nil
}

func (c *DriverConfig) matchesNodeSelector(node *corev1.Node, selector *metav1.LabelSelector) (bool, error) {
	if selector == nil {
		return true, nil
	}

	parsedSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return false, err
	}
	return parsedSelector.Matches(labels.Set(nodeLabels(node))), nil
}

func (d FakeDevice) ToResourceDevice() (resourceapi.Device, error) {
	if d.Name == "" {
		return resourceapi.Device{}, fmt.Errorf("name is required")
	}

	var attrs map[resourceapi.QualifiedName]resourceapi.DeviceAttribute
	if len(d.Attributes) > 0 {
		attrs = make(map[resourceapi.QualifiedName]resourceapi.DeviceAttribute, len(d.Attributes))
		for name, value := range d.Attributes {
			attr, err := value.ToDeviceAttribute()
			if err != nil {
				return resourceapi.Device{}, fmt.Errorf("parse attribute %q: %w", name, err)
			}
			attrs[resourceapi.QualifiedName(name)] = attr
		}
	}

	var capacities map[resourceapi.QualifiedName]resourceapi.DeviceCapacity
	allowMultipleAllocations := d.AllowMultipleAllocations
	if len(d.Capacity) > 0 {
		capacities = make(map[resourceapi.QualifiedName]resourceapi.DeviceCapacity, len(d.Capacity))
		for name, value := range d.Capacity {
			capacity, err := value.ToDeviceCapacity()
			if err != nil {
				return resourceapi.Device{}, fmt.Errorf("parse capacity %q: %w", name, err)
			}
			if capacity.RequestPolicy != nil && allowMultipleAllocations == nil {
				autoEnable := true
				allowMultipleAllocations = &autoEnable
			}
			capacities[resourceapi.QualifiedName(name)] = capacity
		}
	}

	return resourceapi.Device{
		Name:                     d.Name,
		AllowMultipleAllocations: allowMultipleAllocations,
		Attributes:               attrs,
		Capacity:                 capacities,
	}, nil
}

func (d FakeDevice) Merge(override FakeDevice) FakeDevice {
	merged := d
	if override.Name != "" {
		merged.Name = override.Name
	}
	if override.AllowMultipleAllocations != nil {
		merged.AllowMultipleAllocations = override.AllowMultipleAllocations
	}
	merged.Attributes = mergeAttributeMaps(merged.Attributes, override.Attributes)
	merged.Capacity = mergeCapacityMaps(merged.Capacity, override.Capacity)
	return merged
}

func (v FakeAttributeValue) ToDeviceAttribute() (resourceapi.DeviceAttribute, error) {
	count := 0
	if v.Int != nil {
		count++
	}
	if v.Bool != nil {
		count++
	}
	if v.String != nil {
		count++
	}
	if v.Version != nil {
		count++
	}
	if count != 1 {
		return resourceapi.DeviceAttribute{}, fmt.Errorf("exactly one of int/bool/string/version must be set")
	}

	return resourceapi.DeviceAttribute{
		IntValue:     v.Int,
		BoolValue:    v.Bool,
		StringValue:  v.String,
		VersionValue: v.Version,
	}, nil
}

func (c FakeCapacity) ToDeviceCapacity() (resourceapi.DeviceCapacity, error) {
	quantity, err := resource.ParseQuantity(c.Value)
	if err != nil {
		return resourceapi.DeviceCapacity{}, fmt.Errorf("parse value %q: %w", c.Value, err)
	}

	result := resourceapi.DeviceCapacity{
		Value: quantity,
	}
	if c.RequestPolicy == nil {
		return result, nil
	}

	policy, err := c.RequestPolicy.ToCapacityRequestPolicy()
	if err != nil {
		return resourceapi.DeviceCapacity{}, err
	}
	result.RequestPolicy = policy
	return result, nil
}

func (p *FakeCapacityRequestPolicy) ToCapacityRequestPolicy() (*resourceapi.CapacityRequestPolicy, error) {
	if p == nil {
		return nil, nil
	}

	policy := &resourceapi.CapacityRequestPolicy{}
	if p.Default != nil {
		value, err := parseQuantityPointer(*p.Default)
		if err != nil {
			return nil, fmt.Errorf("parse default: %w", err)
		}
		policy.Default = value
	}

	if len(p.ValidValues) > 0 {
		values := make([]resource.Quantity, 0, len(p.ValidValues))
		for _, validValue := range p.ValidValues {
			value, err := resource.ParseQuantity(validValue)
			if err != nil {
				return nil, fmt.Errorf("parse validValues entry %q: %w", validValue, err)
			}
			values = append(values, value)
		}
		policy.ValidValues = values
	}

	if p.ValidRange != nil {
		validRange, err := p.ValidRange.ToCapacityPolicyRange()
		if err != nil {
			return nil, err
		}
		policy.ValidRange = validRange
	}

	return policy, nil
}

func (r *FakeCapacityPolicyRange) ToCapacityPolicyRange() (*resourceapi.CapacityRequestPolicyRange, error) {
	if r == nil {
		return nil, nil
	}

	min, err := parseOptionalQuantity(r.Min)
	if err != nil {
		return nil, fmt.Errorf("parse min: %w", err)
	}
	max, err := parseOptionalQuantity(r.Max)
	if err != nil {
		return nil, fmt.Errorf("parse max: %w", err)
	}
	step, err := parseOptionalQuantity(r.Step)
	if err != nil {
		return nil, fmt.Errorf("parse step: %w", err)
	}

	return &resourceapi.CapacityRequestPolicyRange{
		Min:  min,
		Max:  max,
		Step: step,
	}, nil
}

func parseOptionalQuantity(value *string) (*resource.Quantity, error) {
	if value == nil {
		return nil, nil
	}
	return parseQuantityPointer(*value)
}

func parseQuantityPointer(value string) (*resource.Quantity, error) {
	quantity, err := resource.ParseQuantity(value)
	if err != nil {
		return nil, err
	}
	return &quantity, nil
}

func mergeAttributeMaps(base, override map[string]FakeAttributeValue) map[string]FakeAttributeValue {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}

	merged := make(map[string]FakeAttributeValue, len(base)+len(override))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range override {
		merged[key] = value
	}
	return merged
}

func mergeCapacityMaps(base, override map[string]FakeCapacity) map[string]FakeCapacity {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}

	merged := make(map[string]FakeCapacity, len(base)+len(override))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range override {
		if existing, ok := merged[key]; ok {
			merged[key] = mergeCapacity(existing, value)
			continue
		}
		merged[key] = value
	}
	return merged
}

func mergeCapacity(base, override FakeCapacity) FakeCapacity {
	merged := base
	if override.Value != "" {
		merged.Value = override.Value
	}
	if override.RequestPolicy != nil {
		merged.RequestPolicy = override.RequestPolicy
	}
	return merged
}
