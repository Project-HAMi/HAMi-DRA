# Monitor Component Documentation

The monitor component is an optional feature of HAMi DRA Webhook that collects and exposes GPU resource metrics via Prometheus.

## Overview

The monitor component watches Kubernetes ResourceSlice and ResourceClaim resources, maintains an in-memory cache of GPU device allocations, and exposes Prometheus metrics for monitoring GPU resource usage across the cluster.


## Installation

The monitor component is enabled by default when installing the Helm chart. To disable it:

```bash
helm install hami-dra ./charts/hami-dra \
--set monitor.enabled=false
```

## Configuration

### Basic Configuration

Configure the monitor in `charts/hami-dra/values.yaml`:

```yaml
monitor:
  enabled: true
  replicas: 1
  logLevel: 2
  metricsBindAddress: ":8080"
  healthProbeBindAddress: ":8000"
  kubeAPIQPS: 40.0
  kubeAPIBurst: 60
  collectInterval: "30s"
```

**Configuration Parameters**:

- `enabled`: Enable or disable the monitor component (default: `true`)
- `replicas`: Number of monitor pod replicas (default: `1`)
- `logLevel`: Log verbosity level (default: `2`)
- `metricsBindAddress`: Address and port for metrics endpoint (default: `:8080`)
- `healthProbeBindAddress`: Address and port for health probe endpoints (default: `:8000`)
- `kubeAPIQPS`: QPS limit for Kubernetes API client (default: `40.0`)
- `kubeAPIBurst`: Burst limit for Kubernetes API client (default: `60`)
- `collectInterval`: Interval for metrics collection (default: `30s`)

### Service Configuration

The monitor service can be configured to use different service types depending on your access requirements.

#### ClusterIP (Default)

Use ClusterIP for internal cluster access:

```yaml
monitor:
  enabled: true
  service:
    type: ClusterIP
```

Access metrics via port-forward:
```bash
kubectl port-forward svc/hami-dra-monitor 8080:8080 -n <namespace>
curl http://localhost:8080/metrics
```

#### NodePort

Use NodePort to expose metrics outside the cluster:

**With specified ports**:
```yaml
monitor:
  enabled: true
  service:
    type: NodePort
    nodePort:
      metrics: 30080  # NodePort for metrics endpoint
```

**With auto-assigned ports**:
```yaml
monitor:
  enabled: true
  service:
    type: NodePort
    nodePort:
      metrics: ""  # Kubernetes will assign a random port
```

Access metrics via NodePort:
```bash
# Get the NodePort
kubectl get svc hami-dra-monitor -n <namespace> -o jsonpath='{.spec.ports[?(@.name=="metrics")].nodePort}'

# Access metrics
curl http://<node-ip>:<nodeport>/metrics
```

#### LoadBalancer

Use LoadBalancer for cloud provider load balancer integration:

```yaml
monitor:
  enabled: true
  service:
    type: LoadBalancer
```

## Exposed Metrics

The monitor exposes the following Prometheus metrics:

| Metrics | Description | Example |
| --- | --- | --- |
| hami_dra_gpu_core_limit_ratio | Device core limit for a certain GPU | `{device_index="0",device_name="hami-gpu-1",device_type="NVIDIA A30",device_uuid="GPU-a4d27439-566b-841c-428f-d87e73e4134e",node="a30-node"}` 1 |
| hami_dra_gpu_memory_limit_bytes | Device memory limit for a certain GPU | `{device_index="0",device_name="hami-gpu-1",device_type="NVIDIA A30",device_uuid="GPU-a4d27439-566b-841c-428f-d87e73e4134e",node="a30-node"}` 2.5769803776e+10 |
| hami_dra_gpu_core_allocated_ratio | Device core allocated for a certain GPU | `{device_index="0",device_name="hami-gpu-1",device_type="NVIDIA A30",device_uuid="GPU-a4d27439-566b-841c-428f-d87e73e4134e",node="a30-node"}` 0.3 |
| hami_dra_gpu_memory_allocated_bytes | Device memory allocated for a certain GPU | `{device_index="0",device_name="hami-gpu-1",device_type="NVIDIA A30",device_uuid="GPU-a4d27439-566b-841c-428f-d87e73e4134e",node="a30-node"}` 1.073741824e+10 |
| hami_dra_vgpu_core_allocated_ratio | vGPU Device core allocated for a container | `{device_uuid="GPU-a4d27439-566b-841c-428f-d87e73e4134e",namespace="default",node="a30-node",pod="pod-0"}` 0.3 |
| hami_dra_vgpu_memory_allocated_bytes | vGPU Device memory allocated for a container | `{device_uuid="GPU-a4d27439-566b-841c-428f-d87e73e4134e",namespace="default",node="a30-node",pod="pod-0"}` 1.073741824e+10 |

### Legacy metrics

The old metric names are still exported while `--legacy-metrics` is enabled (the default); set `legacyMetrics: false` in the Helm values to export only the new names.

| Old metric | New metric |
| --- | --- |
| GPUDeviceCoreLimit | hami_dra_gpu_core_limit_ratio |
| GPUDeviceMemoryLimit | hami_dra_gpu_memory_limit_bytes |
| GPUDeviceCoreAllocated | hami_dra_gpu_core_allocated_ratio |
| GPUDeviceMemoryAllocated | hami_dra_gpu_memory_allocated_bytes |
| vGPUDeviceCoreAllocated | hami_dra_vgpu_core_allocated_ratio |
| vGPUDeviceMemoryAllocated | hami_dra_vgpu_memory_allocated_bytes |

Legacy metrics keep their original labels (`nodeid`, `deviceuuid`, `deviceidx`, `devicename`, `devicebrand`, `deviceproductname`, `podnamespace`, `podname`) and units: memory in MB and cores on a 0-100 scale.

## Endpoints

### Metrics Endpoint

- **Path**: `/metrics`
- **Port**: `8080` (configurable via `metricsBindAddress`)
- **Format**: Prometheus text format
- **Access**: `http://<service-address>:8080/metrics`

### Health Check Endpoints

- **Liveness Probe**: `/healthz` on port `8000`
- **Readiness Probe**: `/readyz` on port `8000`
- **Access**: `http://<service-address>:8000/healthz` or `/readyz`

The readiness probe returns `200 OK` when the cache is synced and ready, `503 Service Unavailable` otherwise.

## Prometheus Integration

### Service Discovery Configuration

To automatically discover and scrape metrics from the monitor, add the following to your Prometheus configuration:

```yaml
scrape_configs:
  - job_name: 'hami-dra-monitor'
    kubernetes_sd_configs:
      - role: service
        namespaces:
          names:
            - <monitor-namespace>  # Replace with your namespace
    relabel_configs:
      - source_labels: [__meta_kubernetes_service_name]
        action: keep
        regex: hami-dra-monitor
      - source_labels: [__meta_kubernetes_service_port_name]
        action: keep
        regex: metrics
```

### Static Configuration

Alternatively, you can use static configuration:

```yaml
scrape_configs:
  - job_name: 'hami-dra-monitor'
    static_configs:
      - targets:
        - 'hami-dra-monitor.<namespace>.svc.cluster.local:8080'
```

## Resource Requirements

Default resource requests and limits:

```yaml
monitor:
  resources:
    limits:
      cpu: 500m
      memory: 512Mi
    requests:
      cpu: 100m
      memory: 128Mi
```

Adjust these values based on your cluster size and monitoring requirements.

## Troubleshooting

### Check Monitor Status

```bash
# Check pod status
kubectl get pods -l app.kubernetes.io/component=monitor -n <namespace>

# Check logs
kubectl logs -l app.kubernetes.io/component=monitor -n <namespace>

# Check service
kubectl get svc hami-dra-monitor -n <namespace>
```

### Verify Metrics Endpoint

```bash
# Port-forward to access metrics
kubectl port-forward svc/hami-dra-monitor 8080:8080 -n <namespace>

# Check metrics
curl http://localhost:8080/metrics | grep hami_dra_
```

### Check Cache Sync Status

The monitor requires the cache to be synced before it can collect metrics. Check the logs for:

```
Cache started and synced successfully
Cache is ready
```

If the cache fails to sync, check:
- RBAC permissions for ResourceSlice and ResourceClaim resources
- Network connectivity to the Kubernetes API server
- ResourceSlice and ResourceClaim resources exist in the cluster

## Architecture

The monitor component consists of:

1. **Cache Layer**: Maintains an in-memory cache of ResourceSlice and ResourceClaim resources
2. **Metrics Collector**: Implements Prometheus Collector interface to gather metrics from the cache
3. **HTTP Servers**: 
   - Metrics server on port 8080
   - Health probe server on port 8000

The monitor uses Kubernetes informers to watch ResourceSlice and ResourceClaim resources, ensuring the cache stays up-to-date with cluster state.

## Performance Considerations

- **Cache Sync**: The monitor waits for ResourceSlice cache to sync before processing ResourceClaim events to ensure data consistency
- **Concurrency**: Uses node-level locking to minimize contention when updating device usage
- **API Rate Limiting**: Configure `kubeAPIQPS` and `kubeAPIBurst` to control API server load
- **Metrics Collection**: Metrics are collected on-demand when Prometheus scrapes the endpoint, not on a fixed interval

