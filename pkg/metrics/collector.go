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
	"strconv"

	"github.com/Project-HAMi/HAMi-DRA/pkg/cache"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"
)

// coreScale converts the 0-100 core accounting into the 0-1 fraction
// that a Prometheus _ratio suffix promises.
const coreScale = 100

type Collector struct {
	cache *cache.Cache
}

func NewCollector(cache *cache.Cache) *Collector {
	return &Collector{
		cache: cache,
	}
}

// Describe implements prometheus.Collector
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- nodevGPUMemoryLimitDesc
	ch <- nodevGPUCoreLimitDesc
	ch <- nodevGPUMemoryAllocatedDesc
	ch <- nodevGPUCoreAllocatedDesc
	ch <- podvGPUCoreAllocatedDesc
	ch <- podvGPUMemoryAllocatedDesc
}

// Collect implements prometheus.Collector
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	klog.V(5).Info("Collecting metrics")
	c.collectNodeMetrics(ch)
	c.collectPodMetrics(ch)
}

func (c *Collector) collectNodeMetrics(ch chan<- prometheus.Metric) {
	nodeNames := c.cache.NodeDevices.GetAllNodes()

	for _, nodeName := range nodeNames {
		devices := c.cache.NodeDevices.GetDevices(nodeName)
		if devices == nil {
			continue
		}

		for idx, device := range devices {
			deviceIdx := strconv.Itoa(idx)

			// hami_dra_gpu_memory_limit_bytes
			ch <- prometheus.MustNewConstMetric(
				nodevGPUMemoryLimitDesc,
				prometheus.GaugeValue,
				float64(device.MemoryTotal),
				nodeName, device.UUID, deviceIdx,
				device.Name, device.ProductName,
			)

			// hami_dra_gpu_core_limit_ratio
			ch <- prometheus.MustNewConstMetric(
				nodevGPUCoreLimitDesc,
				prometheus.GaugeValue,
				float64(device.CoresTotal)/coreScale,
				nodeName, device.UUID, deviceIdx,
				device.Name, device.ProductName,
			)

			// hami_dra_gpu_memory_allocated_bytes
			ch <- prometheus.MustNewConstMetric(
				nodevGPUMemoryAllocatedDesc,
				prometheus.GaugeValue,
				float64(device.MemoryUsed),
				nodeName, device.UUID, deviceIdx,
				device.Name, device.ProductName,
			)

			// hami_dra_gpu_core_allocated_ratio
			ch <- prometheus.MustNewConstMetric(
				nodevGPUCoreAllocatedDesc,
				prometheus.GaugeValue,
				float64(device.CoresUsed)/coreScale,
				nodeName, device.UUID, deviceIdx,
				device.Name, device.ProductName,
			)
		}
	}
	klog.V(5).Infof("Collected metrics for %d nodes", len(nodeNames))
}

func (c *Collector) collectPodMetrics(ch chan<- prometheus.Metric) {
	claims := c.cache.NodeDevices.GetAllClaims()

	for _, claim := range claims {
		devices := c.cache.NodeDevices.GetDevices(claim.NodeName)
		for _, result := range claim.AllocationResults {
			var device *cache.NodeDevice
			deviceIdx := "0"
			for idx, d := range devices {
				if d.Name == result.DeviceName {
					device = d
					deviceIdx = strconv.Itoa(idx)
					break
				}
			}
			if device == nil {
				klog.Warningf("Device %s not found on node %s", result.DeviceName, claim.NodeName)
				continue
			}

			for _, podName := range claim.UsedBy {
				ch <- prometheus.MustNewConstMetric(
					podvGPUCoreAllocatedDesc,
					prometheus.GaugeValue,
					float64(result.Cores)/coreScale,
					claim.NodeName,
					device.UUID,
					result.Namespace,
					podName,
				)
				ch <- prometheus.MustNewConstMetric(
					podvGPUMemoryAllocatedDesc,
					prometheus.GaugeValue,
					float64(result.Memory),
					claim.NodeName,
					device.UUID,
					result.Namespace,
					podName,
				)
			}
		}
	}
}
