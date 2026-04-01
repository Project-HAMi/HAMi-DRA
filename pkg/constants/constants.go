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

// Package constants contains shared constants used across the project.
package constants

const (
	// TODO: Decouple these annotation keys from the nvidia.com prefix so fake driver
	// can use driver-specific or generic keys without reusing NVIDIA naming.
	UseUUIDAnnotation   = "nvidia.com/use-gpuuuid"
	NoUseUUIDAnnotation = "nvidia.com/nouse-gpuuuid"
	UseTypeAnnotation   = "nvidia.com/use-gputype"
	NoUseTypeAnnotation = "nvidia.com/nouse-gputype"

	NvidiaDraDriver  = "hami-core-gpu.project-hami.io"
	NvidiaDeviceType = "hami-gpu"

	DraLabel = "hami.io/dra"

	DeviceAttributeUUID         = "uuid"
	DeviceAttributeArchitecture = "architecture"
	DeviceAttributeBrand        = "brand"
	DeviceAttributeProductName  = "productName"
	DeviceCapacityCores         = "cores"
	DeviceCapacityMemory        = "memory"
)
