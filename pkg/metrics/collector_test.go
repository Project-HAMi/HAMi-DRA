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

func newTestCollector(withDevice bool) *Collector {
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
	return NewCollector(c)
}

func TestCollect_NodeMetrics(t *testing.T) {
	want := `
# HELP hami_dra_gpu_memory_limit_bytes Device memory limit for a certain GPU
# TYPE hami_dra_gpu_memory_limit_bytes gauge
hami_dra_gpu_memory_limit_bytes{device_index="0",device_name="gpu0",device_type="V100",device_uuid="uuid-1",node="node1"} 16777216
# HELP hami_dra_gpu_core_limit_ratio Device core limit for a certain GPU
# TYPE hami_dra_gpu_core_limit_ratio gauge
hami_dra_gpu_core_limit_ratio{device_index="0",device_name="gpu0",device_type="V100",device_uuid="uuid-1",node="node1"} 1
# HELP hami_dra_gpu_memory_allocated_bytes Device memory allocated for a certain GPU
# TYPE hami_dra_gpu_memory_allocated_bytes gauge
hami_dra_gpu_memory_allocated_bytes{device_index="0",device_name="gpu0",device_type="V100",device_uuid="uuid-1",node="node1"} 8388608
# HELP hami_dra_gpu_core_allocated_ratio Device core allocated for a certain GPU
# TYPE hami_dra_gpu_core_allocated_ratio gauge
hami_dra_gpu_core_allocated_ratio{device_index="0",device_name="gpu0",device_type="V100",device_uuid="uuid-1",node="node1"} 0.5
`
	names := []string{
		"hami_dra_gpu_memory_limit_bytes",
		"hami_dra_gpu_core_limit_ratio",
		"hami_dra_gpu_memory_allocated_bytes",
		"hami_dra_gpu_core_allocated_ratio",
	}
	if err := testutil.CollectAndCompare(newTestCollector(true), strings.NewReader(want), names...); err != nil {
		t.Errorf("node metrics mismatch: %v", err)
	}
}

func TestCollect_PodMetrics(t *testing.T) {
	want := `
# HELP hami_dra_vgpu_memory_allocated_bytes vGPU Device memory allocated for a container
# TYPE hami_dra_vgpu_memory_allocated_bytes gauge
hami_dra_vgpu_memory_allocated_bytes{device_uuid="uuid-1",namespace="default",node="node1",pod="pod1"} 8388608
# HELP hami_dra_vgpu_core_allocated_ratio vGPU Device core allocated for a container
# TYPE hami_dra_vgpu_core_allocated_ratio gauge
hami_dra_vgpu_core_allocated_ratio{device_uuid="uuid-1",namespace="default",node="node1",pod="pod1"} 0.5
`
	names := []string{
		"hami_dra_vgpu_memory_allocated_bytes",
		"hami_dra_vgpu_core_allocated_ratio",
	}
	if err := testutil.CollectAndCompare(newTestCollector(true), strings.NewReader(want), names...); err != nil {
		t.Errorf("pod metrics mismatch: %v", err)
	}
}

func TestCollect_MissingDevice(t *testing.T) {
	// device not in cache, pod metrics are skipped
	if got := testutil.CollectAndCount(newTestCollector(false)); got != 0 {
		t.Errorf("expected 0 metrics, got %d", got)
	}
}
