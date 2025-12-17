# HAMi DRA Webhook

A Kubernetes mutating webhook that converts GPU device resources to Dynamic Resource Allocation (DRA) ResourceClaims.

## Overview

This webhook automatically transforms Pod specifications that request GPU resources (e.g., `nvidia.com/gpu`) into DRA ResourceClaims, enabling dynamic resource allocation for GPU workloads in Kubernetes.

## Features

- **Automatic Resource Conversion**: Converts GPU resource requests to ResourceClaims
- **Resource Cleanup**: Automatically removes GPU resources from Pod specs and creates corresponding ResourceClaims
- **Annotation Support**: Supports device selection via Pod annotations (UUID, device type)

## Installation

### Prerequisites

- Kubernetes version >= 1.34 with DRA Consumable Capacity [featuregate](https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/) enabled
- [CDI](https://github.com/cncf-tags/container-device-interface?tab=readme-ov-file#how-to-configure-cdi) must be enabled in the underlying container runtime (such as containerd or CRI-O).
- NVIDIA GPU Driver 440 or later

### Configure and install with Helm

You need to ensure [cert-manager](https://cert-manager.io/docs/installation/) is installed before installing the webhook.

Then you can install the webhook with the following command:
```bash
helm install hami-dra-webhook ./chart/hami-dra-webhook
```

If you are not using gpu-operator provided containered drivers, you can use the following command to install the webhook:
```bash
helm install hami-dra-webhook ./chart/hami-dra-webhook \
--set drivers.nvidia.containerDriver=false
```

Then [use the same as hami](https://project-hami.io/zh/docs/userguide/nvidia-device/examples/use-exclusive-card/).

## Configuration

Configure device resources in `chart/hami-dra-webhook/values.yaml`:

```yaml
resourceName: "nvidia.com/gpu"
resourceMem: "nvidia.com/gpumem"
resourceCores: "nvidia.com/gpucores"
```
