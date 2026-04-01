/*
Copyright 2026 The HAMi Authors.

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

package main

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDevicesForNodeMergesGroupsAndNodeOverrides(t *testing.T) {
	groupModel := "A100"
	groupUUID := "group-gpu-0"
	groupMemoryDefault := "80Gi"
	groupMemoryMin := "1Mi"
	groupMemoryMax := "80Gi"
	groupMemoryStep := "1Mi"
	groupMinor := int64(0)
	nodeModel := "custom"
	nodeUUID := "node-gpu-0"
	nodeMinor := int64(2)
	nodeGPU1UUID := "node-gpu-1"
	nodeGPU1Arch := "Ampere"
	driverVersion := "575.57.8"

	cfg := &DriverConfig{
		NodeSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"role": "worker"},
		},
		Groups: []DeviceGroup{
			{
				Name: "a100",
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"gpu-type": "a100"},
				},
				Devices: []FakeDevice{
					{
						Name: "gpu-0",
						Attributes: map[string]FakeAttributeValue{
							"model": {String: &groupModel},
							"uuid":  {String: &groupUUID},
							"minor": {Int: &groupMinor},
						},
						Capacity: map[string]FakeCapacity{
							"memory": {
								Value: "80Gi",
								RequestPolicy: &FakeCapacityRequestPolicy{
									Default: &groupMemoryDefault,
									ValidRange: &FakeCapacityPolicyRange{
										Min:  &groupMemoryMin,
										Max:  &groupMemoryMax,
										Step: &groupMemoryStep,
									},
								},
							},
						},
					},
				},
			},
		},
		Nodes: map[string]NodeConfig{
			"worker-1": {
				Devices: []FakeDevice{
					{
						Name: "gpu-0",
						Attributes: map[string]FakeAttributeValue{
							"model": {String: &nodeModel},
							"uuid":  {String: &nodeUUID},
							"minor": {Int: &nodeMinor},
						},
					},
					{
						Name: "gpu-1",
						Attributes: map[string]FakeAttributeValue{
							"uuid":                       {String: &nodeGPU1UUID},
							"architecture":               {String: &nodeGPU1Arch},
							"driverVersion":              {Version: &driverVersion},
							"attr.project-hami.io/minor": {Int: &nodeMinor},
						},
					},
				},
			},
		},
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-1",
			Labels: map[string]string{
				"role":     "worker",
				"gpu-type": "a100",
			},
		},
	}

	devices, err := cfg.DevicesForNode(node)
	if err != nil {
		t.Fatalf("DevicesForNode returned error: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}
	if got := *devices[0].Attributes["uuid"].StringValue; got != "node-gpu-0" {
		t.Fatalf("expected node override for gpu-0 uuid, got %q", got)
	}
	if got := *devices[0].Attributes["minor"].IntValue; got != 2 {
		t.Fatalf("expected node override for gpu-0 minor, got %d", got)
	}
	if got := *devices[1].Attributes["uuid"].StringValue; got != "node-gpu-1" {
		t.Fatalf("expected explicit gpu-1 device, got %q", got)
	}
	if got := *devices[1].Attributes["driverVersion"].VersionValue; got != "575.57.8" {
		t.Fatalf("expected version attribute for gpu-1, got %q", got)
	}
	if devices[0].AllowMultipleAllocations == nil || !*devices[0].AllowMultipleAllocations {
		t.Fatalf("expected requestPolicy to enable allowMultipleAllocations")
	}
	if devices[0].Capacity["memory"].RequestPolicy == nil {
		t.Fatalf("expected memory requestPolicy to be preserved")
	}
}

func TestDevicesForNodeHonorsGlobalNodeSelector(t *testing.T) {
	cfg := &DriverConfig{
		NodeSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"role": "worker"},
		},
		Nodes: map[string]NodeConfig{
			"infra-1": {
				Devices: []FakeDevice{{Name: "gpu-0"}},
			},
		},
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "infra-1",
			Labels: map[string]string{"role": "infra"},
		},
	}

	devices, err := cfg.DevicesForNode(node)
	if err != nil {
		t.Fatalf("DevicesForNode returned error: %v", err)
	}
	if len(devices) != 0 {
		t.Fatalf("expected no devices, got %d", len(devices))
	}
}

func TestFakeAttributeValueRequiresExactlyOneType(t *testing.T) {
	value := "Ampere"
	minor := int64(2)

	if _, err := (FakeAttributeValue{}).ToDeviceAttribute(); err == nil {
		t.Fatalf("expected empty attribute to fail")
	}
	if _, err := (FakeAttributeValue{String: &value, Int: &minor}).ToDeviceAttribute(); err == nil {
		t.Fatalf("expected multi-type attribute to fail")
	}
}
