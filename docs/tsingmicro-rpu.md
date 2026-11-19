 # Tsingmicro RPU

Deploy HAMi-DRA webhook for clusters that already run
[dra-driver-tsingmicro-rpu](https://github.com/tsingmicro/dra-driver-tsingmicro-rpu)
(driver name and DeviceClass `rpu.tsingmicro.com`).

This chart deploys only the mutating/validating webhook and TLS certificates.
The RPU DRA driver and `DeviceClass` (`rpu.tsingmicro.com`) must be deployed
separately via dra-driver-tsingmicro-rpu.

## Prerequisites

1. Kubernetes with DRA enabled and the `DRAConsumableCapacity` feature gate
   (same requirements as the main chart).
2. [dra-driver-tsingmicro-rpu](https://github.com/tsingmicro/dra-driver-tsingmicro-rpu)
   deployed; `DeviceClass` `rpu.tsingmicro.com` must exist. RPU nodes carry the
   label `tsingmicro.com/device=kuiper`.
3. [cert-manager](https://cert-manager.io/docs/installation/) installed.
4. CDI enabled in the container runtime.

## Install

From the published chart:

```bash
helm install hami-dra hami-dra/hami-dra \
  -n hami-system --create-namespace \
  --set 'deviceVendors={tsingmicro}' \
  --set drivers.nvidia.enabled=false \
  --set monitor.enabled=false
```

Upgrade with the same flags:

```bash
helm upgrade hami-dra hami-dra/hami-dra -n hami-system \
  --set 'deviceVendors={tsingmicro}' \
  --set drivers.nvidia.enabled=false \
  --set monitor.enabled=false
```

## Resource semantics

The webhook converts `rpu.tsingmicro.com` extended resources into one
`ResourceClaim` per container:

| Pod resource | Claim |
|---|---|
| `rpu.tsingmicro.com/rpu: N` | `Exactly` count `N`, selector `device.driver == "rpu.tsingmicro.com" && device.attributes["rpu.tsingmicro.com"].type == "rpu"` |
| `rpu.tsingmicro.com/rpumem: 4096` (MiB) | capacity request `memory: 4Gi` |

- **Whole RPU**: request `rpu` without `rpumem`. This is the default driver
  behavior (`allowMultipleAllocations` unset): one claim occupies one RPU.
- **Shared by memory**: with the driver's Consumable Shares in `memory` mode
  (`--feature-gates=ConsumableShares=true --consumable-shares=memory`), the
  driver publishes `memory` as a consumable capacity, and several claims that
  each request part of the memory are co-located on one RPU. The webhook only
  writes the capacity request; sharing requires that driver configuration.

RPUs publish no compute capacity and the driver does not isolate compute, so
there is no core resource and no percentage conversion.

Annotations select devices, in addition to the type selector every claim
carries:

- `rpu.tsingmicro.com/use-rpuuuid` / `rpu.tsingmicro.com/nouse-rpuuuid`
  match the ResourceSlice `uuid` attribute (the RPU mesh ID).
- `rpu.tsingmicro.com/use-rputype` / `rpu.tsingmicro.com/nouse-rputype`
  match the `productName` attribute.
- The label `hami.io/skip-dra: "true"` keeps a pod out of the conversion.

## Example Pod

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: rpu-full
  namespace: workloads   # not the release namespace: the webhook skips that one by default
spec:
  containers:
  - name: c
    image: ubuntu:22.04
    command: ["sleep", "3000"]
    resources:
      limits:
        rpu.tsingmicro.com/rpu: 1          # whole RPU
        # rpu.tsingmicro.com/rpumem: 8192  # MiB, share one RPU by memory instead
```

## Important values

| Value | Recommended | Notes |
|-------|-------------|-------|
| `deviceVendors` | `[tsingmicro]` | Enables conversion of Tsingmicro RPU resources. Add `nvidia` or `ascend` for mixed clusters. |
| `drivers.nvidia.enabled` | `false` | Do not deploy the NVIDIA DRA driver DaemonSet. |
| `monitor.enabled` | `false` | Monitor is NVIDIA-oriented today; disable for RPU-only clusters. |
| `drivers.tsingmicro.deviceClassName` / `driverName` | empty (default) | Optional overrides for webhook config; falls back to `tsingmicroDeviceClassName` / `tsingmicroDraDriverName`. Does not deploy a driver. |
| `certs.certManager.enabled` | `true` (default) | Uses cert-manager for webhook TLS. |

Resource names and driver identifiers use chart defaults in `values.yaml`
(no override needed in most cases):

```yaml
tsingmicroResourceName: "rpu.tsingmicro.com/rpu"
tsingmicroResourceMem: "rpu.tsingmicro.com/rpumem"
tsingmicroDeviceClassName: rpu.tsingmicro.com
tsingmicroDraDriverName: rpu.tsingmicro.com
```

## What gets deployed

| Component | Tsingmicro RPU install | Default NVIDIA install |
|-----------|------------------------|------------------------|
| hami-dra-webhook | Yes (Tsingmicro vendor) | Yes (NVIDIA vendor) |
| cert-manager resources | Yes | Yes |
| NVIDIA DRA driver DaemonSet | No | Yes |
| hami-dra-monitor | No | Yes |
| RPU DRA driver | External (dra-driver-tsingmicro-rpu) | N/A |

## Verify

```bash
kubectl get resourceslice -o wide
kubectl -n workloads get resourceclaim
kubectl -n workloads get pod rpu-full -o jsonpath='{.spec.containers[0].resources claims}'
```

The claim shows count `1` and the CEL selector above; with `rpumem` it also
shows the `memory` capacity request.
