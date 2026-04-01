# Fake DRA Driver

Goal: provide a minimal kubelet plugin that reads a ConfigMap once at startup and publishes fake devices for the current node.

## Capabilities
1. `nodeSelector`: controls which nodes use the configuration.
2. `groups[].selector`: defines device batches using node label selectors.
3. `nodes.<nodeName>.devices`: defines per-node devices explicitly and overrides batch devices with the same name.
4. Includes a standalone ConfigMap generator web UI command.

## Configuration Format
The configuration is a full `ConfigMap` YAML. The driver reads `data.config.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: fake-dra-config
  namespace: default
data:
  config.yaml: |
    nodeSelector:
      matchLabels:
        node-role.kubernetes.io/worker: ""
    groups:
      - name: a100
        selector:
          matchLabels:
            gpu-type: a100
        devices:
          - name: gpu-0
            allowMultipleAllocations: true
            attributes:
              architecture:
                string: Ampere
              attr.project-hami.io/minor:
                int: 2
              brand:
                string: Nvidia
              cudaComputeCapability:
                version: 8.0.0
              cudaDriverVersion:
                version: 12.9.0
              driverVersion:
                version: 575.57.8
              minor:
                int: 2
              pcieBusID:
                string: 0000:61:00.0
              productName:
                string: NVIDIA A100-SXM4-80GB
              resource.kubernetes.io/pcieRoot:
                string: pci0000:5a
              type:
                string: hami-gpu
              uuid:
                string: GPU-xxxxxxx-xxxx-xxxx-xxxx
            capacity:
              cores:
                value: "100"
                requestPolicy:
                  default: "100"
                  validRange:
                    max: "100"
                    min: "0"
                    step: "1"
              memory:
                value: 80Gi
                requestPolicy:
                  default: 80Gi
                  validRange:
                    max: 80Gi
                    min: 1Mi
                    step: 1Mi
    nodes:
      worker-1:
        devices:
          - name: gpu-1
            attributes:
              uuid:
                string: worker-1-gpu-1
```

Notes:
1. `attributes` supports `string`, `int`, `bool`, and `version`. Each attribute must define exactly one type.
2. `capacity` supports `value`, `requestPolicy.default`, `requestPolicy.validValues`, and `requestPolicy.validRange`.
3. If a capacity entry defines `requestPolicy`, the driver automatically enables `allowMultipleAllocations` for that device. You can also set it explicitly.
4. Device precedence is `nodes` over `groups` for devices with the same name.
5. The current version does not watch for ConfigMap changes. Restart the plugin after updating the ConfigMap.

## Startup Flags
Minimum required flags:

```bash
fake-driver \
  --node-name=worker-1 \
  --configmap-name=fake-dra-config
```

Common optional flags:
1. `--driver-name`: defaults to `fake.dra.hami.io`
2. `--configmap-namespace`: defaults to `default`
3. `--configmap-key`: defaults to `config.yaml`
4. `--kubeconfig`: used for out-of-cluster debugging

## Web UI

```bash
go run ./cmd/fake-confgen --listen-address=:8080
```

Then open `http://127.0.0.1:8080/`. The page generates a full `ConfigMap` YAML and does not persist any state.

How to use the page:
1. Fill the important fields first: `namespace/name`, node selector, group selector, device count, product name, memory, and cores.
2. Non-critical fields such as `uuid`, `minor`, `attr.project-hami.io/minor`, and `pcieBusID` are generated automatically.
3. The page supports visual add/remove for multiple `groups` and multiple `nodes` overrides.
4. Advanced fields are pre-filled and can be expanded only when needed.
5. The middle panel allows direct edits to `data.config.yaml`, and the final `ConfigMap` on the right updates in real time.
6. The UI supports English and Simplified Chinese switching.

## Helm Integration
The chart already supports the fake kubelet plugin. Enable it like this:

```bash
helm upgrade --install hami-dra ./charts/hami-dra \
  --set drivers.fake.enabled=true \
  --set drivers.nvidia.enabled=false
```

Important values:
1. `drivers.fake.image.*`: fake plugin image settings.
2. `drivers.fake.driverName`: DRA driver name.
3. `drivers.fake.deviceClassName`: defaults to `fake-gpu.project-hami.io`.
4. `drivers.fake.configMap.existingName`: use an existing `ConfigMap`; when set, the chart does not create one.
5. `drivers.fake.configMap.name`: optional custom name for the chart-managed `ConfigMap`.
6. `drivers.fake.configMap.key`: defaults to `config.yaml`.
7. By default, the template renders a built-in `ConfigMap` that creates eight `A100-SXM4-80GB` fake devices on every node.
8. If you need to override the default content, use `drivers.fake.configMap.inlineData`.