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

package version

import (
	"fmt"

	utilversion "k8s.io/apimachinery/pkg/util/version"
)

// ReleaseVersion represents a released version.
type ReleaseVersion struct {
	*utilversion.Version
}

// ParseGitVersion parses a git version string, such as:
// - v1.1.0-73-g7e6d4f69
// - v1.1.0
func ParseGitVersion(gitVersion string) (*ReleaseVersion, error) {
	v, err := utilversion.ParseGeneric(gitVersion)
	if err != nil {
		return nil, err
	}

	return &ReleaseVersion{
		Version: v,
	}, nil
}

// FirstMinorRelease returns the minor release but the patch releases always be 0(vx.y.0). e.g:
// - v1.2.1-12-g2eb92858 --> v1.2.0
// - v1.2.3-12-g2e860210 --> v1.2.0
func (r *ReleaseVersion) FirstMinorRelease() string {
	if r.Version == nil {
		return "<nil>"
	}

	return fmt.Sprintf("v%d.%d.0", r.Version.Major(), r.Version.Minor())
}

// PatchRelease returns the stable version with format "vx.y.z".
func (r *ReleaseVersion) PatchRelease() string {
	if r.Version == nil {
		return "<nil>"
	}

	return fmt.Sprintf("v%d.%d.%d", r.Version.Major(), r.Version.Minor(), r.Version.Patch())
}
