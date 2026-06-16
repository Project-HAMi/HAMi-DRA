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

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/stretchr/testify/assert"
)

func TestDRADeviceHygonDefaults(t *testing.T) {
	cfg, err := (&Config{}).DRADevice(VendorHygon)
	assert.NoError(t, err)
	assert.Equal(t, "hygon.com/dcunum", cfg.ResourceCountName)
	assert.Equal(t, "dra.hygon.com", cfg.EffectiveDeviceClassName())
	assert.Equal(t, "dcu", cfg.RequestName)
}

func TestConvertCoresWithReferenceComputeUnits(t *testing.T) {
	cfg, err := (&Config{
		Hygon: HygonConfig{ReferenceComputeUnits: 120},
	}).DRADevice(VendorHygon)
	assert.NoError(t, err)

	converted := cfg.ConvertCores(*resource.NewQuantity(60, resource.DecimalSI))
	assert.Equal(t, int64(72), converted.Value())
}

func TestConvertMemoryMiB(t *testing.T) {
	cfg, err := (&Config{}).DRADevice(VendorHygon)
	assert.NoError(t, err)

	converted := cfg.ConvertMemory(*resource.NewQuantity(2000, resource.DecimalSI))
	assert.Equal(t, int64(2000*1024*1024), converted.Value())
}
