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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Project-HAMi/HAMi-DRA/pkg/config"
)

func TestLoadDeviceTypeTableMissingFile(t *testing.T) {
	table, err := loadDeviceTypeTable(filepath.Join(t.TempDir(), "device-config.yaml"))

	require.NoError(t, err)
	assert.Equal(t, (&config.Config{}).DeviceTypes(), table)
}

func TestLoadDeviceTypeTableFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "device-config.yaml")
	data := `
nvidia:
  draDriverName: fake.dra.hami.io
ascend:
  devices:
    - commonWord: AscendTest
      chipName: test-chip
`
	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))

	table, err := loadDeviceTypeTable(path)

	require.NoError(t, err)
	assert.Equal(t, "fake.dra.hami.io", table.NvidiaDriver)
	assert.Equal(t, "AscendTest", table.AscendCommonWords["test-chip"])
	assert.Equal(t, "Ascend310P", table.AscendCommonWords["310P3"])
}

func TestLoadDeviceTypeTableInvalidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "device-config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("nvidia: [\n"), 0o600))

	_, err := loadDeviceTypeTable(path)

	assert.ErrorContains(t, err, "parse device config file")
}

func TestLoadDeviceTypeTableUnreadableFile(t *testing.T) {
	// A directory exists but cannot be read as a file.
	_, err := loadDeviceTypeTable(t.TempDir())

	assert.ErrorContains(t, err, "read device config file")
}
