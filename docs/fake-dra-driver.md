# fake DRA driver

目标：做一个尽量精简的 kubelet plugin，只在启动时读取一次 ConfigMap，为当前节点发布 fake 设备

## 支持能力
1. `nodeSelector`：控制哪些节点启用这份配置。
2. `groups[].selector`：按节点标签批量下发设备。
3. `nodes.<nodeName>.devices`：按节点名精确定义设备，且覆盖同名批量设备。
4. 提供一个独立的 ConfigMap 生成页面命令。

## 配置格式
配置文件直接使用完整的 `ConfigMap` YAML，驱动实际读取 `data.config.yaml`：

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

说明：
1. `attributes` 支持 `string`、`int`、`bool`、`version` 四种类型，且每个属性必须只设置一种。
2. `capacity` 支持 `value`，也支持 `requestPolicy.default`、`requestPolicy.validValues`、`requestPolicy.validRange`。
3. 如果某个 capacity 配置了 `requestPolicy`，驱动会自动为该设备开启 `allowMultipleAllocations`；也可以显式配置。
4. 同名设备优先级：`nodes` 覆盖 `groups`。
5. 当前版本不监听 ConfigMap 变化，修改后需重启插件。

## 启动参数
最小必填参数：

```bash
fake-driver \
  --node-name=worker-1 \
  --configmap-name=fake-dra-config
```

常用可选参数：
1. `--driver-name`：默认 `fake.dra.hami.io`
2. `--configmap-namespace`：默认 `default`
3. `--configmap-key`：默认 `config.yaml`
4. `--kubeconfig`：集群外调试时使用

## 网页

```bash
go run ./cmd/fake-confgen --listen-address=:8080
```

然后访问 `http://127.0.0.1:8080/`。页面输出的是完整 `ConfigMap` YAML，不做持久化。

页面使用方式：
1. 优先填写关键配置：`namespace/name`、节点 selector、分组 selector、设备数量、型号、显存、核心数。
2. 非关键字段如 `uuid`、`minor`、`attr.project-hami.io/minor`、`pcieBusID` 会自动生成。
3. 页面支持可视化增删多个 `groups` 和多个 `nodes` 覆盖项。
4. 高级字段默认已给出，可按需展开修改。
5. 中间栏可直接微调 `data.config.yaml`，右侧会实时生成最终 `ConfigMap`。

## Helm 集成
Chart 已支持 fake kubelet plugin，可通过如下方式启用：

```bash
helm upgrade --install hami-dra ./charts/hami-dra \
  --set drivers.fake.enabled=true \
  --set drivers.nvidia.enabled=false
```

关键 values：
1. `drivers.fake.image.*`：fake plugin 镜像。
2. `drivers.fake.driverName`：DRA driver name。
3. `drivers.fake.deviceClassName`：默认 `fake-gpu.project-hami.io`。
4. `drivers.fake.configMap.existingName`：引用已有 `ConfigMap`，设置后 chart 不再创建。
5. `drivers.fake.configMap.name`：可选，自定义 chart 创建的 `ConfigMap` 名称。
6. `drivers.fake.configMap.key`：默认 `config.yaml`。
7. 默认情况下，template 会直接渲染一个预制 `ConfigMap`，在所有节点上创建 8 张 `A100-SXM4-80GB` fake 设备。
8. 如确实需要覆盖默认内容，可使用 `drivers.fake.configMap.inlineData`。