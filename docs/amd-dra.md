# AMD GPUs

HAMi-DRA converts `amd.com/gpu`, `amd.com/gpumem`, `amd.com/gpumem-percentage` and `amd.com/gpucores` requests into ResourceClaims for the AMD DRA driver ([amd-dra-driver](https://github.com/Project-HAMi/amd-dra-driver), driver name `gpu.amd.com`). With `drivers.amd.enabled` the chart installs that driver too, with the `GPUSharing` gate on, so several pods can take slices of one GPU: each gets its own CUs (`HSA_CU_MASK`) and its own memory limit.

## Prerequisites

1. Kubernetes with DRA and the `DRAConsumableCapacity` feature gate (on by default since 1.36), and CDI in the container runtime.
2. The `amdgpu` driver **bound** to the GPU, not only loaded. Check that `lspci -k` shows `amdgpu` in use and that `/dev/kfd` exists. An Instinct MI325X needed kernel 6.14 with `amdgpu-dkms` 6.16.13: the in-tree driver of kernels 6.8 and 7.0 did not bind to it, and that DKMS version does not build on kernels 6.17 and 7.0. The kernel memory cap (dmem cgroup) needs kernel 6.14 or newer.
3. [cert-manager](https://cert-manager.io/docs/installation/) for the webhook certificate.
4. A namespace that allows privileged pods: the driver runs privileged.
5. One mode per node. A node handed to the DRA driver runs no HAMi AMD device plugin, because both would hand out the same GPU. Label the node:

   ```bash
   kubectl label node <node> hami.io/amd-mode=dra
   ```

   The driver DaemonSet also needs the `amd.com/gpu.family` label that the AMD GPU labeller sets. Without the labeller, add it by hand (`kubectl label node <node> amd.com/gpu.family=AI`) or override `drivers.amd.daemonSet.affinity`.

## Install

```bash
helm install hami-dra hami-dra/hami-dra -n hami-dra --create-namespace \
  --set "deviceVendors={amd}" \
  --set drivers.nvidia.enabled=false \
  --set drivers.amd.enabled=true
```

Add `nvidia`, `hygon` or `ascend` to `deviceVendors` for a mixed cluster.

## `amdReferenceComputeUnits`

`amd.com/gpucores` is a percentage; the claim asks for absolute CUs. With `amdReferenceComputeUnits` unset (0) the webhook reads the `computeUnits` of the published GPUs and uses it when they all have the same count, so a cluster of one GPU model needs no setting. Set it when GPUs of different sizes share the cluster (the webhook then rejects `gpucores` requests and lists the sizes), or for Volcano jobs, which do not read the ResourceSlices. The value is the `computeUnits` capacity of one GPU:

```bash
kubectl get resourceslice -o jsonpath='{range .items[*].spec.devices[*]}{.name}{" "}{.capacity.computeUnits.value}{"\n"}{end}'
```

An MI325X has 304, an MI300X 304, an RX 9060 XT 32. A configured value always wins over the published sizes. Requests without `gpucores` never need it.

## Use

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: slice
spec:
  containers:
  - name: c
    image: rocm/pytorch:latest
    command: ["sleep", "3000"]
    resources:
      limits:
        amd.com/gpu: 1
        amd.com/gpumem: 20000     # MiB
        amd.com/gpucores: 25      # percent of amdReferenceComputeUnits
```

- `amd.com/gpumem-percentage: 10` takes a percentage of the GPU memory instead of MiB.
- Without `gpumem` and `gpucores` the pod gets the whole GPU and no limits.
- Annotations select GPUs: `amd.com/use-gpu-uuid`, `amd.com/nouse-gpu-uuid`, `amd.com/use-gputype` and `amd.com/nouse-gputype`. A type matches any part of the product name, for example `MI325X` for `AMD_Instinct_MI325X` (driver 0.4.1 or newer; older drivers published `AMD_Radeon_Graphics` for the MI325X).
- `hami.io/skip-dra` keeps a pod out of the conversion.

## Verify

```bash
kubectl -n hami-dra get pods
kubectl get resourceslice -o yaml | grep -E "productName|brand|computeUnits|memory:"
kubectl get resourceclaim
kubectl exec <pod> -- env | grep -E "HSA_CU_MASK|HIP_DEVICE_MEMORY_LIMIT"
```

A slice pod shows `HSA_CU_MASK=0:0-75` (25 percent of 304) and `HIP_DEVICE_MEMORY_LIMIT_0=20000m`. The monitor labels its metrics with `devicebrand` and `deviceproductname`.

## Troubleshooting

- **`unsupported device vendor "amd"` and the webhook restarts.** The webhook image is older than the AMD support. Use chart 0.2.4 or newer.
- **The ResourceSlice is empty, driver log says `Discovered 0 AMD GPU devices`.** `amdgpu` is loaded but not bound to the GPU; see the prerequisites.
- **A pod with `gpucores` is rejected.** The published GPUs differ in `computeUnits` or none is published yet; set `amdReferenceComputeUnits`.
- **A pod stays Pending.** The GPU has too few free CUs or too little free memory for the request, or a type or UUID annotation matches no GPU. A whole-GPU claim waits until every slice on that GPU is gone.
- **The driver pod does not start.** The node needs the labels `hami.io/amd-mode=dra` and `amd.com/gpu.family`.
