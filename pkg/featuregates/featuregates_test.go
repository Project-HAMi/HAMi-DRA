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

package featuregates

import (
	"strings"
	"testing"

	utilversion "k8s.io/apimachinery/pkg/util/version"

	"github.com/spf13/pflag"
)

func TestFeatureGates(t *testing.T) {
	fg := FeatureGates()
	if fg == nil {
		t.Fatal("FeatureGates() returned nil")
	}
}

func TestEnabled(t *testing.T) {
	// GPUSchedulerPolicy is disabled by default (0.2 alpha)
	enabled := Enabled(GPUSchedulerPolicyByDeviceConstraint)
	if enabled {
		t.Error("GPUSchedulerPolicy should be disabled by default")
	}
}

func TestKnownFeatures(t *testing.T) {
	// Mock the featureGates singleton with a version that supports the feature
	original := featureGates
	defer func() { featureGates = original }()

	// Use v0.2.0 to ensure GPUSchedulerPolicyByDeviceConstraint (introduced in 0.2) is included
	// utilversion.MustParseGeneric is not available in all k8s versions, using ParseGeneric
	v, err := utilversion.ParseGeneric("v0.2.0")
	if err != nil {
		t.Fatalf("Failed to parse version: %v", err)
	}
	featureGates = newFeatureGates(v)

	features := KnownFeatures()
	found := false
	// We look for the feature name. The exact string format might vary slightly
	// but should contain the feature name.
	expected := string(GPUSchedulerPolicyByDeviceConstraint)

	for _, f := range features {
		if strings.Contains(f, expected) {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("KnownFeatures() expected to contain feature %q, but got list: %v", expected, features)
	}
}

func TestAddFlags(t *testing.T) {
	// Mock the featureGates singleton with a version that supports the feature
	original := featureGates
	defer func() { featureGates = original }()

	v, err := utilversion.ParseGeneric("v0.2.0")
	if err != nil {
		t.Fatalf("Failed to parse version: %v", err)
	}
	featureGates = newFeatureGates(v)

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	AddFlags(fs)

	f := fs.Lookup("feature-gates")
	if f == nil {
		t.Fatal("AddFlags() failed to add 'feature-gates' flag")
	}

	if f.Name != "feature-gates" {
		t.Errorf("Expected flag name 'feature-gates', got %s", f.Name)
	}

	usage := f.Usage
	if !strings.Contains(usage, string(GPUSchedulerPolicyByDeviceConstraint)) {
		t.Errorf("Flag usage should contain GPUSchedulerPolicy, got: %s", usage)
	}
}
