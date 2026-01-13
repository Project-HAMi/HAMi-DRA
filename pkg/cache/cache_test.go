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
	"testing"
	"time"

	"github.com/Project-HAMi/HAMi-DRA/pkg/constants"
	v1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewCache(t *testing.T) {
	// Use fake client for testing to avoid requiring real Kubernetes cluster
	client := fake.NewSimpleClientset()
	c := NewCacheWithClient(client)
	if c == nil {
		t.Fatal("NewCacheWithClient() returned nil")
	}
	if c.NodeDevices == nil {
		t.Error("NodeDevices should not be nil")
	}
	if c.stopCh == nil {
		t.Error("stopCh should not be nil")
	}
	if c.ready {
		t.Error("Cache should not be ready initially")
	}
}

func TestCache_Start_Stop(t *testing.T) {
	// Create a fake Kubernetes client
	client := fake.NewSimpleClientset()

	// Create cache with fake client
	c := NewCacheWithClient(client)

	// Start cache
	err := c.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Wait a bit for informers to sync
	time.Sleep(100 * time.Millisecond)

	// Check if ready
	if !c.IsReady() {
		t.Error("Cache should be ready after Start()")
	}

	// Stop cache
	c.Stop()

	// Check if stopped
	if c.IsReady() {
		t.Error("Cache should not be ready after Stop()")
	}
}

func TestCache_GetDevices(t *testing.T) {
	c := &Cache{
		NodeDevices: NewNodeDevices(),
	}

	// Initially should be empty
	nodeNames := c.NodeDevices.GetAllNodes()
	if len(nodeNames) != 0 {
		t.Errorf("Expected empty nodes list, got %d nodes", len(nodeNames))
	}

	// Add some devices manually
	nodeName := "test-node"
	c.NodeDevices.mu.Lock()
	nodeInfo := &NodeDeviceInfo{
		Devices: []*NodeDevice{
			{
				Name:         "gpu0",
				UUID:         "test-uuid-1",
				Architecture: "sm_80",
				Brand:        "NVIDIA",
				ProductName:  "Tesla V100",
				CoresTotal:   100,
				CoresUsed:    50,
				MemoryTotal:  16000,
				MemoryUsed:   8000,
			},
		},
	}
	c.NodeDevices.Nodes[nodeName] = nodeInfo
	c.NodeDevices.mu.Unlock()

	// Get devices using GetDevices
	devices := c.NodeDevices.GetDevices(nodeName)
	if devices == nil {
		t.Fatalf("Expected devices for node %s", nodeName)
	}

	if len(devices) != 1 {
		t.Fatalf("Expected 1 device, got %d", len(devices))
	}

	device := devices[0]
	if device.Name != "gpu0" {
		t.Errorf("Expected device name 'gpu0', got '%s'", device.Name)
	}
	if device.CoresTotal != 100 {
		t.Errorf("Expected CoresTotal 100, got %d", device.CoresTotal)
	}
	if device.CoresUsed != 50 {
		t.Errorf("Expected CoresUsed 50, got %d", device.CoresUsed)
	}

	// Verify GetDevices returns a copy (modifying returned devices shouldn't affect original)
	device.CoresUsed = 999
	c.NodeDevices.mu.RLock()
	originalDevice := c.NodeDevices.Nodes[nodeName].Devices[0]
	c.NodeDevices.mu.RUnlock()
	if originalDevice.CoresUsed != 50 {
		t.Error("Modifying returned device should not affect original device")
	}
}

func TestCache_onAddSlice(t *testing.T) {
	c := &Cache{
		NodeDevices: NewNodeDevices(),
		stopCh:      make(chan struct{}),
		ready:       false,
	}

	nodeName := "test-node"
	slice := createTestResourceSlice("test-slice", nodeName, "gpu0", "test-uuid-1")

	// Test adding slice
	c.onAddSlice(slice)

	// Verify device was added
	c.NodeDevices.mu.RLock()
	nodeInfo, ok := c.NodeDevices.Nodes[nodeName]
	c.NodeDevices.mu.RUnlock()

	if !ok {
		t.Fatal("Device should be added to cache")
	}

	nodeInfo.mu.RLock()
	devices := nodeInfo.Devices
	nodeInfo.mu.RUnlock()

	if len(devices) != 1 {
		t.Fatalf("Expected 1 device, got %d", len(devices))
	}

	device := devices[0]
	if device.Name != "gpu0" {
		t.Errorf("Expected device name 'gpu0', got '%s'", device.Name)
	}
	if device.UUID != "test-uuid-1" {
		t.Errorf("Expected UUID 'test-uuid-1', got '%s'", device.UUID)
	}
}

func TestCache_onUpdateSlice(t *testing.T) {
	c := &Cache{
		NodeDevices: NewNodeDevices(),
		stopCh:      make(chan struct{}),
		ready:       false,
	}

	nodeName := "test-node"
	oldSlice := createTestResourceSlice("test-slice", nodeName, "gpu0", "test-uuid-1")
	newSlice := createTestResourceSlice("test-slice", nodeName, "gpu0", "test-uuid-1")

	// Add initial slice
	c.onAddSlice(oldSlice)

	// Update slice
	c.onUpdateSlice(oldSlice, newSlice)

	// Verify device still exists
	c.NodeDevices.mu.RLock()
	nodeInfo, ok := c.NodeDevices.Nodes[nodeName]
	c.NodeDevices.mu.RUnlock()

	if !ok {
		t.Fatal("Device should still exist after update")
	}

	nodeInfo.mu.RLock()
	devices := nodeInfo.Devices
	nodeInfo.mu.RUnlock()

	if len(devices) != 1 {
		t.Fatalf("Expected 1 device, got %d", len(devices))
	}
}

func TestCache_onDeleteSlice(t *testing.T) {
	c := &Cache{
		NodeDevices: NewNodeDevices(),
		stopCh:      make(chan struct{}),
		ready:       false,
	}

	nodeName := "test-node"
	slice := createTestResourceSlice("test-slice", nodeName, "gpu0", "test-uuid-1")

	// Add slice
	c.onAddSlice(slice)

	// Verify device exists
	c.NodeDevices.mu.RLock()
	_, ok := c.NodeDevices.Nodes[nodeName]
	c.NodeDevices.mu.RUnlock()

	if !ok {
		t.Fatal("Device should exist before delete")
	}

	// Delete slice
	c.onDeleteSlice(slice)

	// Verify device was removed
	c.NodeDevices.mu.RLock()
	_, ok = c.NodeDevices.Nodes[nodeName]
	c.NodeDevices.mu.RUnlock()

	if ok {
		t.Error("Device should be removed after delete")
	}
}

func TestCache_onAddSlice_NoNodeName(t *testing.T) {
	c := &Cache{
		NodeDevices: NewNodeDevices(),
		stopCh:      make(chan struct{}),
		ready:       false,
	}

	// Create slice without node name
	slice := &resourceapi.ResourceSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-slice",
		},
		Spec: resourceapi.ResourceSliceSpec{
			NodeName: nil, // No node name
			Devices: []resourceapi.Device{
				{
					Name: "gpu0",
					Capacity: map[resourceapi.QualifiedName]resourceapi.DeviceCapacity{
						constants.DeviceCapacityCores: {
							Value: resource.MustParse("100"),
						},
						constants.DeviceCapacityMemory: {
							Value: resource.MustParse("16000"),
						},
					},
					Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
						constants.DeviceAttributeUUID: {
							StringValue: stringPtr("test-uuid-1"),
						},
					},
				},
			},
		},
	}

	// Should not panic and should skip
	c.onAddSlice(slice)

	// Verify no devices were added
	c.NodeDevices.mu.RLock()
	if len(c.NodeDevices.Nodes) != 0 {
		t.Error("No devices should be added when node name is missing")
	}
	c.NodeDevices.mu.RUnlock()
}

func TestCache_onAddClaim_NoAllocation(t *testing.T) {
	c := &Cache{
		NodeDevices: NewNodeDevices(),
		stopCh:      make(chan struct{}),
		ready:       false,
	}

	// Create claim without allocation
	claim := &resourceapi.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-claim",
			Namespace: "default",
		},
		Spec: resourceapi.ResourceClaimSpec{
			Devices: resourceapi.DeviceClaim{
				Requests: []resourceapi.DeviceRequest{
					{
						Name: "gpu",
						Exactly: &resourceapi.ExactDeviceRequest{
							Count: 1,
						},
					},
				},
			},
		},
		Status: resourceapi.ResourceClaimStatus{
			Allocation: nil, // No allocation
		},
	}

	// Should not panic
	c.onAddClaim(claim)

	// Verify no allocations were recorded
	c.NodeDevices.Claims.Mux.RLock()
	if len(c.NodeDevices.Claims.Claims) != 0 {
		t.Error("No allocations should be recorded when allocation is nil")
	}
	c.NodeDevices.Claims.Mux.RUnlock()
}

func TestCache_onAddClaim_WithAllocation(t *testing.T) {
	c := &Cache{
		NodeDevices: NewNodeDevices(),
		stopCh:      make(chan struct{}),
		ready:       false,
	}

	// First add a device
	nodeName := "test-node"
	slice := createTestResourceSlice("test-slice", nodeName, "gpu0", "test-uuid-1")
	c.onAddSlice(slice)

	// Create claim with allocation
	claim := createTestResourceClaim("test-claim", "default", nodeName, "gpu0", 50, 8000)

	c.onAddClaim(claim)

	// Verify device usage was updated
	c.NodeDevices.mu.RLock()
	nodeInfo, ok := c.NodeDevices.Nodes[nodeName]
	c.NodeDevices.mu.RUnlock()

	if !ok {
		t.Fatal("Device should exist")
	}

	nodeInfo.mu.RLock()
	devices := nodeInfo.Devices
	nodeInfo.mu.RUnlock()

	if len(devices) != 1 {
		t.Fatalf("Expected 1 device, got %d", len(devices))
	}

	device := devices[0]
	if device.CoresUsed != 50 {
		t.Errorf("Expected CoresUsed 50, got %d", device.CoresUsed)
	}
	if device.MemoryUsed != 8000 {
		t.Errorf("Expected MemoryUsed 8000, got %d", device.MemoryUsed)
	}

	// Verify allocation was recorded
	c.NodeDevices.Claims.Mux.RLock()
	allocation, ok := c.NodeDevices.Claims.Claims["default/test-claim"]
	c.NodeDevices.Claims.Mux.RUnlock()

	if !ok {
		t.Error("Allocation should be recorded")
	}

	if allocation == nil {
		t.Error("Allocation should not be nil")
	}
}

func TestCache_onDeleteClaim(t *testing.T) {
	c := &Cache{
		NodeDevices: NewNodeDevices(),
		stopCh:      make(chan struct{}),
		ready:       false,
	}

	// First add a device
	nodeName := "test-node"
	slice := createTestResourceSlice("test-slice", nodeName, "gpu0", "test-uuid-1")
	c.onAddSlice(slice)

	// Create and add claim
	claim := createTestResourceClaim("test-claim", "default", nodeName, "gpu0", 50, 8000)

	c.onAddClaim(claim)

	// Verify device usage
	c.NodeDevices.mu.RLock()
	nodeInfo := c.NodeDevices.Nodes[nodeName]
	c.NodeDevices.mu.RUnlock()

	nodeInfo.mu.RLock()
	devices := nodeInfo.Devices
	nodeInfo.mu.RUnlock()

	if devices[0].CoresUsed != 50 {
		t.Errorf("Expected CoresUsed 50 before delete, got %d", devices[0].CoresUsed)
	}

	// Delete claim
	c.onDeleteClaim(claim)

	// Verify device usage was released
	c.NodeDevices.mu.RLock()
	nodeInfo = c.NodeDevices.Nodes[nodeName]
	c.NodeDevices.mu.RUnlock()

	c.NodeDevices.Claims.Mux.RLock()
	_, ok := c.NodeDevices.Claims.Claims["default/test-claim"]
	c.NodeDevices.Claims.Mux.RUnlock()

	nodeInfo.mu.RLock()
	devices = nodeInfo.Devices
	nodeInfo.mu.RUnlock()

	if devices[0].CoresUsed != 0 {
		t.Errorf("Expected CoresUsed 0 after delete, got %d", devices[0].CoresUsed)
	}

	if ok {
		t.Error("Allocation record should be removed after delete")
	}
}

func TestCache_onUpdateClaim(t *testing.T) {
	c := &Cache{
		NodeDevices: NewNodeDevices(),
		stopCh:      make(chan struct{}),
		ready:       false,
	}

	// First add a device
	nodeName := "test-node"
	slice := createTestResourceSlice("test-slice", nodeName, "gpu0", "test-uuid-1")
	c.onAddSlice(slice)

	oldClaim := createTestResourceClaim("test-claim", "default", nodeName, "gpu0", 50, 8000)
	newClaim := createTestResourceClaim("test-claim", "default", nodeName, "gpu0", 75, 12000)

	c.onAddClaim(oldClaim)
	c.onUpdateClaim(oldClaim, newClaim)

	// Verify device usage was updated
	c.NodeDevices.mu.RLock()
	nodeInfo := c.NodeDevices.Nodes[nodeName]
	c.NodeDevices.mu.RUnlock()

	nodeInfo.mu.RLock()
	devices := nodeInfo.Devices
	nodeInfo.mu.RUnlock()

	if devices[0].CoresUsed != 75 {
		t.Errorf("Expected CoresUsed 75 after update, got %d", devices[0].CoresUsed)
	}
	if devices[0].MemoryUsed != 12000 {
		t.Errorf("Expected MemoryUsed 12000 after update, got %d", devices[0].MemoryUsed)
	}
}

func TestCache_IsReady(t *testing.T) {
	c := &Cache{
		NodeDevices: NewNodeDevices(),
		stopCh:      make(chan struct{}),
		ready:       false,
	}

	if c.IsReady() {
		t.Error("Cache should not be ready initially")
	}

	c.ready = true
	if !c.IsReady() {
		t.Error("Cache should be ready after setting ready to true")
	}
}

// Helper functions

func createTestResourceSlice(name, nodeName, deviceName, uuid string) *resourceapi.ResourceSlice {
	return &resourceapi.ResourceSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: resourceapi.ResourceSliceSpec{
			NodeName: stringPtr(nodeName),
			Devices: []resourceapi.Device{
				{
					Name: deviceName,
					Capacity: map[resourceapi.QualifiedName]resourceapi.DeviceCapacity{
						constants.DeviceCapacityCores: {
							Value: resource.MustParse("100"),
						},
						constants.DeviceCapacityMemory: {
							Value: resource.MustParse("16000"),
						},
					},
					Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
						constants.DeviceAttributeUUID: {
							StringValue: stringPtr(uuid),
						},
						constants.DeviceAttributeArchitecture: {
							StringValue: stringPtr("sm_80"),
						},
						constants.DeviceAttributeBrand: {
							StringValue: stringPtr("NVIDIA"),
						},
						constants.DeviceAttributeProductName: {
							StringValue: stringPtr("Tesla V100"),
						},
					},
				},
			},
		},
	}
}

func stringPtr(s string) *string {
	return &s
}

// createTestResourceClaim creates a ResourceClaim with allocation and NodeSelector
func createTestResourceClaim(name, namespace, nodeName, deviceName string, cores, memory int64) *resourceapi.ResourceClaim {
	return &resourceapi.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: resourceapi.ResourceClaimSpec{
			Devices: resourceapi.DeviceClaim{
				Requests: []resourceapi.DeviceRequest{
					{
						Name: "gpu",
						Exactly: &resourceapi.ExactDeviceRequest{
							Count: 1,
							Capacity: &resourceapi.CapacityRequirements{
								Requests: map[resourceapi.QualifiedName]resource.Quantity{
									constants.DeviceCapacityCores:  resource.MustParse(fmt.Sprintf("%d", cores)),
									constants.DeviceCapacityMemory: resource.MustParse(fmt.Sprintf("%d", memory)),
								},
							},
						},
					},
				},
			},
		},
		Status: resourceapi.ResourceClaimStatus{
			Allocation: &resourceapi.AllocationResult{
				NodeSelector: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchFields: []v1.NodeSelectorRequirement{
								{
									Key:      "metadata.name",
									Operator: v1.NodeSelectorOpIn,
									Values:   []string{nodeName},
								},
							},
						},
					},
				},
				Devices: resourceapi.DeviceAllocationResult{
					Results: []resourceapi.DeviceRequestAllocationResult{
						{
							Request: "gpu",
							Driver:  "test-driver",
							Pool:    nodeName,
							Device:  deviceName,
							ConsumedCapacity: map[resourceapi.QualifiedName]resource.Quantity{
								constants.DeviceCapacityCores:  resource.MustParse(fmt.Sprintf("%d", cores)),
								constants.DeviceCapacityMemory: resource.MustParse(fmt.Sprintf("%d", memory)),
							},
						},
					},
				},
			},
		},
	}
}
