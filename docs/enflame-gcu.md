# Enflame GCU

Deploy HAMi-DRA webhook for Enflame GCU clusters that already run
[enflame-dra-driver](https://github.com/Project-HAMi/enflame-dra-driver).

This chart deploys only the mutating/validating webhook and TLS certificates.
The GCU DRA driver and `DeviceClass` (`gcu.enflame.com`) must be deployed
separately via enflame-dra-driver.

## Prerequisites

1. Kubernetes with DRA enabled (same requirements as the main chart).
2. [enflame-dra-driver](https://github.com/Project-HAMi/enflame-dra-driver) deployed; `DeviceClass` `gcu.enflame.com` must exist.
3. [cert-manager](https://cert-manager.io/docs/installation/) installed.
4. `hami-dra-webhook` image built and reachable from all nodes (override `webhook.image.*` if not using the default registry).

## Install

From a local checkout:

```bash
helm install hami-dra ./charts/hami-dra \
  -n hami-system --create-namespace \
  -f ./charts/hami-dra/enflame-values.yaml
```

Equivalent `--set` form:

```bash
helm install hami-dra ./charts/hami-dra \
  -n hami-system --create-namespace \
  --set 'deviceVendors={enflame}' \
  --set drivers.nvidia.enabled=false \
  --set monitor.enabled=false
```

From the published chart:

```bash
helm install hami-dra hami-dra/hami-dra \
  -n hami-system --create-namespace \
  --set 'deviceVendors={enflame}' \
  --set drivers.nvidia.enabled=false \
  --set monitor.enabled=false
```

## Important values

| Value | Recommended | Notes |
|-------|-------------|-------|
| `deviceVendors` | `[enflame]` | Enables conversion of Enflame GCU resources. Add other vendors for mixed clusters. |
| `drivers.nvidia.enabled` | `false` | Do not deploy the NVIDIA DRA driver DaemonSet. |
| `drivers.enflame.deviceClassName` / `driverName` | empty (default) | Optional overrides for webhook config; falls back to `gcuDeviceClassName` / `gcuDraDriverName`. Does not deploy a driver. |
| `monitor.enabled` | `true` (in `enflame-values.yaml`) | Reads `gcu.enflame.com` ResourceSlices as-is. Allocation gauges need claim `ConsumedCapacity` (from whole-card Default/`ValidValues` RequestPolicy). |

GCU resource names and driver identifiers use chart defaults in `values.yaml`:

```yaml
gcuResourceName: "enflame.com/gcu"
gcuResourceMem: "enflame.com/gcu-memory"
gcuResourceCores: "enflame.com/gcu-cores"
gcuDeviceClassName: gcu.enflame.com
gcuDraDriverName: gcu.enflame.com
```

enflame-dra-driver advertises both `cores` (0–100) and `memory` capacities with
a whole-card RequestPolicy. Whole-card Pods typically request `enflame.com/gcu`
alone; the driver fills default ConsumedCapacity.

### UUID / product selection

Same annotation style as HAMi Enflame:

| Annotation | Effect |
|------------|--------|
| `enflame.com/use-gpuuuid` | Restrict claim to listed UUIDs |
| `enflame.com/nouse-gpuuuid` | Exclude listed UUIDs |
| `enflame.com/use-gcutype` | Match `productName` (e.g. `Enflame S60`) |
| `enflame.com/nouse-gcutype` | Exclude product names |

## Example Pod

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: gcu-job
  annotations:
    enflame.com/use-gpuuuid: TR6W18220404
spec:
  containers:
  - name: app
    image: ubuntu:24.04
    command: ["sleep", "3600"]
    resources:
      limits:
        enflame.com/gcu: 1
```

The webhook strips `enflame.com/gcu` and creates a ResourceClaim against
`DeviceClass` `gcu.enflame.com` with request name `gcu`.

## What gets deployed

| Component | Enflame GCU install | Default NVIDIA install |
|-----------|---------------------|------------------------|
| hami-dra-webhook | Yes (Enflame vendor) | Yes (NVIDIA vendor) |
| cert-manager resources | Yes | Yes |
| NVIDIA DRA driver DaemonSet | No | Yes |
| hami-dra-monitor | Yes | Yes |
| GCU DRA driver | External (enflame-dra-driver) | N/A |

## Monitor notes

Verified against a live `gcu.enflame.com` ResourceSlice: device UUID / brand /
productName / memory limit show up under the existing `GPUDevice*` metrics.
Core limit/allocated stay `0` (driver has no `cores` capacity yet).
