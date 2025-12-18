GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
BUILD_ARCH ?= linux/$(GOARCH)

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

.PHONY: build docker-build test clean run license license-check fmt lint

# Build the webhook binary
build:
	go build -o bin/webhook cmd/webhook/main.go

# Build docker images
.PHONY: docker-build
docker-build:
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

# Run tests
test:
	go test ./...

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
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f webhook

# Run webhook locally (requires kubeconfig)
run: build
	./bin/webhook \
		--kubeconfig=$$HOME/.kube/config \
		--bind-address=0.0.0.0 \
		--secure-port=8443 \
		--cert-dir=/tmp/k8s-webhook-server/serving-certs

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
