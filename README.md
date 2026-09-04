# HAMi DRA Webhook

A Kubernetes mutating webhook that converts GPU device resources to Dynamic Resource Allocation (DRA) ResourceClaims.

## Overview

This webhook automatically transforms Pod specifications that request GPU resources (e.g., `nvidia.com/gpu`) into DRA ResourceClaims, enabling dynamic resource allocation for GPU workloads in Kubernetes.

## Features

- **Automatic Resource Conversion**: Converts GPU resource requests to ResourceClaims
- **Resource Cleanup**: Automatically removes GPU resources from Pod specs and creates corresponding ResourceClaims
- **Annotation Support**: Supports device selection via Pod annotations (UUID, device type)
- **Metrics Monitoring**: Optional monitor component that collects and exposes GPU resource metrics via Prometheus

## Installation

### Prerequisites

- Kubernetes version >= 1.34 with DRA Consumable Capacity [featuregate](https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/) enabled
- [CDI](https://github.com/cncf-tags/container-device-interface?tab=readme-ov-file#how-to-configure-cdi) must be enabled in the underlying container runtime (such as containerd or CRI-O).
- NVIDIA GPU Driver 440 or later

### Configure and install with Helm

You need [cert-manager](https://cert-manager.io/docs/installation/) installed before installing the webhook. If you don't want to use cert-manager, set `certs.certManager.enabled=false` and provide your own certificate via `certs.custom.crt` and `certs.custom.key`.

Add the HAMi-DRA Helm repository:
```bash
helm repo add hami-dra https://project-hami.github.io/HAMi-DRA
helm repo update
```

Install the chart:
```bash
helm install hami-dra hami-dra/hami-dra
```

To upgrade to the latest version in the future:
```bash
helm repo update
helm upgrade hami-dra hami-dra/hami-dra
```

If you are not using gpu-operator provided containerd drivers, you can use the following command to install the webhook:
```bash
helm install hami-dra hami-dra/hami-dra \
  --set drivers.nvidia.containerDriver=false
```


Then [use the same as hami](https://project-hami.io/zh/docs/userguide/nvidia-device/examples/use-exclusive-card/).

### Hygon DCU

For clusters running Hygon DCU with [k8s-hcu-dra-driver](https://github.com/HYGON-AI/k8s-hcu-dra-driver), see [docs/hygon-dcu.md](./docs/hygon-dcu.md).

## Configuration

### Device Resources

Configure device resources via `--set` flags or a custom `values.yaml`. The default resource names are:

```yaml
resourceName: "nvidia.com/gpu"
resourceMem: "nvidia.com/gpumem"
resourceCores: "nvidia.com/gpucores"
```

### Monitor Component

The monitor component is an optional feature that collects and exposes GPU resource metrics via Prometheus. It is enabled by default.

**Quick Start**:

Set the monitor service to NodePort so we can access it outside the cluster:
```yaml
monitor:
  enabled: true
  service:
    type: NodePort
    nodePort:
      metrics: 31995
```

Access metrics:
```bash
# With NodePort
curl http://<node-ip>:31995/metrics
```

You will see metrics like this:


![metrics.png](./docs/metrics.png)

For detailed configuration, metrics documentation, and Prometheus integration, see [MONITOR.md](./docs/MONITOR.md).
