# Hygon DCU

Deploy HAMi-DRA webhook for Hygon DCU clusters that already run [k8s-dcu-dra-driver](https://github.com/HYGON-AI/k8s-dcu-dra-driver).

This chart deploys only the mutating/validating webhook and TLS certificates. The DCU DRA driver and `DeviceClass` (`dra.hygon.com`) must be deployed separately via k8s-dcu-dra-driver.

## Prerequisites

1. Kubernetes with DRA enabled (same requirements as the main chart).
2. [k8s-dcu-dra-driver](https://github.com/HYGON-AI/k8s-dcu-dra-driver) deployed; `DeviceClass` `dra.hygon.com` must exist.
3. [cert-manager](https://cert-manager.io/docs/installation/) installed.
4. `hami-dra-webhook` image built and reachable from all nodes (override `webhook.image.*` if not using the default registry).

## Install

From a local checkout:

```bash
helm install hami-dra ./charts/hami-dra \
  -n hami-system --create-namespace \
  --set deviceVendor=hygon \
  --set drivers.nvidia.enabled=false \
  --set monitor.enabled=false
```

From the published chart:

```bash
helm install hami-dra hami-dra/hami-dra \
  -n hami-system --create-namespace \
  --set deviceVendor=hygon \
  --set drivers.nvidia.enabled=false \
  --set monitor.enabled=false
```

Upgrade with the same flags:

```bash
helm upgrade hami-dra ./charts/hami-dra -n hami-system \
  --set deviceVendor=hygon \
  --set drivers.nvidia.enabled=false \
  --set monitor.enabled=false
```

## Important values

| Value | Recommended | Notes |
|-------|-------------|-------|
| `deviceVendor` | `hygon` | Switches webhook to Hygon DCU resource names and `dra.hygon.com` driver. |
| `drivers.nvidia.enabled` | `false` | Do not deploy the NVIDIA DRA driver DaemonSet. |
| `drivers.dcu.deviceClassName` / `driverName` | empty (default) | Optional overrides for webhook config; falls back to `dcuDeviceClassName` / `dcuDraDriverName`. Does not deploy a driver. |
| `monitor.enabled` | `false` | Monitor is NVIDIA-oriented today; disable for DCU-only clusters. |
| `certs.certManager.enabled` | `true` (default) | Uses cert-manager for webhook TLS. |

DCU resource names and driver identifiers use chart defaults in `values.yaml` (no override needed in most cases):

```yaml
dcuResourceName: "hygon.com/dcunum"
dcuResourceMem: "hygon.com/dcumem"
dcuResourceCores: "hygon.com/dcucores"
dcuDeviceClassName: dra.hygon.com
dcuDraDriverName: dra.hygon.com
```

### Fractional compute (`hygon.com/dcucores`)

When Pods request compute as a percentage via `hygon.com/dcucores`, set `dcuReferenceComputeUnits` to one card's total compute units from the cluster:

```bash
kubectl get resourceslice -o yaml
```

Example:

```bash
helm upgrade hami-dra ./charts/hami-dra -n hami-system \
  --set deviceVendor=hygon \
  --set drivers.nvidia.enabled=false \
  --set monitor.enabled=false \
  --set dcuReferenceComputeUnits=128
```

For whole-card requests only (`hygon.com/dcunum` without `dcumem` / `dcucores`), leave `dcuReferenceComputeUnits` at `0` (default).

## What gets deployed

| Component | Hygon DCU install | Default NVIDIA install |
|-----------|-------------------|------------------------|
| hami-dra-webhook | Yes (Hygon vendor) | Yes (NVIDIA vendor) |
| cert-manager resources | Yes | Yes |
| NVIDIA DRA driver DaemonSet | No | Yes |
| hami-dra-monitor | No | Yes |
| DCU DRA driver | External (k8s-dcu-dra-driver) | N/A |
