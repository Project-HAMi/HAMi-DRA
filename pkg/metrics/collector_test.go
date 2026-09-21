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

import (
	"strings"
	"testing"

	"github.com/Project-HAMi/HAMi-DRA/pkg/cache"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"k8s.io/client-go/kubernetes/fake"
)

func newTestCollector(withDevice, legacy bool) *Collector {
	c := cache.NewCacheWithClient(fake.NewSimpleClientset())
	if withDevice {
		c.NodeDevices.Nodes["node1"] = &cache.NodeDeviceInfo{
			Devices: []*cache.NodeDevice{
				{
					Name:        "gpu0",
					UUID:        "uuid-1",
					Brand:       "NVIDIA",
					ProductName: "V100",
					CoresTotal:  100,
					CoresUsed:   50,
					MemoryTotal: 16777216,
					MemoryUsed:  8388608,
				},
			},
		}
	}
	c.NodeDevices.Claims.Claims["default/claim1"] = &cache.DeviceAllocation{
		NodeName: "node1",
		UsedBy:   []string{"pod1"},
		AllocationResults: []*cache.AllocationResult{
			{Namespace: "default", DeviceName: "gpu0", Cores: 50, Memory: 8388608},
		},
	}
	return NewCollector(c, legacy)
}

func TestCollect_NodeMetrics(t *testing.T) {
	want := `
# HELP hami_dra_gpu_memory_limit_bytes Device memory limit for a certain GPU
# TYPE hami_dra_gpu_memory_limit_bytes gauge
hami_dra_gpu_memory_limit_bytes{devicebrand="NVIDIA",deviceidx="0",devicename="gpu0",deviceproductname="V100",deviceuuid="uuid-1",nodeid="node1"} 16777216
# HELP hami_dra_gpu_core_limit_ratio Device core limit for a certain GPU
# TYPE hami_dra_gpu_core_limit_ratio gauge
hami_dra_gpu_core_limit_ratio{devicebrand="NVIDIA",deviceidx="0",devicename="gpu0",deviceproductname="V100",deviceuuid="uuid-1",nodeid="node1"} 1
# HELP hami_dra_gpu_memory_allocated_bytes Device memory allocated for a certain GPU
# TYPE hami_dra_gpu_memory_allocated_bytes gauge
hami_dra_gpu_memory_allocated_bytes{devicebrand="NVIDIA",deviceidx="0",devicename="gpu0",deviceproductname="V100",deviceuuid="uuid-1",nodeid="node1"} 8388608
# HELP hami_dra_gpu_core_allocated_ratio Device core allocated for a certain GPU
# TYPE hami_dra_gpu_core_allocated_ratio gauge
hami_dra_gpu_core_allocated_ratio{devicebrand="NVIDIA",deviceidx="0",devicename="gpu0",deviceproductname="V100",deviceuuid="uuid-1",nodeid="node1"} 0.5
`
	names := []string{
		"hami_dra_gpu_memory_limit_bytes",
		"hami_dra_gpu_core_limit_ratio",
		"hami_dra_gpu_memory_allocated_bytes",
		"hami_dra_gpu_core_allocated_ratio",
	}
	if err := testutil.CollectAndCompare(newTestCollector(true, false), strings.NewReader(want), names...); err != nil {
		t.Errorf("node metrics mismatch: %v", err)
	}
}

func TestCollect_PodMetrics(t *testing.T) {
	want := `
# HELP hami_dra_vgpu_memory_allocated_bytes vGPU Device memory allocated for a container
# TYPE hami_dra_vgpu_memory_allocated_bytes gauge
hami_dra_vgpu_memory_allocated_bytes{devicebrand="NVIDIA",deviceidx="0",devicename="gpu0",deviceproductname="V100",deviceuuid="uuid-1",nodeid="node1",podname="pod1",podnamespace="default"} 8388608
# HELP hami_dra_vgpu_core_allocated_ratio vGPU Device core allocated for a container
# TYPE hami_dra_vgpu_core_allocated_ratio gauge
hami_dra_vgpu_core_allocated_ratio{devicebrand="NVIDIA",deviceidx="0",devicename="gpu0",deviceproductname="V100",deviceuuid="uuid-1",nodeid="node1",podname="pod1",podnamespace="default"} 0.5
`
	names := []string{
		"hami_dra_vgpu_memory_allocated_bytes",
		"hami_dra_vgpu_core_allocated_ratio",
	}
	if err := testutil.CollectAndCompare(newTestCollector(true, false), strings.NewReader(want), names...); err != nil {
		t.Errorf("pod metrics mismatch: %v", err)
	}
}

func TestCollect_MissingDevice(t *testing.T) {
	// device not in cache, pod metrics are skipped
	if got := testutil.CollectAndCount(newTestCollector(false, false)); got != 0 {
		t.Errorf("expected 0 metrics, got %d", got)
	}
}

func TestCollect_LegacyDisabled(t *testing.T) {
	col := newTestCollector(true, false)
	for _, name := range []string{
		"GPUDeviceMemoryLimit",
		"GPUDeviceCoreLimit",
		"GPUDeviceMemoryAllocated",
		"GPUDeviceCoreAllocated",
		"vGPUDeviceMemoryAllocated",
		"vGPUDeviceCoreAllocated",
	} {
		if got := testutil.CollectAndCount(col, name); got != 0 {
			t.Errorf("legacy metric %s emitted while the flag is off, got %d", name, got)
		}
	}
}

func TestCollect_LegacyEnabled(t *testing.T) {
	want := `
# HELP GPUDeviceMemoryLimit Device memory limit for a certain GPU
# TYPE GPUDeviceMemoryLimit gauge
GPUDeviceMemoryLimit{devicebrand="NVIDIA",deviceidx="0",devicename="gpu0",deviceproductname="V100",deviceuuid="uuid-1",nodeid="node1"} 16
# HELP GPUDeviceCoreLimit Device core limit for a certain GPU
# TYPE GPUDeviceCoreLimit gauge
GPUDeviceCoreLimit{devicebrand="NVIDIA",deviceidx="0",devicename="gpu0",deviceproductname="V100",deviceuuid="uuid-1",nodeid="node1"} 100
# HELP GPUDeviceMemoryAllocated Device memory allocated for a certain GPU
# TYPE GPUDeviceMemoryAllocated gauge
GPUDeviceMemoryAllocated{devicebrand="NVIDIA",deviceidx="0",devicename="gpu0",deviceproductname="V100",deviceuuid="uuid-1",nodeid="node1"} 8
# HELP GPUDeviceCoreAllocated Device core allocated for a certain GPU
# TYPE GPUDeviceCoreAllocated gauge
GPUDeviceCoreAllocated{devicebrand="NVIDIA",deviceidx="0",devicename="gpu0",deviceproductname="V100",deviceuuid="uuid-1",nodeid="node1"} 50
# HELP vGPUDeviceMemoryAllocated vGPU Device memory allocated for a container
# TYPE vGPUDeviceMemoryAllocated gauge
vGPUDeviceMemoryAllocated{devicebrand="NVIDIA",deviceidx="0",devicename="gpu0",deviceproductname="V100",deviceuuid="uuid-1",nodeid="node1",podname="pod1",podnamespace="default"} 8
# HELP vGPUDeviceCoreAllocated vGPU Device core allocated for a container
# TYPE vGPUDeviceCoreAllocated gauge
vGPUDeviceCoreAllocated{devicebrand="NVIDIA",deviceidx="0",devicename="gpu0",deviceproductname="V100",deviceuuid="uuid-1",nodeid="node1",podname="pod1",podnamespace="default"} 50
`
	names := []string{
		"GPUDeviceMemoryLimit",
		"GPUDeviceCoreLimit",
		"GPUDeviceMemoryAllocated",
		"GPUDeviceCoreAllocated",
		"vGPUDeviceMemoryAllocated",
		"vGPUDeviceCoreAllocated",
	}
	col := newTestCollector(true, true)
	if err := testutil.CollectAndCompare(col, strings.NewReader(want), names...); err != nil {
		t.Errorf("legacy metrics mismatch: %v", err)
	}
	// additive, not a toggle: the new metrics are still emitted
	if got := testutil.CollectAndCount(col, "hami_dra_gpu_memory_limit_bytes"); got != 1 {
		t.Errorf("expected the new metric alongside the legacy ones, got %d", got)
	}
}
