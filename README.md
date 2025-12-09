# HAMi DRA Webhook

A Kubernetes mutating webhook that converts GPU device resources to Dynamic Resource Allocation (DRA) ResourceClaims.

## Overview

This webhook automatically transforms Pod specifications that request GPU resources (e.g., `nvidia.com/gpu`) into DRA ResourceClaims, enabling dynamic resource allocation for GPU workloads in Kubernetes.

## Features

- **Automatic Resource Conversion**: Converts GPU resource requests to ResourceClaims
- **Multi-Device Support**: Supports NVIDIA GPU, MLU, Hygon DCU, Metax sGPU, Enflame VGCU, and Kunlun XPU
- **Resource Cleanup**: Automatically removes GPU resources from Pod specs and creates corresponding ResourceClaims
- **Annotation Support**: Supports device selection via Pod annotations (UUID, device type)

## Quick Start

### Deploy with Helm

```bash
helm install hami-dra-webhook ./chart/hami-dra-webhook
```

## Configuration

Configure device resources in `chart/hami-dra-webhook/values.yaml`:

```yaml
resourceName: "nvidia.com/gpu"
resourceMem: "nvidia.com/gpumem"
resourceCores: "nvidia.com/gpucores"
```

## License

Copyright 2025 The HAMi Authors.

Licensed under the Apache License, Version 2.0.
