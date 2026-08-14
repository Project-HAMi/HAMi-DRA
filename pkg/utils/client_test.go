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

import (
	"os"
	"path/filepath"
	"testing"
)

const fakeKubeconfig = `apiVersion: v1
kind: Config
clusters:
- name: fake
  cluster:
    server: https://127.0.0.1:6443
contexts:
- name: fake
  context:
    cluster: fake
    user: fake
current-context: fake
users:
- name: fake
  user: {}
`

func TestNewClientWithRateLimitAppliesQPSAndBurst(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(path, []byte(fakeKubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", path)

	client, err := NewClientWithRateLimit(100, 150)
	if err != nil {
		t.Fatalf("NewClientWithRateLimit failed: %v", err)
	}
	if client.config.QPS != 100 || client.config.Burst != 150 {
		t.Errorf("got QPS=%v Burst=%v, want 100 and 150", client.config.QPS, client.config.Burst)
	}
}
