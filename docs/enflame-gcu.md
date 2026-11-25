# Enflame GCU

Deploy HAMi-DRA webhook (and optional monitor) for Enflame GCU clusters that
already run
[enflame-dra-driver](https://github.com/Project-HAMi/enflame-dra-driver).

This chart deploys the mutating/validating webhook, TLS certificates, and
(when enabled) `hami-dra-monitor`. The GCU DRA driver and `DeviceClass`
(`gcu.enflame.com`) must be deployed separately via enflame-dra-driver.

## Prerequisites

1. Kubernetes with DRA and the `DRAConsumableCapacity` feature gate (same bar as the main chart / AMD path).
2. [enflame-dra-driver](https://github.com/Project-HAMi/enflame-dra-driver) deployed; `DeviceClass` `gcu.enflame.com` must exist.
3. [cert-manager](https://cert-manager.io/docs/installation/) installed (or provide your own webhook certs; see the README).
4. `hami-dra-webhook` (and `hami-dra-monitor` if enabled) images built from this revision. Chart defaults still point at a pre-Enflame release tag, so override `webhook.image.*` / `monitor.image.*` with images from this checkout.

## Install

Build images from this revision first (chart defaults are still `v0.2.4` and do not include Enflame conversion):

```bash
make docker-build-webhook docker-build-monitor
```

Then install from the local checkout with those image tags:

```bash
helm install hami-dra ./charts/hami-dra \
  -n hami-system --create-namespace \
  -f ./charts/hami-dra/enflame-values.yaml \
  --set webhook.image.tag=latest \
  --set monitor.image.tag=latest
```

Equivalent `--set` form:

```bash
helm install hami-dra ./charts/hami-dra \
  -n hami-system --create-namespace \
  --set 'deviceVendors={enflame}' \
  --set drivers.nvidia.enabled=false \
  --set monitor.enabled=true \
  --set certs.certManager.enabled=true \
  --set webhook.image.tag=latest \
  --set monitor.image.tag=latest
```

Published Helm chart install is not available yet for Enflame (use a local
checkout and images built from this revision until a release that includes this
vendor ships).

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
gcuResourceCores: "enflame.com/gcu-core"
gcuDeviceClassName: gcu.enflame.com
gcuDraDriverName: gcu.enflame.com
```

enflame-dra-driver advertises both `cores` (0–100) and `memory` capacities with
a whole-card-only RequestPolicy (`Default` / sole `ValidValues` = full card).

- Request only `enflame.com/gcu` (count): the claim uses the driver’s Default
  capacities, so the scheduler records full-card `ConsumedCapacity` (needed by
  `hami-dra-monitor` allocation gauges).
- Optional `enflame.com/gcu-core` is the HAMi 0–100 scale (same numbers the
  driver publishes as capacity `cores`); the webhook leaves those values as-is
  and does not use `referenceComputeUnits` the way Hygon/AMD percentage cores
  do. Optional `enflame.com/gcu-memory` is MiB and is converted to bytes like
  other vendors. Partial optional requests round up to a full card and still
  allocate; Kubernetes rejects only requests that exceed all valid values.

The label `hami.io/skip-dra: "true"` keeps a Pod out of DRA conversion (same as
other vendors). `webhook.config.resourceClaimTemplate` from the main chart also
applies when you need ResourceClaimTemplate instead of ResourceClaim.

### UUID selection

Use a UUID advertised by your own driver when you need a specific GCU:

| Annotation | Effect |
|------------|--------|
| `enflame.com/use-gpuuuid` | Restrict claim to listed UUIDs |
| `enflame.com/nouse-gpuuuid` | Exclude listed UUIDs |

## Example Pod

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: gcu-job
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

`hami-dra-monitor` reads ResourceSlices and claim `ConsumedCapacity` generically
(`cores` / `memory`). With enflame-dra-driver’s whole-card RequestPolicy, a
count-only claim shows non-zero Core/Memory allocated gauges (for example
`cores=100` and full `memory`) under the current metric names. Enable monitor
via `enflame-values.yaml` (`monitor.enabled: true`).
