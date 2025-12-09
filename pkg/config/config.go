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
	"gopkg.in/yaml.v3"
)

const (
	SchedulerDeviceConfigName = "hami-scheduler-device"
	DeviceConfigFileName      = "device-config.yaml"
)

const (
	HandshakeAnnos       = "hami.io/node-handshake"
	RegisterAnnos        = "hami.io/node-nvidia-register"
	RegisterGPUPairScore = "hami.io/node-nvidia-score"
	NvidiaGPUDevice      = "NVIDIA"
	NvidiaGPUCommonWord  = "GPU"
	GPUInUse             = "nvidia.com/use-gputype"
	GPUNoUse             = "nvidia.com/nouse-gputype"
	NumaBind             = "nvidia.com/numa-bind"
	NodeLockNvidia       = "hami.io/mutex.lock"
	// GPUUseUUID is user can use specify GPU device for set GPU UUID.
	GPUUseUUID = "nvidia.com/use-gpuuuid"
	// GPUNoUseUUID is user can not use specify GPU device for set GPU UUID.
	GPUNoUseUUID = "nvidia.com/nouse-gpuuuid"
	AllocateMode = "nvidia.com/vgpu-mode"

	MigMode      = "mig"
	HamiCoreMode = "hami-core"
	MpsMode      = "mps"
)

var (
	NodeName          string
	RuntimeSocketFlag string
	DisableCoreLimit  *bool

	// DevicePluginFilterDevice need device-plugin filter this device, don't register this device.
	DevicePluginFilterDevice *FilterDevice
)

type MigPartedSpec struct {
	Version    string                        `json:"version"               yaml:"version"`
	MigConfigs map[string]MigConfigSpecSlice `json:"mig-configs,omitempty" yaml:"mig-configs,omitempty"`
}

// MigConfigSpec defines the spec to declare the desired MIG configuration for a set of GPUs.
type MigConfigSpec struct {
	DeviceFilter any              `json:"device-filter,omitempty" yaml:"device-filter,flow,omitempty"`
	Devices      []int32          `json:"devices"                 yaml:"devices,flow"`
	MigEnabled   bool             `json:"mig-enabled"             yaml:"mig-enabled"`
	MigDevices   map[string]int32 `json:"mig-devices"             yaml:"mig-devices"`
}

// MigConfigSpecSlice represents a slice of 'MigConfigSpec'.
type MigConfigSpecSlice []MigConfigSpec

// GPUCoreUtilizationPolicy is set nvidia gpu core isolation policy.
type GPUCoreUtilizationPolicy string

const (
	DefaultCorePolicy GPUCoreUtilizationPolicy = "default"
	ForceCorePolicy   GPUCoreUtilizationPolicy = "force"
	DisableCorePolicy GPUCoreUtilizationPolicy = "disable"
)

type LibCudaLogLevel string

const (
	Error    LibCudaLogLevel = "0"
	Warnings LibCudaLogLevel = "1"
	Infos    LibCudaLogLevel = "3"
	Debugs   LibCudaLogLevel = "4"
)

type Config struct {
	Nvidia NvidiaConfig `yaml:"nvidia"`
}

type NvidiaConfig struct {
	// These configs are shared and can be overritten by Nodeconfig.
	NodeDefaultConfig            `yaml:",inline"`
	ResourceCountName            string `yaml:"resourceCountName"`
	ResourceMemoryName           string `yaml:"resourceMemoryName"`
	ResourceCoreName             string `yaml:"resourceCoreName"`
	ResourceMemoryPercentageName string `yaml:"resourceMemoryPercentageName"`
	ResourcePriority             string `yaml:"resourcePriorityName"`
	OverwriteEnv                 bool   `yaml:"overwriteEnv"`
	DefaultMemory                int32  `yaml:"defaultMemory"`
	DefaultCores                 int32  `yaml:"defaultCores"`
	DefaultGPUNum                int32  `yaml:"defaultGPUNum"`
	// TODO Whether these should be removed
	DisableCoreLimit  bool                   `yaml:"disableCoreLimit"`
	MigGeometriesList []AllowedMigGeometries `yaml:"knownMigGeometries"`
	// GPUCorePolicy through webhook automatic injected to container env
	GPUCorePolicy GPUCoreUtilizationPolicy `yaml:"gpuCorePolicy"`
	// RuntimeClassName is the name of the runtime class to be added to pod.spec.runtimeClassName
	RuntimeClassName string `yaml:"runtimeClassName"`
}

// These configs can be sepecified for each node by using Nodeconfig.
type NodeDefaultConfig struct {
	//nolint:tagalign
	DeviceSplitCount *uint `yaml:"deviceSplitCount" json:"devicesplitcount"`
	//nolint:tagalign
	DeviceMemoryScaling *float64 `yaml:"deviceMemoryScaling" json:"devicememoryscaling"`
	//nolint:tagalign
	DeviceCoreScaling *float64 `yaml:"deviceCoreScaling" json:"devicecorescaling"`
	// LogLevel is LIBCUDA_LOG_LEVEL value
	//nolint:tagalign
	LogLevel *LibCudaLogLevel `yaml:"libCudaLogLevel" json:"libcudaloglevel"`
}

type FilterDevice struct {
	// UUID is the device ID.
	UUID []string `json:"uuid"`
	// Index is the device index.
	Index []uint `json:"index"`
}

type DevicePluginConfigs struct {
	Nodeconfig []struct {
		// These configs is shared and will overrite those in NvidiaConfig.
		NodeDefaultConfig `json:",inline"`
		Name              string        `json:"name"`
		OperatingMode     string        `json:"operatingmode"`
		Migstrategy       string        `json:"migstrategy"`
		FilterDevice      *FilterDevice `json:"filterdevices"`
	} `json:"nodeconfig"`
}

type AllowedMigGeometries struct {
	Models     []string   `yaml:"models"`
	Geometries []Geometry `yaml:"allowedGeometries"`
}

type Geometry []MigTemplate

type MigTemplate struct {
	Name   string `yaml:"name"`
	Memory int32  `yaml:"memory"`
	Count  int32  `yaml:"count"`
}

func Unmarshal(data []byte) (*NvidiaConfig, error) {
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config.Nvidia, nil
}

func Marshal(nvidiaConfig *NvidiaConfig) ([]byte, error) {
	return yaml.Marshal(nvidiaConfig)
}
