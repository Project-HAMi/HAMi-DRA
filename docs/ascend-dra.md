# Ascend DRA (HAMivNPUCore)

The webhook can convert HAMi Ascend device-plugin resources into DRA
`ResourceClaim`s for [ascend-dra-driver](https://github.com/FouoF/ascend-dra-driver)
(`ascend.project-hami.io`).

This path covers **HAMivNPUCore** consumable-capacity sharing only. Traditional
full-card / template vNPU allocation is a separate DeviceClass and allocation
model; do not OR both types in one CEL selector.

## Prerequisites

- Kubernetes >= 1.34 with `DynamicResourceAllocation` and `DRAConsumableCapacity`
- `ascend-dra-driver` installed with HAMivNPUCore enabled (chart default)
- CDI enabled and Ascend Docker Runtime on NPU nodes (set `runtimeClassName` on the Pod if needed)
- cert-manager for the webhook

Install the driver separately. This chart does not ship the kubelet plugin.

## Install the webhook

```bash
helm install hami-dra ./charts/hami-dra \
  -n hami-system --create-namespace \
  -f ./charts/hami-dra/ascend-values.yaml
```

One webhook converts every HAMi `vnpus.configs` chip (910A, 910B2, 910B3,
910B4-1, 910B4, 310P, 910C). It matches `huawei.com/<commonWord>` plus
`-memory` / `-core`, and UUID annotations `hami.io/use-<commonWord>-uuid`
(same keys as HAMi). Restrict the list with `ascendDevices` in values if
needed.

## Example Pod

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: ascend-share
  annotations:
    hami.io/use-nputype: "310P3"
    hami.io/use-Ascend310P-uuid: "<device-uuid>"
spec:
  containers:
    - name: app
      image: busybox
      resources:
        limits:
          huawei.com/Ascend310P: 1
          huawei.com/Ascend310P-memory: "1024"
          huawei.com/Ascend310P-core: "50"
```

910B3 uses `huawei.com/Ascend910B3` and `hami.io/use-Ascend910B3-uuid`.
`hami.io/use-nputype` is compared to ResourceSlice `productName` (for
example `310P3`), not the HAMi commonWord.

The webhook:

- Creates a `ResourceClaim` with request name `npu`, DeviceClass
  `hami-vnpu-core.project-hami.io`, and capacity `memory` / `cores`
- Removes the HAMi extended resources from the container

## DeviceClass

Claims use DeviceClass `hami-vnpu-core.project-hami.io` with:

```
device.driver == "ascend.project-hami.io" &&
device.attributes["ascend.project-hami.io"].type == "HAMivNPUCore"
```

`ascend-dra-driver` should create this class. The webhook chart only references the name.

## Later: mixed nodes and traditional vNPU

Some nodes can publish `HAMivNPUCore` and others `NPU` if the plugin runs with
different feature gates (separate DaemonSets). Scheduling unification should use
`spec.devices.requests[].firstAvailable` (feature gate `DRAPrioritizedList`):

1. First subrequest: the current HAMivNPUCore `Exactly` request (capacity + this DeviceClass)
2. Later subrequests: traditional vNPU DeviceClasses with their own opaque template config

That is prioritized fallback (first subrequest that fits cluster-wide wins), not
an unweighted OR. Claim construction is centralized in
`DRADeviceConfig.NewPrimaryDeviceRequest()` so the first subrequest can be reused.
