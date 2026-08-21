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

package config

import (
	"math"
	"regexp"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
)

// Quantities with huge decimal exponents parse fine but make every later
// accessor (AsInt64, Value, String) grind through a big-decimal slow path
// for hundreds of milliseconds or worse, which stalls fuzz workers.
var hugeExponent = regexp.MustCompile(`[eE][-+]?[0-9]{4,}`)

// parseReasonableQuantity parses qtyStr and skips inputs outside the domain
// the converters are written for: pod resource limits are validated as
// non-negative by the API server, and quantities whose value does not fit an
// int64 are not meaningful device requests.
func parseReasonableQuantity(t *testing.T, qtyStr string) resource.Quantity {
	if hugeExponent.MatchString(qtyStr) {
		t.Skip()
	}
	qty, err := resource.ParseQuantity(qtyStr)
	if err != nil {
		t.Skip()
	}
	if v, ok := qty.AsInt64(); !ok || v < 0 {
		t.Skip()
	}
	return qty
}

func FuzzConvertCores(f *testing.F) {
	f.Add("100", int64(0))
	f.Add("50", int64(60))
	f.Add("1", int64(1))
	f.Fuzz(func(t *testing.T, qtyStr string, refUnits int64) {
		qty := parseReasonableQuantity(t, qtyStr)
		cfg := &DRADeviceConfig{ReferenceComputeUnits: refUnits}
		converted, err := cfg.ConvertCores(qty)
		if err != nil {
			return
		}
		if refUnits > 0 && qty.Value() > 0 && converted.Value() < 1 {
			t.Errorf("ConvertCores(%s, ref=%d) = %s, expected at least 1", qtyStr, refUnits, converted.String())
		}
	})
}

func FuzzConvertMemory(f *testing.F) {
	f.Add("128")
	f.Add("0")
	f.Add("1Ki")
	f.Fuzz(func(t *testing.T, qtyStr string) {
		qty := parseReasonableQuantity(t, qtyStr)
		cfg := &DRADeviceConfig{}
		converted := cfg.ConvertMemory(qty)
		// The MiB-to-bytes invariant only holds while the product fits in
		// int64; ConvertMemory silently overflows beyond that.
		if qty.Value() > math.MaxInt64/(1024*1024) {
			t.Skip()
		}
		if want := qty.Value() * 1024 * 1024; converted.Value() != want {
			t.Errorf("ConvertMemory(%s) = %d, want %d", qtyStr, converted.Value(), want)
		}
	})
}
