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
	"strings"

	"github.com/Project-HAMi/HAMi-DRA/pkg/constants"
)

type DeviceTypeTable struct {
	// DRA driver names, e.g. hami-core-gpu.project-hami.io and ascend.project-hami.io.
	NvidiaDriver string
	AscendDriver string
	// AscendCommonWords maps an Ascend chip name to its commonWord, e.g. "310P3" -> "Ascend310P".
	AscendCommonWords map[string]string
}

// DeviceTypes builds the table from the configmap, ignoring c.Vendors.
// Ascend chips from the configmap override built-in chips with the same name.
func (c *Config) DeviceTypes() DeviceTypeTable {
	if c == nil {
		c = &Config{}
	}
	table := DeviceTypeTable{
		NvidiaDriver:      firstNonEmpty(c.Nvidia.DraDriverName, constants.NvidiaDraDriver),
		AscendDriver:      firstNonEmpty(c.Ascend.DraDriverName, constants.AscendDraDriver),
		AscendCommonWords: make(map[string]string),
	}
	for _, chip := range append(DefaultAscendVNPUs(), c.Ascend.Devices...) {
		if chip.ChipName == "" || chip.CommonWord == "" {
			continue
		}
		table.AscendCommonWords[chip.ChipName] = chip.CommonWord
	}
	return table
}

// DeviceType maps a product name to its device type: "Tesla P4" -> "NVIDIA-Tesla P4"
// for NVIDIA, "310P3" -> "Ascend310P" for Ascend. Others currently keep their product name.
func (t DeviceTypeTable) DeviceType(driver, productName string) string {
	switch {
	case productName == "":
		return ""
	case driver == t.NvidiaDriver:
		if strings.HasPrefix(productName, NvidiaGPUDevice) {
			return productName
		}
		return NvidiaGPUDevice + "-" + productName
	case driver == t.AscendDriver:
		if word, ok := t.AscendCommonWords[productName]; ok {
			return word
		}
	}
	return productName
}
