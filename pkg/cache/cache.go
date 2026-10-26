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
	"reflect"
	"sync/atomic"
	"time"

	"github.com/Project-HAMi/HAMi-DRA/pkg/utils"
	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listersresourcev1 "k8s.io/client-go/listers/resource/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type Cache struct {
	*NodeDevices

	stopCh      chan struct{}
	kubeClient  kubernetes.Interface
	sliceLister listersresourcev1.ResourceSliceLister
	claimLister listersresourcev1.ResourceClaimLister
	ready       atomic.Bool
}

// NewCache creates a new Cache with a Kubernetes client.
// This function will attempt to create a real Kubernetes client.
// For testing, use NewCacheWithClient instead.
func NewCache() *Cache {
	klog.Infof("Initializing cache")
	kubeClient, err := utils.NewClient()
	if err != nil {
		klog.Fatalf("Failed to create Kubernetes client: %v", err)
	}
	return NewCacheWithClient(kubeClient.Interface)
}

// NewCacheWithClient creates a new Cache with the provided Kubernetes client.
// This is useful for testing with fake clients.
func NewCacheWithClient(kubeClient kubernetes.Interface) *Cache {
	return &Cache{
		stopCh:      make(chan struct{}),
		NodeDevices: NewNodeDevices(),
		kubeClient:  kubeClient,
	}
}

func (c *Cache) Start() error {
	informerFactory := informers.NewSharedInformerFactoryWithOptions(c.kubeClient, time.Hour*1)
	c.sliceLister = informerFactory.Resource().V1().ResourceSlices().Lister()
	c.claimLister = informerFactory.Resource().V1().ResourceClaims().Lister()

	// First, set up ResourceSlice informer and handlers
	sliceInformer := informerFactory.Resource().V1().ResourceSlices().Informer()
	_, err := sliceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onAddSlice,
		UpdateFunc: c.onUpdateSlice,
		DeleteFunc: c.onDeleteSlice,
	})
	if err != nil {
		return fmt.Errorf("failed to add ResourceSlice event handler: %w", err)
	}

	// Start informer factory
	klog.V(5).Info("Starting informer factory")
	informerFactory.Start(c.stopCh)

	// Wait for ResourceSlice to sync completely before registering ResourceClaim handlers
	// This ensures that all node/device information is available when processing ResourceClaim events
	klog.V(5).Info("Waiting for ResourceSlice cache to sync")
	if !cache.WaitForCacheSync(c.stopCh, sliceInformer.HasSynced) {
		return fmt.Errorf("failed to sync ResourceSlice informer")
	}
	klog.V(5).Info("ResourceSlice cache synced successfully")

	// Now set up ResourceClaim informer handlers
	// ResourceSlice data is already available, so ResourceClaim handlers can safely reference it
	claimInformer := informerFactory.Resource().V1().ResourceClaims().Informer()
	_, err = claimInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onAddClaim,
		UpdateFunc: c.onUpdateClaim,
		DeleteFunc: c.onDeleteClaim,
	})
	if err != nil {
		return fmt.Errorf("failed to add ResourceClaim event handler: %w", err)
	}

	// Wait for ResourceClaim to sync
	// Note: The informer may have already synced, but we need to ensure it's synced after handlers are registered
	klog.V(5).Info("Waiting for ResourceClaim cache to sync")
	if !cache.WaitForCacheSync(c.stopCh, claimInformer.HasSynced) {
		return fmt.Errorf("failed to sync ResourceClaim informer")
	}
	klog.V(5).Info("ResourceClaim cache synced successfully")

	c.ready.Store(true)
	klog.Infof("Cache started and synced successfully")
	return nil
}

func (c *Cache) Stop() {
	close(c.stopCh)
	c.ready.Store(false)
	klog.Infof("Cache stopped")
}

func (c *Cache) IsReady() bool {
	return c.ready.Load()
}

func (c *Cache) onAddClaim(obj interface{}) {
	claim, ok := obj.(*resourceapi.ResourceClaim)
	if !ok {
		return
	}
	c.NodeDevices.onAddClaim(claim)
}

func (c *Cache) onUpdateClaim(oldObj, newObj interface{}) {
	oldClaim, ok1 := oldObj.(*resourceapi.ResourceClaim)
	newClaim, ok2 := newObj.(*resourceapi.ResourceClaim)
	if !ok1 || !ok2 {
		return
	}
	if reflect.DeepEqual(oldClaim.Status, newClaim.Status) {
		return
	}
	c.NodeDevices.onUpdateClaim(oldClaim, newClaim)
}

func (c *Cache) onDeleteClaim(obj interface{}) {
	claim, ok := obj.(*resourceapi.ResourceClaim)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		claim, ok = tombstone.Obj.(*resourceapi.ResourceClaim)
		if !ok {
			return
		}
	}
	c.NodeDevices.onDeleteClaim(claim)
}

// onAddSlice is called when a new ResourceSlice is added
func (c *Cache) onAddSlice(obj interface{}) {
	slice, ok := obj.(*resourceapi.ResourceSlice)
	if !ok {
		return
	}
	if slice.Spec.NodeName == nil {
		klog.Warningf("ResourceSlice %s has no node name, skipping", slice.Name)
		return
	}
	nodeName := *slice.Spec.NodeName
	devices := c.ParseNodeDevice(slice)
	c.NodeDevices.setSliceDevices(nodeName, slice.Name, devices)
	klog.V(4).Infof("Added ResourceSlice %s for node %s with %d devices", slice.Name, nodeName, len(devices))
}

func (c *Cache) onUpdateSlice(oldObj, newObj interface{}) {
	oldSlice, ok1 := oldObj.(*resourceapi.ResourceSlice)
	newSlice, ok2 := newObj.(*resourceapi.ResourceSlice)
	if !ok1 || !ok2 {
		return
	}
	if newSlice.Spec.NodeName == nil {
		klog.Warningf("ResourceSlice %s has no node name, skipping update", newSlice.Name)
		return
	}
	newNodeName := *newSlice.Spec.NodeName

	if oldSlice.Spec.NodeName != nil && *oldSlice.Spec.NodeName != newNodeName {
		c.NodeDevices.setSliceDevices(*oldSlice.Spec.NodeName, oldSlice.Name, nil)
	}

	c.NodeDevices.setSliceDevices(newNodeName, newSlice.Name, c.ParseNodeDevice(newSlice))
	klog.V(4).Infof("Updated ResourceSlice %s for node %s", newSlice.Name, newNodeName)
}

func (c *Cache) onDeleteSlice(obj interface{}) {
	slice, ok := obj.(*resourceapi.ResourceSlice)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		slice, ok = tombstone.Obj.(*resourceapi.ResourceSlice)
		if !ok {
			return
		}
	}
	if slice.Spec.NodeName == nil {
		klog.Warningf("ResourceSlice %s has no node name, skipping delete", slice.Name)
		return
	}
	nodeName := *slice.Spec.NodeName
	c.NodeDevices.setSliceDevices(nodeName, slice.Name, nil)

	klog.V(4).Infof("Deleted ResourceSlice %s for node %s", slice.Name, nodeName)
}
