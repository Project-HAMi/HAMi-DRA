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

package utils

import "testing"

func TestOwnsClaimName(t *testing.T) {
	long := "averyveryverylongnamespacename-averyveryverylongpodname-container-abcdef12"
	for _, tc := range []struct {
		name, ns, owner string
		want            bool
	}{
		{"default-pod-ctr", "default", "pod", true},
		{"default-pod-ctr-amd", "default", "pod", true},
		{"shared-claim", "default", "pod", false},
		{"default-other-ctr", "default", "pod", false},
		{"default-pod", "default", "pod", false},
		{long, "averyveryverylongnamespacename", "averyveryverylongpodname", true},
		{"other-" + long, "averyveryverylongnamespacename", "averyveryverylongpodname", false},
	} {
		if got := OwnsClaimName(tc.name, tc.ns, tc.owner); got != tc.want {
			t.Errorf("OwnsClaimName(%q,%q,%q)=%v want %v", tc.name, tc.ns, tc.owner, got, tc.want)
		}
	}
}
