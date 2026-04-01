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

package configmapgen

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewHandlerServesDefaultsAndIndex(t *testing.T) {
	handler, err := NewHandler()
	if err != nil {
		t.Fatalf("NewHandler returned error: %v", err)
	}

	indexReq := httptest.NewRequest(http.MethodGet, "/", nil)
	indexResp := httptest.NewRecorder()
	handler.ServeHTTP(indexResp, indexReq)
	if indexResp.Code != http.StatusOK {
		t.Fatalf("expected index status 200, got %d", indexResp.Code)
	}
	if body := indexResp.Body.String(); !strings.Contains(body, "Fake DRA ConfigMap Generator") {
		t.Fatalf("expected index page content, got %q", body)
	}

	apiReq := httptest.NewRequest(http.MethodGet, "/api/defaults", nil)
	apiResp := httptest.NewRecorder()
	handler.ServeHTTP(apiResp, apiReq)
	if apiResp.Code != http.StatusOK {
		t.Fatalf("expected defaults status 200, got %d", apiResp.Code)
	}

	var payload BootstrapData
	if err := json.Unmarshal(apiResp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode defaults payload: %v", err)
	}
	if len(payload.Form.Groups) == 0 {
		t.Fatalf("expected at least one default group")
	}
	if payload.Form.Groups[0].DeviceCount <= 0 {
		t.Fatalf("expected positive group device count, got %d", payload.Form.Groups[0].DeviceCount)
	}
}
