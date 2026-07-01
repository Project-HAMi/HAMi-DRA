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

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/Project-HAMi/HAMi-DRA/pkg/constants"
)

const (
	VendorNvidia = "nvidia"
	VendorHygon  = "hygon"
)

// DRADeviceConfig holds runtime settings for converting device-plugin resources to DRA claims.
type DRADeviceConfig struct {
	ResourceCountName  string
	ResourceMemoryName string
	ResourceCoreName   string
	DeviceClassName    string
	DraDriverName      string
	RequestName        string
	DeviceType         string

	UseUUIDAnnotation   string
	NoUseUUIDAnnotation string
	UseTypeAnnotation   string
	NoUseTypeAnnotation string

	// ReferenceComputeUnits converts hygon.com/dcucores percentage to absolute cores when > 0.
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

func (c *DRADeviceConfig) TypeSelectorExpression() string {
	driver := c.EffectiveDraDriverName()
	if c.DeviceType == constants.HygonDeviceType {
		return fmt.Sprintf(`device.driver == "%s" && device.attributes["%s"].type == "%s"`, driver, driver, c.DeviceType)
	}
	return fmt.Sprintf(`device.attributes["%s"].type == "%s"`, driver, c.DeviceType)
}

func (c *DRADeviceConfig) ConvertMemory(memQty resource.Quantity) resource.Quantity {
	// HAMi device-plugin memory resources are expressed in MiB.
	return resource.MustParse(fmt.Sprintf("%d", memQty.Value()*1024*1024))
}

func (c *DRADeviceConfig) ConvertCores(coreQty resource.Quantity) resource.Quantity {
	if c.ReferenceComputeUnits > 0 {
		pct := coreQty.Value()
		absolute := pct * c.ReferenceComputeUnits / 100
		if absolute < 1 {
			absolute = 1
		}
		return *resource.NewQuantity(absolute, resource.DecimalSI)
	}
	return coreQty
}

func draDeviceFromNvidia(c *NvidiaConfig) *DRADeviceConfig {
	if c == nil {
		c = &NvidiaConfig{}
	}
	return &DRADeviceConfig{
		ResourceCountName:     c.ResourceCountName,
		ResourceMemoryName:    c.ResourceMemoryName,
		ResourceCoreName:      c.ResourceCoreName,
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
		ResourceCountName:     firstNonEmpty(c.ResourceCountName, "hygon.com/dcunum"),
		ResourceMemoryName:    firstNonEmpty(c.ResourceMemoryName, "hygon.com/dcumem"),
		ResourceCoreName:      firstNonEmpty(c.ResourceCoreName, "hygon.com/dcucores"),
		DeviceClassName:       firstNonEmpty(c.DeviceClassName, constants.HygonDraDriver),
		DraDriverName:         firstNonEmpty(c.DraDriverName, constants.HygonDraDriver),
		RequestName:           firstNonEmpty(c.RequestName, "dcu"),
		DeviceType:            constants.HygonDeviceType,
		UseUUIDAnnotation:     firstNonEmpty(c.UseUUIDAnnotation, constants.HygonUseUUIDAnnotation),
		NoUseUUIDAnnotation:   firstNonEmpty(c.NoUseUUIDAnnotation, constants.HygonNoUseUUIDAnnotation),
		UseTypeAnnotation:     firstNonEmpty(c.UseTypeAnnotation, constants.HygonUseTypeAnnotation),
		NoUseTypeAnnotation:   firstNonEmpty(c.NoUseTypeAnnotation, constants.HygonNoUseTypeAnnotation),
		ReferenceComputeUnits: c.ReferenceComputeUnits,
	}
	return cfg
}

func (c *Config) DRADevice(vendor string) (*DRADeviceConfig, error) {
	selected := vendor
	if selected == "" {
		selected = c.Vendor
	}
	switch selected {
	case "", VendorNvidia:
		return draDeviceFromNvidia(&c.Nvidia), nil
	case VendorHygon:
		return draDeviceFromHygon(&c.Hygon), nil
	default:
		return nil, fmt.Errorf("unsupported device vendor %q", selected)
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
