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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/Project-HAMi/HAMi-DRA/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDRADeviceHygonDefaults(t *testing.T) {
	cfg, err := (&Config{}).DRADevice(VendorHygon)
	assert.NoError(t, err)
	assert.Equal(t, "hygon.com/hcunum", cfg.ResourceCountName)
	assert.Equal(t, "dra.hygon.com", cfg.EffectiveDeviceClassName())
	assert.Equal(t, "hcu", cfg.RequestName)
}

func TestConvertCoresWithReferenceComputeUnits(t *testing.T) {
	cfg, err := (&Config{
		Hygon: HygonConfig{ReferenceComputeUnits: 120},
	}).DRADevice(VendorHygon)
	assert.NoError(t, err)

	converted, err := cfg.ConvertCores(*resource.NewQuantity(60, resource.DecimalSI))
	assert.NoError(t, err)
	assert.Equal(t, int64(72), converted.Value())

	rounded, err := cfg.ConvertCores(*resource.NewQuantity(1, resource.DecimalSI))
	assert.NoError(t, err)
	assert.Equal(t, int64(2), rounded.Value())
}

func TestConvertCoresHygonRequiresReferenceComputeUnits(t *testing.T) {
	cfg, err := (&Config{}).DRADevice(VendorHygon)
	assert.NoError(t, err)

	_, err = cfg.ConvertCores(*resource.NewQuantity(50, resource.DecimalSI))
	assert.Error(t, err)
}

func TestConvertMemoryMiB(t *testing.T) {
	cfg, err := (&Config{}).DRADevice(VendorHygon)
	assert.NoError(t, err)

	converted := cfg.ConvertMemory(*resource.NewQuantity(2000, resource.DecimalSI))
	assert.Equal(t, int64(2000*1024*1024), converted.Value())
}

func TestDRADeviceAscendDefaults(t *testing.T) {
	cfgs, err := (&Config{}).DRADevices(VendorAscend)
	assert.NoError(t, err)
	assert.Len(t, cfgs, 7)

	words := make([]string, 0, len(cfgs))
	var cfg310P *DRADeviceConfig
	for _, cfg := range cfgs {
		words = append(words, cfg.CommonWord)
		if cfg.CommonWord == "Ascend310P" {
			cfg310P = cfg
		}
	}
	assert.Equal(t, []string{
		"Ascend910A", "Ascend910B2", "Ascend910B3", "Ascend910B4-1",
		"Ascend910B4", "Ascend310P", "Ascend910C",
	}, words)

	require.NotNil(t, cfg310P)
	assert.Equal(t, "huawei.com/Ascend310P", cfg310P.ResourceCountName)
	assert.Equal(t, "huawei.com/Ascend310P-memory", cfg310P.ResourceMemoryName)
	assert.Equal(t, "huawei.com/Ascend310P-core", cfg310P.ResourceCoreName)
	assert.Equal(t, "hami.io/use-Ascend310P-uuid", cfg310P.UseUUIDAnnotation)
	assert.Equal(t, constants.AscendDeviceClassName, cfg310P.EffectiveDeviceClassName())
	assert.Equal(t, constants.AscendDraDriver, cfg310P.EffectiveDraDriverName())
	assert.Equal(t, constants.AscendRequestName, cfg310P.RequestName)
	assert.Equal(t, constants.AscendHAMivNPUCoreDeviceType, cfg310P.DeviceType)
	assert.Empty(t, cfg310P.RuntimeClassName)
	assert.Equal(t,
		`device.driver == "ascend.project-hami.io" && device.attributes["ascend.project-hami.io"].type == "HAMivNPUCore"`,
		cfg310P.TypeSelectorExpression(),
	)

	req := cfg310P.NewPrimaryDeviceRequest()
	assert.Equal(t, "npu", req.Name)
	require.NotNil(t, req.Exactly)
	assert.Nil(t, req.FirstAvailable)
	assert.Equal(t, constants.AscendDeviceClassName, req.Exactly.DeviceClassName)
}

func TestDRADeviceAscendLegacySingleChip(t *testing.T) {
	cfg, err := (&Config{Ascend: AscendConfig{
		ResourceCountName:  "huawei.com/Ascend910B3",
		ResourceMemoryName: "huawei.com/Ascend910B3-memory",
		ResourceCoreName:   "huawei.com/Ascend910B3-core",
	}}).DRADevice(VendorAscend)
	assert.NoError(t, err)
	assert.Equal(t, "Ascend910B3", cfg.CommonWord)
	assert.Equal(t, "huawei.com/Ascend910B3", cfg.ResourceCountName)
	assert.Equal(t, "hami.io/use-Ascend910B3-uuid", cfg.UseUUIDAnnotation)
}

func TestApplyRuntimeClass(t *testing.T) {
	cfg, err := (&Config{}).DRADevice(VendorAscend)
	require.NoError(t, err)

	spec := &corev1.PodSpec{}
	cfg.ApplyRuntimeClass(spec)
	assert.Nil(t, spec.RuntimeClassName)

	cfg.RuntimeClassName = "ascend"
	cfg.ApplyRuntimeClass(spec)
	require.NotNil(t, spec.RuntimeClassName)
	assert.Equal(t, "ascend", *spec.RuntimeClassName)

	existing := "custom"
	spec.RuntimeClassName = &existing
	cfg.ApplyRuntimeClass(spec)
	assert.Equal(t, "custom", *spec.RuntimeClassName)
}

func TestDRADeviceAscendEmptyResourceNameIndexes(t *testing.T) {
	_, err := (&Config{Ascend: AscendConfig{
		Devices: []AscendVNPUConfig{
			{CommonWord: "bad-key"},
			{CommonWord: "also-bad"},
		},
	}}).DRADevices(VendorAscend)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty resourceName at indexes [0 1]")
}
