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
	"github.com/Project-HAMi/HAMi-DRA/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"
)

// coreScale converts the 0-100 core accounting into the 0-1 fraction
// that a Prometheus _ratio suffix promises.
const coreScale = 100

type Collector struct {
	cache       *cache.Cache
	legacy      bool
	deviceTypes config.DeviceTypeTable
}

func NewCollector(cache *cache.Cache, legacy bool, deviceTypes config.DeviceTypeTable) *Collector {
	return &Collector{
		cache:       cache,
		legacy:      legacy,
		deviceTypes: deviceTypes,
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

	if !c.legacy {
		return
	}
	ch <- legacyNodevGPUMemoryLimitDesc
	ch <- legacyNodevGPUCoreLimitDesc
	ch <- legacyNodevGPUMemoryAllocatedDesc
	ch <- legacyNodevGPUCoreAllocatedDesc
	ch <- legacyPodvGPUCoreAllocatedDesc
	ch <- legacyPodvGPUMemoryAllocatedDesc
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
			deviceType := c.deviceTypes.DeviceType(device.Driver, device.ProductName)

			// hami_dra_gpu_memory_limit_bytes
			ch <- prometheus.MustNewConstMetric(
				nodevGPUMemoryLimitDesc,
				prometheus.GaugeValue,
				float64(device.MemoryTotal),
				nodeName, device.UUID, deviceIdx,
				device.Name, deviceType,
			)

			// hami_dra_gpu_core_limit_ratio
			ch <- prometheus.MustNewConstMetric(
				nodevGPUCoreLimitDesc,
				prometheus.GaugeValue,
				float64(device.CoresTotal)/coreScale,
				nodeName, device.UUID, deviceIdx,
				device.Name, deviceType,
			)

			// hami_dra_gpu_memory_allocated_bytes
			ch <- prometheus.MustNewConstMetric(
				nodevGPUMemoryAllocatedDesc,
				prometheus.GaugeValue,
				float64(device.MemoryUsed),
				nodeName, device.UUID, deviceIdx,
				device.Name, deviceType,
			)

			// hami_dra_gpu_core_allocated_ratio
			ch <- prometheus.MustNewConstMetric(
				nodevGPUCoreAllocatedDesc,
				prometheus.GaugeValue,
				float64(device.CoresUsed)/coreScale,
				nodeName, device.UUID, deviceIdx,
				device.Name, deviceType,
			)

			if !c.legacy {
				continue
			}

			// GPUDeviceMemoryLimit (deprecated)
			ch <- prometheus.MustNewConstMetric(
				legacyNodevGPUMemoryLimitDesc,
				prometheus.GaugeValue,
				float64(device.MemoryTotal)/1024/1024, // convert to MB
				nodeName, device.UUID, deviceIdx,
				device.Name, device.Brand, device.ProductName,
			)

			// GPUDeviceCoreLimit (deprecated)
			ch <- prometheus.MustNewConstMetric(
				legacyNodevGPUCoreLimitDesc,
				prometheus.GaugeValue,
				float64(device.CoresTotal),
				nodeName, device.UUID, deviceIdx,
				device.Name, device.Brand, device.ProductName,
			)

			// GPUDeviceMemoryAllocated (deprecated)
			ch <- prometheus.MustNewConstMetric(
				legacyNodevGPUMemoryAllocatedDesc,
				prometheus.GaugeValue,
				float64(device.MemoryUsed)/1024/1024, // convert to MB
				nodeName, device.UUID, deviceIdx,
				device.Name, device.Brand, device.ProductName,
			)

			// GPUDeviceCoreAllocated (deprecated)
			ch <- prometheus.MustNewConstMetric(
				legacyNodevGPUCoreAllocatedDesc,
				prometheus.GaugeValue,
				float64(device.CoresUsed),
				nodeName, device.UUID, deviceIdx,
				device.Name, device.Brand, device.ProductName,
			)
		}
	}
	klog.V(5).Infof("Collected metrics for %d nodes", len(nodeNames))
}

// podDevice is one pod's allocation on one device, summed over its claims.
type podDevice struct {
	node, namespace, pod string
	device               *cache.NodeDevice
	deviceIdx            string
	cores, memory        int64
}

type podDeviceKey struct {
	node, deviceName, namespace, pod string
}

func (c *Collector) collectPodMetrics(ch chan<- prometheus.Metric) {
	for _, pd := range c.podDevices() {
		if pd.device.UUID == "" {
			klog.Warningf("Device %s on node %s has no UUID, skipping hami_dra_vgpu_* metrics for pod %s/%s",
				pd.device.Name, pd.node, pd.namespace, pd.pod)
		} else {
			ch <- prometheus.MustNewConstMetric(
				podvGPUCoreAllocatedDesc,
				prometheus.GaugeValue,
				float64(pd.cores)/coreScale,
				pd.node,
				pd.device.UUID,
				pd.namespace,
				pd.pod,
			)
			ch <- prometheus.MustNewConstMetric(
				podvGPUMemoryAllocatedDesc,
				prometheus.GaugeValue,
				float64(pd.memory),
				pd.node,
				pd.device.UUID,
				pd.namespace,
				pd.pod,
			)
		}

		if !c.legacy {
			continue
		}

		ch <- prometheus.MustNewConstMetric(
			legacyPodvGPUCoreAllocatedDesc,
			prometheus.GaugeValue,
			float64(pd.cores),
			pd.node,
			pd.device.UUID,
			pd.deviceIdx,
			pd.device.Name,
			pd.device.Brand,
			pd.device.ProductName,
			pd.namespace,
			pd.pod,
		)
		ch <- prometheus.MustNewConstMetric(
			legacyPodvGPUMemoryAllocatedDesc,
			prometheus.GaugeValue,
			float64(pd.memory)/1024/1024,
			pd.node,
			pd.device.UUID,
			pd.deviceIdx,
			pd.device.Name,
			pd.device.Brand,
			pd.device.ProductName,
			pd.namespace,
			pd.pod,
		)
	}
}

// podDevices sums each pod's claims per device. A pod can hold several
// claims on one GPU, and a scrape fails if a series appears twice.
func (c *Collector) podDevices() map[podDeviceKey]*podDevice {
	sums := make(map[podDeviceKey]*podDevice)
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
				key := podDeviceKey{claim.NodeName, device.Name, result.Namespace, podName}
				pd, ok := sums[key]
				if !ok {
					pd = &podDevice{
						node:      claim.NodeName,
						namespace: result.Namespace,
						pod:       podName,
						device:    device,
						deviceIdx: deviceIdx,
					}
					sums[key] = pd
				}
				pd.cores += result.Cores
				pd.memory += result.Memory
			}
		}
	}
	return sums
}
