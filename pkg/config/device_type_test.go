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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeviceTypesDefaults(t *testing.T) {
	table := (&Config{}).DeviceTypes()

	assert.Equal(t, "hami-core-gpu.project-hami.io", table.NvidiaDriver)
	assert.Equal(t, "ascend.project-hami.io", table.AscendDriver)
	assert.Equal(t, map[string]string{
		"910A":      "Ascend910A",
		"910B2":     "Ascend910B2",
		"910B3":     "Ascend910B3",
		"910B4-1":   "Ascend910B4-1",
		"910B4":     "Ascend910B4",
		"310P3":     "Ascend310P",
		"Ascend910": "Ascend910C",
	}, table.AscendCommonWords)
}

func TestDeviceTypesFakeDriver(t *testing.T) {
	// The chart writes this name into nvidia.draDriverName when the fake
	// driver replaces the NVIDIA one.
	table := (&Config{Nvidia: NvidiaConfig{DraDriverName: "fake.dra.hami.io"}}).DeviceTypes()

	assert.Equal(t, "fake.dra.hami.io", table.NvidiaDriver)
}

func TestDeviceTypesAddsConfiguredChip(t *testing.T) {
	table := (&Config{Ascend: AscendConfig{Devices: []AscendVNPUConfig{
		{ChipName: "test-chip", CommonWord: "AscendTest"},
	}}}).DeviceTypes()

	assert.Equal(t, "AscendTest", table.AscendCommonWords["test-chip"])
	assert.Equal(t, "Ascend310P", table.AscendCommonWords["310P3"])
}

func TestDeviceTypesConfiguredChipOverridesBuiltIn(t *testing.T) {
	table := (&Config{Ascend: AscendConfig{Devices: []AscendVNPUConfig{
		{ChipName: "310P3", CommonWord: "AscendTest"},
	}}}).DeviceTypes()

	assert.Equal(t, "AscendTest", table.AscendCommonWords["310P3"])
}

func TestDeviceTypesSkipsIncompleteChips(t *testing.T) {
	table := (&Config{Ascend: AscendConfig{Devices: []AscendVNPUConfig{
		// Rendered by the chart when ascendDevices is empty: no chipName.
		{CommonWord: "Ascend310P", ResourceName: "huawei.com/Ascend310P"},
		// No commonWord: the chip keeps its own name.
		{ChipName: "910B3", ResourceName: "huawei.com/Ascend910B3"},
	}}}).DeviceTypes()

	assert.NotContains(t, table.AscendCommonWords, "")
	assert.Equal(t, "Ascend310P", table.AscendCommonWords["310P3"])
	assert.Equal(t, "Ascend910B3", table.AscendCommonWords["910B3"])
}

func TestDeviceTypeTableDeviceType(t *testing.T) {
	table := (&Config{}).DeviceTypes()
	tests := []struct {
		name        string
		driver      string
		productName string
		want        string
	}{
		{name: "nvidia without prefix", driver: "hami-core-gpu.project-hami.io", productName: "Tesla P4", want: "NVIDIA-Tesla P4"},
		{name: "nvidia with prefix", driver: "hami-core-gpu.project-hami.io", productName: "NVIDIA A30", want: "NVIDIA A30"},
		{name: "ascend known chip", driver: "ascend.project-hami.io", productName: "310P3", want: "Ascend310P"},
		{name: "ascend unknown chip", driver: "ascend.project-hami.io", productName: "test-chip", want: "test-chip"},
		{name: "other vendor", driver: "dra.hygon.com", productName: "K100_AI", want: "K100_AI"},
		{name: "no product name", driver: "hami-core-gpu.project-hami.io", productName: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, table.DeviceType(tt.driver, tt.productName))
		})
	}
}
