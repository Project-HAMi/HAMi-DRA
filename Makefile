GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
BUILD_ARCH ?= linux/$(GOARCH)
GOTOOLCHAIN ?= go1.25.0+auto
GOLANGCI_LINT_VERSION ?= v2.5.0
GOLANGCI_LINT_ARGS ?= --timeout=5m

export GOTOOLCHAIN

ifeq ($(BUILD_ARM),true)
ifneq ($(GOARCH),arm64)
	  BUILD_ARCH= linux/$(GOARCH),linux/arm64
endif
endif
ifeq ($(BUILD_X86),true)
ifneq ($(GOARCH),amd64)
	  BUILD_ARCH= linux/$(GOARCH),linux/amd64
endif
endif

REGISTRY_REPO?="ghcr.io/projecthami"

.PHONY: build build-monitor build-fake-driver docker-build docker-build-monitor docker-build-fake-driver test test-quick test-coverage clean run run-monitor run-fake-driver license license-check fmt lint

# Build the webhook binary
build:
	go build -o bin/webhook cmd/webhook/main.go

# Build the monitor binary
build-monitor:
	go build -o bin/monitor cmd/monitor/main.go

# Build the fake driver binary
build-fake-driver:
	go build -o bin/fake-driver ./cmd/fake-driver

# Build docker images
docker-build: docker-build-webhook docker-build-monitor docker-build-fake-driver

# Build webhook docker images
.PHONY: docker-build-webhook
docker-build-webhook:
	echo "Building hami-dra-webhook for arch = $(BUILD_ARCH)"
	export DOCKER_CLI_EXPERIMENTAL=enabled ;\
	! ( docker buildx ls | grep hami-dra-webhook-multi-platform-builder ) && docker buildx create --use --platform=$(BUILD_ARCH) --name hami-dra-webhook-multi-platform-builder --driver-opt image=docker.io/moby/buildkit:buildx-stable-1 ;\
	docker buildx build \
			--builder hami-dra-webhook-multi-platform-builder \
			--platform $(BUILD_ARCH) \
			--build-arg LDFLAGS=$(LDFLAGS) \
			--tag $(REGISTRY_REPO)/hami-dra-webhook:latest  \
			-f ./docker/hami-dra-webhook/Dockerfile \
			--load \
			.

# Build monitor docker image
.PHONY: docker-build-monitor
docker-build-monitor:
	echo "Building hami-dra-monitor for arch = $(BUILD_ARCH)"
	export DOCKER_CLI_EXPERIMENTAL=enabled ;\
	! ( docker buildx ls | grep hami-dra-monitor-multi-platform-builder ) && docker buildx create --use --platform=$(BUILD_ARCH) --name hami-dra-monitor-multi-platform-builder --driver-opt image=docker.io/moby/buildkit:buildx-stable-1 ;\
	docker buildx build \
			--builder hami-dra-monitor-multi-platform-builder \
			--platform $(BUILD_ARCH) \
			--build-arg LDFLAGS=$(LDFLAGS) \
			--tag $(REGISTRY_REPO)/hami-dra-monitor:latest  \
			-f ./docker/hami-dra-monitor/Dockerfile \
			--load \
			.

# Build fake driver docker image
.PHONY: docker-build-fake-driver
docker-build-fake-driver:
	echo "Building hami-dra-fake-driver for arch = $(BUILD_ARCH)"
	export DOCKER_CLI_EXPERIMENTAL=enabled ;\
	! ( docker buildx ls | grep hami-dra-fake-driver-multi-platform-builder ) && docker buildx create --use --platform=$(BUILD_ARCH) --name hami-dra-fake-driver-multi-platform-builder --driver-opt image=docker.io/moby/buildkit:buildx-stable-1 ;\
	docker buildx build \
			--builder hami-dra-fake-driver-multi-platform-builder \
			--platform $(BUILD_ARCH) \
			--build-arg LDFLAGS=$(LDFLAGS) \
			--tag $(REGISTRY_REPO)/hami-dra-fake-driver:latest  \
			-f ./docker/hami-dra-fake-driver/Dockerfile \
			--load \
			.

# Run tests
test:
	go test -race ./...

# Run tests without race detection (faster)
test-quick:
	go test ./...

# Run tests with coverage (matches CI coverage step)
test-coverage:
	go test -v -coverprofile=coverage.out -covermode=atomic ./...

# Format Go code
fmt:
	@echo "Formatting Go code..."
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	else \
		gofmt -s -w .; \
	fi

# Lint Go code
lint:
	@echo "Linting Go code..."
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run $(GOLANGCI_LINT_ARGS)

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f webhook monitor fake-driver

# Run webhook locally (requires kubeconfig)
run: build
	./bin/webhook \
		--kubeconfig=$$HOME/.kube/config \
		--bind-address=0.0.0.0 \
		--secure-port=8443 \
		--cert-dir=/tmp/k8s-webhook-server/serving-certs

# Run monitor locally (requires kubeconfig)
run-monitor: build-monitor
	./bin/monitor \
		--kubeconfig=$$HOME/.kube/config \
		--metrics-bind-address=:8080 \
		--health-probe-bind-address=:8000

# Run fake driver locally (requires kubeconfig and an existing ConfigMap)
run-fake-driver: build-fake-driver
	./bin/fake-driver \
		--kubeconfig=$$HOME/.kube/config \
		--node-name=$${NODE_NAME} \
		--configmap-name=$${CONFIGMAP_NAME} \
		--configmap-namespace=$${CONFIGMAP_NAMESPACE:-default}

# Generate certificates for local development
cert:
	./scripts/generate-cert.sh

# Add or update license headers in all Go files
# Try to use addlicense tool if available, otherwise use the script
license:
	@if command -v addlicense >/dev/null 2>&1; then \
		echo "Using addlicense tool..."; \
		addlicense -c "The HAMi Authors" -l apache -y 2025 -s -f .license-header.txt .; \
	else \
		echo "addlicense not found, using script..."; \
		echo "To install addlicense: ./scripts/install-addlicense.sh"; \
		./scripts/add-license.sh; \
	fi

# Check license headers (dry-run with addlicense)
license-check:
	@if command -v addlicense >/dev/null 2>&1; then \
		addlicense -c "The HAMi Authors" -l apache -y 2025 -s -f .license-header.txt -check .; \
	else \
		echo "addlicense not found. Install it with: ./scripts/install-addlicense.sh"; \
		exit 1; \
	fi
