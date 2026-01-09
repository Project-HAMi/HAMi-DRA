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
		"GPUDeviceMemoryLimit",
		"Device memory limit for a certain GPU",
		[]string{"nodeid", "deviceuuid", "deviceidx", "devicename", "devicebrand", "deviceproductname"}, nil,
	)
	nodevGPUCoreLimitDesc = prometheus.NewDesc(
		"GPUDeviceCoreLimit",
		"Device memory core limit for a certain GPU",
		[]string{"nodeid", "deviceuuid", "deviceidx", "devicename", "devicebrand", "deviceproductname"}, nil,
	)
	nodevGPUMemoryAllocatedDesc = prometheus.NewDesc(
		"GPUDeviceMemoryAllocated",
		"Device memory allocated for a certain GPU",
		[]string{"nodeid", "deviceuuid", "deviceidx", "devicename", "devicebrand", "deviceproductname"}, nil,
	)
	nodevGPUCoreAllocatedDesc = prometheus.NewDesc(
		"GPUDeviceCoreAllocated",
		"Device core allocated for a certain GPU",
		[]string{"nodeid", "deviceuuid", "deviceidx", "devicename", "devicebrand", "deviceproductname"}, nil,
	)
	podvGPUMemoryAllocatedDesc = prometheus.NewDesc(
		"vGPUDeviceMemoryAllocated",
		"vGPU Device memory allocated for a container",
		[]string{"nodeid", "deviceuuid", "deviceidx", "devicename", "devicebrand", "deviceproductname", "podnamespace", "podname"}, nil,
	)
	podvGPUCoreAllocatedDesc = prometheus.NewDesc(
		"vGPUDeviceCoreAllocated",
		"vGPU Device core allocated for a container",
		[]string{"nodeid", "deviceuuid", "deviceidx", "devicename", "devicebrand", "deviceproductname", "podnamespace", "podname"}, nil,
	)
)

func RegisterMetrics(collector *Collector) {
	prometheus.MustRegister(collector)
}
