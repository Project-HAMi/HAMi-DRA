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
	"sync"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/component-base/featuregate"

	version "github.com/Project-HAMi/HAMi-DRA/pkg/version"
	"github.com/spf13/pflag"
)

const (
	// GPUSchedulerPolicy allows converting `hami.io/gpu-scheduler-policy` annotation to constraints in ResourceClaim
	GPUSchedulerPolicyByDeviceConstraint featuregate.Feature = "GPUSchedulerPolicyByDeviceConstraint"
)

// defaultFeatureGates contains the default settings for all project-specific feature gates.
// These will be registered with the standard Kubernetes feature gate system.
var defaultFeatureGates = map[featuregate.Feature]featuregate.VersionedSpecs{
	GPUSchedulerPolicyByDeviceConstraint: {
		{
			Default:    false,
			PreRelease: featuregate.Alpha,
			Version:    utilversion.MajorMinor(0, 2),
		},
	},
}

var (
	featureGatesOnce sync.Once
	featureGates     featuregate.MutableVersionedFeatureGate
)

// FeatureGates instantiates and returns the package-level singleton representing
// the set of all feature gates and their values.
// It contains both project-specific feature gates and standard Kubernetes logging feature gates.
func FeatureGates() featuregate.MutableVersionedFeatureGate {
	proVer := version.Get()
	relVer, err := version.ParseGitVersion(proVer.GitVersion)
	if err != nil {
		v := utilversion.MajorMinor(0, 0)
		relVer = &version.ReleaseVersion{
			Version: v,
		}
	}

	if featureGates == nil {
		featureGatesOnce.Do(func() {
			featureGates = newFeatureGates(relVer.Version)
		})
	}
	return featureGates
}

// newFeatureGates instantiates a new set of feature gates with project-specific feature gates,
// along with appropriate default values.
// Mostly used for testing.
func newFeatureGates(version *utilversion.Version) featuregate.MutableVersionedFeatureGate {
	// Create a versioned feature gate with the specified version
	// This ensures proper version handling for our feature gates
	fg := featuregate.NewVersionedFeatureGate(version)

	// Add project-specific feature gates
	utilruntime.Must(fg.AddVersioned(defaultFeatureGates))

	return fg
}

// Enabled returns true if the specified feature gate is enabled in the global FeatureGates singleton.
// This is a convenience function that uses the global feature gate registry.
func Enabled(feature featuregate.Feature) bool {
	return FeatureGates().Enabled(feature)
}

// KnownFeatures returns a list of known feature gates with their descriptions.
func KnownFeatures() []string {
	return FeatureGates().KnownFeatures()
}

func AddFlags(fs *pflag.FlagSet) {
	// Add the project-specific feature gates.
	fs.AddFlag(&pflag.Flag{
		Name: "feature-gates",
		Usage: "A set of key=value pairs that describe feature gates for alpha/experimental features. " +
			"Options are:\n     " + strings.Join(KnownFeatures(), "\n     "),
		Value: FeatureGates().(pflag.Value), //nolint:forcetypeassert // No need for type check: FeatureGates is a *featuregate.featureGate, which implements pflag.Value.
	})
}
