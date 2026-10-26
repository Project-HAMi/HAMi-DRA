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
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExitError(t *testing.T) {
	fatal := errors.New("publish failed")
	shutdown := errors.New("shutdown failed")

	assert.NoError(t, exitError(context.Canceled, nil), "signal shutdown is clean")
	assert.NoError(t, exitError(nil, nil))
	assert.ErrorIs(t, exitError(context.Canceled, shutdown), shutdown)

	err := exitError(fatal, nil)
	assert.ErrorIs(t, err, fatal, "a fatal cause must make the process exit non-zero")
	err = exitError(fatal, shutdown)
	assert.ErrorIs(t, err, fatal)
	assert.ErrorIs(t, err, shutdown)
}
