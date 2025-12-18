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

import "testing"

func TestReleaseVersion(t *testing.T) {
	tests := []struct {
		Name                    string
		GitVersion              string
		ExpectFirstMinorRelease string
		ExpectPatchRelease      string
		ExpectError             bool
	}{
		{
			Name:                    "first minor release",
			GitVersion:              "v1.1.0",
			ExpectFirstMinorRelease: "v1.1.0",
			ExpectPatchRelease:      "v1.1.0",
			ExpectError:             false,
		},
		{
			Name:                    "subsequent minor release",
			GitVersion:              "v1.1.1",
			ExpectFirstMinorRelease: "v1.1.0",
			ExpectPatchRelease:      "v1.1.1",
			ExpectError:             false,
		},
		{
			Name:                    "normal git version",
			GitVersion:              "v1.1.1-6-gf20c721a",
			ExpectFirstMinorRelease: "v1.1.0",
			ExpectPatchRelease:      "v1.1.1",
			ExpectError:             false,
		},
		{
			Name:                    "abnormal version",
			GitVersion:              "vx.y.z-6-gf20c721a",
			ExpectFirstMinorRelease: "",
			ExpectPatchRelease:      "",
			ExpectError:             true,
		},
	}

	for i := range tests {
		tc := tests[i]

		t.Run(tc.Name, func(t *testing.T) {
			rv, err := ParseGitVersion(tc.GitVersion)
			if err != nil {
				if !tc.ExpectError {
					t.Fatalf("No error is expected but got: %v", err)
				}
				// Stop and passes this test as error is expected.
				return
			} else if err == nil {
				if tc.ExpectError {
					t.Fatalf("Expect error, but got nil")
				}
			}

			if rv.FirstMinorRelease() != tc.ExpectFirstMinorRelease {
				t.Fatalf("expect first minor release: %s, but got: %s", tc.ExpectFirstMinorRelease, rv.FirstMinorRelease())
			}

			if rv.PatchRelease() != tc.ExpectPatchRelease {
				t.Fatalf("expect patch release: %s, but got: %s", tc.ExpectPatchRelease, rv.PatchRelease())
			}
		})
	}
}
