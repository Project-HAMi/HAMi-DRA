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

package config

import (
	"fmt"
	"strings"

	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/Project-HAMi/HAMi-DRA/pkg/constants"
)

const (
	VendorNvidia = "nvidia"
	VendorHygon  = "hygon"
	VendorAscend = "ascend"
)

// DRADeviceConfig holds runtime settings for converting device-plugin resources to DRA claims.
type DRADeviceConfig struct {
	Vendor             string
	ResourceCountName  string
	ResourceMemoryName string
	ResourceCoreName   string
	DeviceClassName    string
	DraDriverName      string
	RequestName        string
	DeviceType         string
	// CommonWord is the HAMi chip key (e.g. Ascend310P). Empty for NVIDIA/Hygon.
	CommonWord string

	UseUUIDAnnotation   string
	NoUseUUIDAnnotation string
	UseTypeAnnotation   string
	NoUseTypeAnnotation string

	// ReferenceComputeUnits converts hygon.com/hcucores percentage to absolute cores when > 0.
	ReferenceComputeUnits int64
}

func (c *DRADeviceConfig) EffectiveDeviceClassName() string {
	if c != nil && c.DeviceClassName != "" {
		return c.DeviceClassName
	}
	return constants.NvidiaDraDriver
}

func (c *DRADeviceConfig) EffectiveDraDriverName() string {
	if c != nil && c.DraDriverName != "" {
		return c.DraDriverName
	}
	return constants.NvidiaDraDriver
}

func (c *DRADeviceConfig) ClaimNameSuffix() string {
	if c.CommonWord != "" {
		return strings.ToLower(c.CommonWord)
	}
	return c.Vendor
}

func (c *DRADeviceConfig) TypeSelectorExpression() string {
	driver := c.EffectiveDraDriverName()
	if c.selectorIncludesDriver() {
		return fmt.Sprintf(`device.driver == "%s" && device.attributes["%s"].type == "%s"`, driver, driver, c.DeviceType)
	}
	return fmt.Sprintf(`device.attributes["%s"].type == "%s"`, driver, c.DeviceType)
}

func (c *DRADeviceConfig) selectorIncludesDriver() bool {
	switch c.DeviceType {
	case constants.HygonDeviceType, constants.AscendHAMivNPUCoreDeviceType:
		return true
	default:
		return false
	}
}

// NewPrimaryDeviceRequest builds the HAMivNPUCore (or GPU/DCU) Exactly request.
// Mixed-cluster fallback should later set FirstAvailable with this Exact request
// as the first DeviceSubRequest and traditional vNPU DeviceClasses after it
// (feature gate DRAPrioritizedList). Keep capacity/config per subrequest; do not
// OR both device types in a single CEL selector.
func (c *DRADeviceConfig) NewPrimaryDeviceRequest() resourceapi.DeviceRequest {
	return resourceapi.DeviceRequest{
		Name: c.RequestName,
		Exactly: &resourceapi.ExactDeviceRequest{
			AllocationMode: resourceapi.DeviceAllocationModeExactCount,
			Capacity: &resourceapi.CapacityRequirements{
				Requests: make(map[resourceapi.QualifiedName]resource.Quantity),
			},
			DeviceClassName: c.EffectiveDeviceClassName(),
			Selectors: []resourceapi.DeviceSelector{
				{
					CEL: &resourceapi.CELDeviceSelector{
						Expression: c.TypeSelectorExpression(),
					},
				},
			},
		},
	}
}

func (c *DRADeviceConfig) ConvertMemory(memQty resource.Quantity) resource.Quantity {
	// HAMi device-plugin memory resources are expressed in MiB.
	return resource.MustParse(fmt.Sprintf("%d", memQty.Value()*1024*1024))
}

func (c *DRADeviceConfig) ConvertCores(coreQty resource.Quantity) (resource.Quantity, error) {
	if c.DeviceType == constants.HygonDeviceType && c.ReferenceComputeUnits <= 0 {
		return resource.Quantity{}, fmt.Errorf("referenceComputeUnits must be configured to convert hygon.com/hcucores requests")
	}
	if c.ReferenceComputeUnits > 0 {
		pct := coreQty.Value()
		absolute := (pct*c.ReferenceComputeUnits + 99) / 100
		if absolute < 1 {
			absolute = 1
		}
		return *resource.NewQuantity(absolute, resource.DecimalSI), nil
	}
	return coreQty, nil
}

func draDeviceFromNvidia(c *NvidiaConfig) *DRADeviceConfig {
	if c == nil {
		c = &NvidiaConfig{}
	}
	return &DRADeviceConfig{
		Vendor:                VendorNvidia,
		ResourceCountName:     firstNonEmpty(c.ResourceCountName, "nvidia.com/gpu"),
		ResourceMemoryName:    firstNonEmpty(c.ResourceMemoryName, "nvidia.com/gpumem"),
		ResourceCoreName:      firstNonEmpty(c.ResourceCoreName, "nvidia.com/gpucores"),
		DeviceClassName:       c.DeviceClassName,
		DraDriverName:         c.DraDriverName,
		RequestName:           "gpu",
		DeviceType:            constants.NvidiaDeviceType,
		UseUUIDAnnotation:     constants.UseUUIDAnnotation,
		NoUseUUIDAnnotation:   constants.NoUseUUIDAnnotation,
		UseTypeAnnotation:     constants.UseTypeAnnotation,
		NoUseTypeAnnotation:   constants.NoUseTypeAnnotation,
		ReferenceComputeUnits: 0,
	}
}

func draDeviceFromHygon(c *HygonConfig) *DRADeviceConfig {
	if c == nil {
		c = &HygonConfig{}
	}
	cfg := &DRADeviceConfig{
		Vendor:                VendorHygon,
		ResourceCountName:     firstNonEmpty(c.ResourceCountName, "hygon.com/hcunum"),
		ResourceMemoryName:    firstNonEmpty(c.ResourceMemoryName, "hygon.com/hcumem"),
		ResourceCoreName:      firstNonEmpty(c.ResourceCoreName, "hygon.com/hcucores"),
		DeviceClassName:       firstNonEmpty(c.DeviceClassName, constants.HygonDraDriver),
		DraDriverName:         firstNonEmpty(c.DraDriverName, constants.HygonDraDriver),
		RequestName:           firstNonEmpty(c.RequestName, "hcu"),
		DeviceType:            constants.HygonDeviceType,
		UseUUIDAnnotation:     firstNonEmpty(c.UseUUIDAnnotation, constants.HygonUseUUIDAnnotation),
		NoUseUUIDAnnotation:   firstNonEmpty(c.NoUseUUIDAnnotation, constants.HygonNoUseUUIDAnnotation),
		UseTypeAnnotation:     firstNonEmpty(c.UseTypeAnnotation, constants.HygonUseTypeAnnotation),
		NoUseTypeAnnotation:   firstNonEmpty(c.NoUseTypeAnnotation, constants.HygonNoUseTypeAnnotation),
		ReferenceComputeUnits: c.ReferenceComputeUnits,
	}
	return cfg
}

// DefaultAscendVNPUs matches HAMi charts/hami scheduler device-config vnpus.configs.
func DefaultAscendVNPUs() []AscendVNPUConfig {
	chips := []string{
		"Ascend910A",
		"Ascend910B2",
		"Ascend910B3",
		"Ascend910B4-1",
		"Ascend910B4",
		"Ascend310P",
		"Ascend910C",
	}
	chipName := map[string]string{
		"Ascend910A":    "910A",
		"Ascend910B2":   "910B2",
		"Ascend910B3":   "910B3",
		"Ascend910B4-1": "910B4-1",
		"Ascend910B4":   "910B4",
		"Ascend310P":    "310P3",
		"Ascend910C":    "Ascend910",
	}
	out := make([]AscendVNPUConfig, 0, len(chips))
	for _, word := range chips {
		out = append(out, AscendVNPUConfig{
			CommonWord:         word,
			ChipName:           chipName[word],
			ResourceName:       "huawei.com/" + word,
			ResourceMemoryName: "huawei.com/" + word + "-memory",
			ResourceCoreName:   "huawei.com/" + word + "-core",
		})
	}
	return out
}

func ascendUseUUIDAnnotation(commonWord string) string {
	return fmt.Sprintf("hami.io/use-%s-uuid", commonWord)
}

func ascendNoUseUUIDAnnotation(commonWord string) string {
	return fmt.Sprintf("hami.io/no-use-%s-uuid", commonWord)
}

func draDevicesFromAscend(c *AscendConfig) ([]*DRADeviceConfig, []int) {
	if c == nil {
		c = &AscendConfig{}
	}
	vnpus := c.Devices
	legacySingle := len(vnpus) == 0 && c.ResourceCountName != ""
	if legacySingle {
		word := strings.TrimPrefix(c.ResourceCountName, "huawei.com/")
		if word == c.ResourceCountName || word == "" {
			word = "Ascend310P"
		}
		vnpus = []AscendVNPUConfig{{
			CommonWord:         word,
			ResourceName:       c.ResourceCountName,
			ResourceMemoryName: firstNonEmpty(c.ResourceMemoryName, c.ResourceCountName+"-memory"),
			ResourceCoreName:   firstNonEmpty(c.ResourceCoreName, c.ResourceCountName+"-core"),
		}}
	}
	if len(vnpus) == 0 {
		vnpus = DefaultAscendVNPUs()
	}

	out := make([]*DRADeviceConfig, 0, len(vnpus))
	var emptyResourceNameIndexes []int
	for i, vnpu := range vnpus {
		if vnpu.ResourceName == "" {
			emptyResourceNameIndexes = append(emptyResourceNameIndexes, i)
			continue
		}
		word := vnpu.CommonWord
		if word == "" {
			word = strings.TrimPrefix(vnpu.ResourceName, "huawei.com/")
		}
		useUUID := ascendUseUUIDAnnotation(word)
		noUseUUID := ascendNoUseUUIDAnnotation(word)
		if legacySingle {
			useUUID = firstNonEmpty(c.UseUUIDAnnotation, useUUID)
			noUseUUID = firstNonEmpty(c.NoUseUUIDAnnotation, noUseUUID)
		}
		out = append(out, &DRADeviceConfig{
			Vendor:                VendorAscend,
			ResourceCountName:     vnpu.ResourceName,
			ResourceMemoryName:    vnpu.ResourceMemoryName,
			ResourceCoreName:      vnpu.ResourceCoreName,
			DeviceClassName:       firstNonEmpty(c.DeviceClassName, constants.AscendDeviceClassName),
			DraDriverName:         firstNonEmpty(c.DraDriverName, constants.AscendDraDriver),
			RequestName:           firstNonEmpty(c.RequestName, constants.AscendRequestName),
			DeviceType:            constants.AscendHAMivNPUCoreDeviceType,
			CommonWord:            word,
			UseUUIDAnnotation:     useUUID,
			NoUseUUIDAnnotation:   noUseUUID,
			UseTypeAnnotation:     firstNonEmpty(c.UseTypeAnnotation, constants.AscendUseTypeAnnotation),
			NoUseTypeAnnotation:   firstNonEmpty(c.NoUseTypeAnnotation, constants.AscendNoUseTypeAnnotation),
			ReferenceComputeUnits: 0,
		})
	}
	return out, emptyResourceNameIndexes
}

func (c *Config) DRADevices(vendors []string) ([]*DRADeviceConfig, error) {
	if c.LegacyVendor != "" {
		return nil, fmt.Errorf("config field vendor was removed; use vendors instead")
	}
	selected := vendors
	if len(selected) == 0 {
		selected = c.Vendors
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("at least one device vendor must be configured")
	}

	seenVendors := make(map[string]struct{}, len(selected))
	seenResources := make(map[string]string)
	seenSuffixes := make(map[string]string)
	var result []*DRADeviceConfig
	for _, vendor := range selected {
		if _, ok := seenVendors[vendor]; ok {
			return nil, fmt.Errorf("duplicate device vendor %q", vendor)
		}
		seenVendors[vendor] = struct{}{}

		var configs []*DRADeviceConfig
		switch vendor {
		case VendorNvidia:
			configs = []*DRADeviceConfig{draDeviceFromNvidia(&c.Nvidia)}
		case VendorHygon:
			configs = []*DRADeviceConfig{draDeviceFromHygon(&c.Hygon)}
		case VendorAscend:
			var emptyIndexes []int
			configs, emptyIndexes = draDevicesFromAscend(&c.Ascend)
			if len(configs) == 0 {
				if len(emptyIndexes) > 0 {
					return nil, fmt.Errorf("no ascend devices configured: empty resourceName at indexes %v", emptyIndexes)
				}
				return nil, fmt.Errorf("no ascend devices configured")
			}
		default:
			return nil, fmt.Errorf("unsupported device vendor %q", vendor)
		}

		for _, cfg := range configs {
			if cfg.ResourceCountName == "" {
				return nil, fmt.Errorf("device vendor %q has an empty count resource name", vendor)
			}
			for _, resourceName := range []string{cfg.ResourceCountName, cfg.ResourceMemoryName, cfg.ResourceCoreName} {
				if resourceName == "" {
					continue
				}
				if previous, ok := seenResources[resourceName]; ok {
					return nil, fmt.Errorf("resource name %q is configured by both %q and %q", resourceName, previous, vendor)
				}
				seenResources[resourceName] = vendor
			}
			suffix := cfg.ClaimNameSuffix()
			if previous, ok := seenSuffixes[suffix]; ok {
				return nil, fmt.Errorf("claim name suffix %q is configured by both %q and %q", suffix, previous, vendor)
			}
			seenSuffixes[suffix] = vendor
			result = append(result, cfg)
		}
	}
	return result, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
