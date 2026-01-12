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

			// GPUDeviceMemoryLimit
			ch <- prometheus.MustNewConstMetric(
				nodevGPUMemoryLimitDesc,
				prometheus.GaugeValue,
				float64(device.MemoryTotal)/1024/1024, // convert to MB
				nodeName, device.UUID, deviceIdx,
				device.Name, device.Brand, device.ProductName,
			)

			// GPUDeviceCoreLimit
			ch <- prometheus.MustNewConstMetric(
				nodevGPUCoreLimitDesc,
				prometheus.GaugeValue,
				float64(device.CoresTotal),
				nodeName, device.UUID, deviceIdx,
				device.Name, device.Brand, device.ProductName,
			)

			// GPUDeviceMemoryAllocated
			ch <- prometheus.MustNewConstMetric(
				nodevGPUMemoryAllocatedDesc,
				prometheus.GaugeValue,
				float64(device.MemoryUsed)/1024/1024, // convert to MB
				nodeName, device.UUID, deviceIdx,
				device.Name, device.Brand, device.ProductName,
			)

			// GPUDeviceCoreAllocated
			ch <- prometheus.MustNewConstMetric(
				nodevGPUCoreAllocatedDesc,
				prometheus.GaugeValue,
				float64(device.CoresUsed),
				nodeName, device.UUID, deviceIdx,
				device.Name, device.Brand, device.ProductName,
			)
		}
	}
	klog.V(5).Infof("Collected metrics for %d nodes", len(nodeNames))
}

func (c *Collector) collectPodMetrics(ch chan<- prometheus.Metric) {
	claims := c.cache.NodeDevices.GetAllClaims()

	for _, claim := range claims {
		devices := c.cache.NodeDevices.GetDevices(claim.NodeName)
		var device *cache.NodeDevice
		for _, result := range claim.AllocationResults {
			for _, d := range devices {
				if d.Name == result.DeviceName {
					device = d
					break
				}
			}

			deviceName := ""
			deviceBrand := ""
			deviceProductName := ""
			deviceIdx := "0"
			if device != nil {
				deviceName = device.Name
				deviceBrand = device.Brand
				deviceProductName = device.ProductName
				for idx, d := range devices {
					if d.UUID == result.DeviceName {
						deviceIdx = strconv.Itoa(idx)
						break
					}
				}
			} else {
				// should not happen
				klog.Warningf("Device %s not found for claim %s", result.DeviceName, claim.NodeName)
				continue
			}

			for _, podName := range claim.UsedBy {
				ch <- prometheus.MustNewConstMetric(
					podvGPUCoreAllocatedDesc,
					prometheus.GaugeValue,
					float64(result.Cores),
					claim.NodeName,
					device.UUID,
					deviceIdx,
					deviceName,
					deviceBrand,
					deviceProductName,
					result.Namespace,
					podName,
				)
				ch <- prometheus.MustNewConstMetric(
					podvGPUMemoryAllocatedDesc,
					prometheus.GaugeValue,
					float64(result.Memory)/1024/1024, // convert to MB
					claim.NodeName,
					device.UUID,
					deviceIdx,
					deviceName,
					deviceBrand,
					deviceProductName,
					result.Namespace,
					podName,
				)
			}
		}
	}
}
