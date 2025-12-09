# Karmada Webhook Template

这是一个基于 Karmada 项目实现的 Kubernetes Webhook 服务器模版。该模版提供了一个完整的、可扩展的 webhook 服务器框架，可以用于实现自定义的 mutating 和 validating webhooks。

## 功能特性

- ✅ 基于 controller-runtime 的 webhook 服务器
- ✅ 支持 Mutating Webhook 和 Validating Webhook
- ✅ TLS 证书支持
- ✅ 健康检查和就绪探针
- ✅ Prometheus 指标支持
- ✅ 完整的 Kubernetes 部署配置
- ✅ 示例 webhook handlers

## 项目结构

```
.
├── cmd/
│   └── webhook/
│       ├── main.go              # 程序入口
│       └── app/
│           ├── webhook.go       # Webhook 服务器主逻辑
│           └── options/
│               └── options.go   # 命令行选项
├── pkg/
│   └── webhook/
│       └── example/
│           ├── mutating.go      # Mutating webhook 示例
│           └── validating.go    # Validating webhook 示例
├── deploy/
│   ├── deployment.yaml          # Deployment 配置
│   ├── webhook-configuration.yaml  # Webhook 配置
│   └── cert-manager.yaml        # 证书管理配置（可选）
├── Dockerfile                   # Docker 镜像构建文件
├── go.mod                       # Go 模块定义
└── README.md                    # 项目说明文档
```

## 快速开始

### 本地开发

1. **克隆仓库**
```bash
git clone <your-repo-url>
cd karmada-webhook-template
```

2. **安装依赖**
```bash
go mod download
```

3. **运行 webhook 服务器**
```bash
go run cmd/webhook/main.go \
  --kubeconfig=$HOME/.kube/config \
  --bind-address=0.0.0.0 \
  --secure-port=8443 \
  --cert-dir=/tmp/k8s-webhook-server/serving-certs
```

### 构建 Docker 镜像

```bash
docker build -t webhook:latest .
```

### 部署到 Kubernetes

1. **生成证书（使用 cert-manager）**

如果你使用 cert-manager，可以直接应用配置：

```bash
kubectl apply -f deploy/cert-manager.yaml
```

或者手动生成证书：

```bash
# 生成证书脚本（需要安装 openssl）
./scripts/generate-cert.sh
```

2. **部署 webhook 服务器**

```bash
kubectl apply -f deploy/deployment.yaml
```

3. **配置 webhook**

```bash
kubectl apply -f deploy/webhook-configuration.yaml
```

## 自定义 Webhook

### 添加 Mutating Webhook

1. 在 `pkg/webhook/` 下创建新的包
2. 实现 `admission.Handler` 接口：

```go
type MyMutatingAdmission struct {
    Decoder admission.Decoder
}

func (a *MyMutatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
    // 你的 mutating 逻辑
    return admission.PatchResponseFromRaw(req.Object.Raw, marshaledBytes)
}
```

3. 在 `cmd/webhook/app/webhook.go` 中注册：

```go
hookServer.Register("/mutate-myresource", &webhook.Admission{
    Handler: &myresource.MutatingAdmission{Decoder: decoder},
})
```

### 添加 Validating Webhook

类似地，实现 validating webhook：

```go
type MyValidatingAdmission struct {
    Decoder admission.Decoder
}

func (v *MyValidatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
    // 你的验证逻辑
    if valid {
        return admission.Allowed("")
    }
    return admission.Denied("reason")
}
```

## 配置选项

Webhook 服务器支持以下命令行选项：

- `--bind-address`: 绑定地址（默认: 0.0.0.0）
- `--secure-port`: HTTPS 端口（默认: 8443）
- `--cert-dir`: 证书目录（默认: /tmp/k8s-webhook-server/serving-certs）
- `--tls-cert-file-name`: 证书文件名（默认: tls.crt）
- `--tls-private-key-file-name`: 私钥文件名（默认: tls.key）
- `--tls-min-version`: 最小 TLS 版本（默认: 1.3）
- `--kube-api-qps`: Kube API QPS（默认: 40.0）
- `--kube-api-burst`: Kube API Burst（默认: 60）
- `--metrics-bind-address`: 指标服务地址（默认: :8080）
- `--health-probe-bind-address`: 健康检查地址（默认: :8000）

## 测试

### 测试 Mutating Webhook

创建一个 Pod：

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: nginx
    image: nginx:latest
EOF
```

检查 Pod 是否被添加了标签：

```bash
kubectl get pod test-pod -o jsonpath='{.metadata.labels}'
```

应该看到 `webhook-mutated: "true"` 标签。

### 测试 Validating Webhook

尝试创建一个没有容器的 Pod（应该被拒绝）：

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: invalid-pod
spec:
  containers: []
EOF
```

应该看到验证错误。

## 参考

- [Kubernetes Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/)
- [controller-runtime Webhooks](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/webhook)
- [Karmada Project](https://github.com/karmada-io/karmada)

## 许可证

Apache License 2.0

