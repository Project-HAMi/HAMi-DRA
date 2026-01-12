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

package cache

import (
	"fmt"
	"maps"
	"sync"

	"github.com/Project-HAMi/HAMi-DRA/pkg/constants"
	v1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/klog/v2"
)

type NodeDevices struct {
	Nodes  map[string]*NodeDeviceInfo
	mu     sync.RWMutex
	Claims *ClaimsCache
}

type NodeDeviceInfo struct {
	mu      sync.RWMutex
	Devices []*NodeDevice
}

type ClaimsCache struct {
	Mux    sync.RWMutex
	Claims map[string]*DeviceAllocation // key: claim key (namespace/name)
}

type NodeDevice struct {
	Name         string
	UUID         string
	Architecture string
	Brand        string
	ProductName  string
	CoresTotal   int64
	CoresUsed    int64
	MemoryTotal  int64
	MemoryUsed   int64
}

type DeviceAllocation struct {
	AllocationResults []*AllocationResult
	UsedBy            []string
	NodeName          string
}

type AllocationResult struct {
	Namespace  string
	DeviceName string
	Cores      int64
	Memory     int64
}

func NewNodeDevices() *NodeDevices {
	return &NodeDevices{
		Nodes: make(map[string]*NodeDeviceInfo),
		Claims: &ClaimsCache{
			Claims: make(map[string]*DeviceAllocation),
		},
	}
}

func (n *NodeDevices) getOrCreateNodeInfo(nodeName string) *NodeDeviceInfo {
	n.mu.Lock()
	defer n.mu.Unlock()

	nodeInfo, ok := n.Nodes[nodeName]
	if !ok {
		nodeInfo = &NodeDeviceInfo{
			Devices: make([]*NodeDevice, 0),
		}
		n.Nodes[nodeName] = nodeInfo
	}
	return nodeInfo
}

func (n *NodeDevices) ParseNodeDevice(slice *resourceapi.ResourceSlice) []*NodeDevice {
	klog.V(4).InfoS("Parsing ResourceSlice", "name", slice.Name, "node", slice.Spec.NodeName)
	nodeDevices := []*NodeDevice{}
	for _, device := range slice.Spec.Devices {
		coresTotal := device.Capacity[constants.DeviceCapacityCores].Value
		memoryTotal := device.Capacity[constants.DeviceCapacityMemory].Value

		uuid := ""
		if uuidAttr, ok := device.Attributes[constants.DeviceAttributeUUID]; ok && uuidAttr.StringValue != nil {
			uuid = *uuidAttr.StringValue
		}

		architecture := ""
		if archAttr, ok := device.Attributes[constants.DeviceAttributeArchitecture]; ok && archAttr.StringValue != nil {
			architecture = *archAttr.StringValue
		}

		brand := ""
		if brandAttr, ok := device.Attributes[constants.DeviceAttributeBrand]; ok && brandAttr.StringValue != nil {
			brand = *brandAttr.StringValue
		}

		productName := ""
		if productAttr, ok := device.Attributes[constants.DeviceAttributeProductName]; ok && productAttr.StringValue != nil {
			productName = *productAttr.StringValue
		}

		nodeDevices = append(nodeDevices, &NodeDevice{
			Name:         device.Name,
			UUID:         uuid,
			Architecture: architecture,
			Brand:        brand,
			ProductName:  productName,
			CoresTotal:   coresTotal.Value(),
			CoresUsed:    0,
			MemoryTotal:  memoryTotal.Value(),
			MemoryUsed:   0,
		})
	}
	return nodeDevices
}

// GetDevices returns a copy of the devices on the node.
func (n *NodeDevices) GetDevices(nodeName string) []*NodeDevice {
	n.mu.RLock()
	nodeInfo := n.Nodes[nodeName]
	n.mu.RUnlock()

	if nodeInfo == nil {
		return nil
	}

	nodeInfo.mu.RLock()
	defer nodeInfo.mu.RUnlock()

	deviceCopies := make([]*NodeDevice, len(nodeInfo.Devices))
	for i, device := range nodeInfo.Devices {
		deviceCopies[i] = &NodeDevice{
			Name:         device.Name,
			UUID:         device.UUID,
			Architecture: device.Architecture,
			Brand:        device.Brand,
			ProductName:  device.ProductName,
			CoresTotal:   device.CoresTotal,
			CoresUsed:    device.CoresUsed,
			MemoryTotal:  device.MemoryTotal,
			MemoryUsed:   device.MemoryUsed,
		}
	}
	return deviceCopies
}

func (n *NodeDevices) GetAllNodes() []string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	nodes := make([]string, 0, len(n.Nodes))
	for nodeName := range n.Nodes {
		nodes = append(nodes, nodeName)
	}
	return nodes
}

func (n *NodeDevices) GetAllClaims() map[string]*DeviceAllocation {
	n.Claims.Mux.RLock()
	defer n.Claims.Mux.RUnlock()

	result := make(map[string]*DeviceAllocation)
	maps.Copy(result, n.Claims.Claims)
	return result
}

func (n *NodeDevices) onAddClaim(claim *resourceapi.ResourceClaim) {
	if claim.Status.Allocation == nil {
		klog.V(5).Infof("ResourceClaim %s/%s has no allocation yet", claim.Namespace, claim.Name)
		return
	}

	claimKey := fmt.Sprintf("%s/%s", claim.Namespace, claim.Name)

	if claim.Status.Allocation.Devices.Results != nil {

		allocation := &DeviceAllocation{}
		usedBy := []string{}
		for _, used := range claim.Status.ReservedFor {
			usedBy = append(usedBy, used.Name)
		}
		allocation.UsedBy = usedBy
		nodeName, err := getNodeNameFromNodeSelector(claim.Status.Allocation.NodeSelector)
		if err != nil {
			klog.Warningf("Failed to get node name from node selector: %v", err)
			return
		}
		allocation.NodeName = nodeName

		for _, result := range claim.Status.Allocation.Devices.Results {
			if result.ConsumedCapacity != nil && result.Device != "" {

				coresPerDevice := result.ConsumedCapacity[constants.DeviceCapacityCores]
				memoryPerDevice := result.ConsumedCapacity[constants.DeviceCapacityMemory]
				klog.V(5).Infof("Increasing node usage for device %s on node %s: CoresUsed=%d, MemoryUsed=%d",
					result.Device, nodeName, coresPerDevice.Value(), memoryPerDevice.Value())
				n.increseNodeUsage(nodeName, result.Device, coresPerDevice.Value(), memoryPerDevice.Value())

				allocationResult := &AllocationResult{
					Namespace:  claim.Namespace,
					DeviceName: result.Device,
					Cores:      coresPerDevice.Value(),
					Memory:     memoryPerDevice.Value(),
				}
				allocation.AllocationResults = append(allocation.AllocationResults, allocationResult)
			} else {
				klog.Warningf("ResourceClaim %s/%s has no consumed capacity or pool or device", claim.Namespace, claim.Name)
				continue
			}
		}

		n.Claims.Mux.Lock()
		n.Claims.Claims[claimKey] = allocation
		n.Claims.Mux.Unlock()
	}
}

func (n *NodeDevices) onDeleteClaim(claim *resourceapi.ResourceClaim) {
	claimKey := fmt.Sprintf("%s/%s", claim.Namespace, claim.Name)

	n.Claims.Mux.RLock()
	allocation, ok := n.Claims.Claims[claimKey]
	n.Claims.Mux.RUnlock()

	if !ok {
		if claim.Status.Allocation == nil {
			return
		}

		if claim.Status.Allocation.Devices.Results != nil {
			nodeName, err := getNodeNameFromNodeSelector(claim.Status.Allocation.NodeSelector)
			if err != nil {
				klog.Warningf("Failed to get node name from node selector: %v", err)
				return
			}
			for _, result := range claim.Status.Allocation.Devices.Results {
				if result.ConsumedCapacity != nil && result.Device != "" {
					coresPerDevice := result.ConsumedCapacity[constants.DeviceCapacityCores]
					memoryPerDevice := result.ConsumedCapacity[constants.DeviceCapacityMemory]
					klog.V(5).Infof("Decreasing node usage for device %s on node %s: CoresUsed=%d, MemoryUsed=%d",
						result.Device, nodeName, coresPerDevice.Value(), memoryPerDevice.Value())
					n.decreaseNodeUsage(nodeName, result.Device, coresPerDevice.Value(), memoryPerDevice.Value())
				}
			}
		}
		return
	}

	for _, result := range allocation.AllocationResults {
		klog.V(1).Infof("Decreasing node usage for device %s on node %s: CoresUsed=%d, MemoryUsed=%d",
			result.DeviceName, allocation.NodeName, result.Cores, result.Memory)
		n.decreaseNodeUsage(allocation.NodeName, result.DeviceName, result.Cores, result.Memory)
	}

	n.Claims.Mux.Lock()
	delete(n.Claims.Claims, claimKey)
	n.Claims.Mux.Unlock()
}

func (n *NodeDevices) onUpdateClaim(oldClaim, newClaim *resourceapi.ResourceClaim) {
	n.onDeleteClaim(oldClaim)
	n.onAddClaim(newClaim)
}

func (n *NodeDevices) increseNodeUsage(nodeName, deviceName string, coresPerDevice, memoryPerDevice int64) {
	n.mu.RLock()
	nodeInfo := n.Nodes[nodeName]
	n.mu.RUnlock()

	if nodeInfo == nil {
		klog.Warningf("Node %s not found when updating device usage", nodeName)
		return
	}

	nodeInfo.mu.Lock()
	defer nodeInfo.mu.Unlock()

	for _, device := range nodeInfo.Devices {
		if device.Name == deviceName {
			device.CoresUsed += coresPerDevice
			device.MemoryUsed += memoryPerDevice
			klog.V(5).Infof("Updated device %s (%s) on node %s: CoresUsed=%d, MemoryUsed=%d",
				device.Name, device.UUID, nodeName, device.CoresUsed, device.MemoryUsed)
			break
		}
	}
}

func (n *NodeDevices) decreaseNodeUsage(nodeName, deviceName string, coresPerDevice, memoryPerDevice int64) {
	n.mu.RLock()
	nodeInfo := n.Nodes[nodeName]
	n.mu.RUnlock()

	if nodeInfo == nil {
		klog.Warningf("Node %s not found when decreasing device usage", nodeName)
		return
	}

	nodeInfo.mu.Lock()
	defer nodeInfo.mu.Unlock()

	for _, device := range nodeInfo.Devices {
		if device.Name == deviceName {
			device.CoresUsed -= coresPerDevice
			device.MemoryUsed -= memoryPerDevice
			if device.CoresUsed < 0 {
				klog.Warningf("CoresUsed is less than 0 for device %s (%s) on node %s", device.Name, device.UUID, nodeName)
				device.CoresUsed = 0
			}
			if device.MemoryUsed < 0 {
				klog.Warningf("MemoryUsed is less than 0 for device %s (%s) on node %s", device.Name, device.UUID, nodeName)
				device.MemoryUsed = 0
			}
			klog.V(5).Infof("Released device %s (%s) on node %s: CoresUsed=%d, MemoryUsed=%d",
				device.Name, device.UUID, nodeName, device.CoresUsed, device.MemoryUsed)
			break
		}
	}
}

func getNodeNameFromNodeSelector(nodeSelector *v1.NodeSelector) (string, error) {
	if nodeSelector != nil && nodeSelector.NodeSelectorTerms != nil {
		for _, match := range nodeSelector.NodeSelectorTerms {
			for _, matchField := range match.MatchFields {
				if matchField.Key == "metadata.name" && matchField.Operator == v1.NodeSelectorOpIn {
					if len(matchField.Values) == 0 {
						return "", fmt.Errorf("node selector value is empty")
					}
					return matchField.Values[0], nil
				}
			}
		}
	}
	return "", fmt.Errorf("node selector does not contain metadata.name")
}
