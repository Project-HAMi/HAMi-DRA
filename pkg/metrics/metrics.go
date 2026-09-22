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

package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	nodevGPUMemoryLimitDesc = prometheus.NewDesc(
		"hami_dra_gpu_memory_limit_bytes",
		"Device memory limit for a certain GPU",
		[]string{"nodeid", "deviceuuid", "deviceidx", "devicename", "devicebrand", "deviceproductname"}, nil,
	)
	nodevGPUCoreLimitDesc = prometheus.NewDesc(
		"hami_dra_gpu_core_limit_ratio",
		"Device core limit for a certain GPU",
		[]string{"nodeid", "deviceuuid", "deviceidx", "devicename", "devicebrand", "deviceproductname"}, nil,
	)
	nodevGPUMemoryAllocatedDesc = prometheus.NewDesc(
		"hami_dra_gpu_memory_allocated_bytes",
		"Device memory allocated for a certain GPU",
		[]string{"nodeid", "deviceuuid", "deviceidx", "devicename", "devicebrand", "deviceproductname"}, nil,
	)
	nodevGPUCoreAllocatedDesc = prometheus.NewDesc(
		"hami_dra_gpu_core_allocated_ratio",
		"Device core allocated for a certain GPU",
		[]string{"nodeid", "deviceuuid", "deviceidx", "devicename", "devicebrand", "deviceproductname"}, nil,
	)
	podvGPUMemoryAllocatedDesc = prometheus.NewDesc(
		"hami_dra_vgpu_memory_allocated_bytes",
		"vGPU Device memory allocated for a container",
		[]string{"nodeid", "deviceuuid", "deviceidx", "devicename", "devicebrand", "deviceproductname", "podnamespace", "podname"}, nil,
	)
	podvGPUCoreAllocatedDesc = prometheus.NewDesc(
		"hami_dra_vgpu_core_allocated_ratio",
		"vGPU Device core allocated for a container",
		[]string{"nodeid", "deviceuuid", "deviceidx", "devicename", "devicebrand", "deviceproductname", "podnamespace", "podname"}, nil,
	)
)

// Legacy descriptors reproduce the pre-rename metrics exactly: old names,
// memory in MB and cores on a 0-100 scale. They are emitted only when
// --legacy-metrics is set.
var (
	legacyNodevGPUMemoryLimitDesc = prometheus.NewDesc(
		"GPUDeviceMemoryLimit",
		"Device memory limit for a certain GPU",
		[]string{"nodeid", "deviceuuid", "deviceidx", "devicename", "devicebrand", "deviceproductname"}, nil,
	)
	legacyNodevGPUCoreLimitDesc = prometheus.NewDesc(
		"GPUDeviceCoreLimit",
		"Device core limit for a certain GPU",
		[]string{"nodeid", "deviceuuid", "deviceidx", "devicename", "devicebrand", "deviceproductname"}, nil,
	)
	legacyNodevGPUMemoryAllocatedDesc = prometheus.NewDesc(
		"GPUDeviceMemoryAllocated",
		"Device memory allocated for a certain GPU",
		[]string{"nodeid", "deviceuuid", "deviceidx", "devicename", "devicebrand", "deviceproductname"}, nil,
	)
	legacyNodevGPUCoreAllocatedDesc = prometheus.NewDesc(
		"GPUDeviceCoreAllocated",
		"Device core allocated for a certain GPU",
		[]string{"nodeid", "deviceuuid", "deviceidx", "devicename", "devicebrand", "deviceproductname"}, nil,
	)
	legacyPodvGPUMemoryAllocatedDesc = prometheus.NewDesc(
		"vGPUDeviceMemoryAllocated",
		"vGPU Device memory allocated for a container",
		[]string{"nodeid", "deviceuuid", "deviceidx", "devicename", "devicebrand", "deviceproductname", "podnamespace", "podname"}, nil,
	)
	legacyPodvGPUCoreAllocatedDesc = prometheus.NewDesc(
		"vGPUDeviceCoreAllocated",
		"vGPU Device core allocated for a container",
		[]string{"nodeid", "deviceuuid", "deviceidx", "devicename", "devicebrand", "deviceproductname", "podnamespace", "podname"}, nil,
	)
)
