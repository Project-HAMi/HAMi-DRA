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

package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"k8s.io/klog/v2"

	"github.com/Project-HAMi/HAMi-DRA/pkg/config"
)

// loadDeviceTypeTable reads the device config mounted from the configmap.
// A missing file falls back to the built-in table.
func loadDeviceTypeTable(path string) (config.DeviceTypeTable, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		klog.Warningf("Device config file %s not found, using built-in defaults", path)
		return (&config.Config{}).DeviceTypes(), nil
	}
	if err != nil {
		return config.DeviceTypeTable{}, fmt.Errorf("read device config file %s: %w", path, err)
	}
	cfg, err := config.Unmarshal(data)
	if err != nil {
		return config.DeviceTypeTable{}, fmt.Errorf("parse device config file %s: %w", path, err)
	}
	return cfg.DeviceTypes(), nil
}
