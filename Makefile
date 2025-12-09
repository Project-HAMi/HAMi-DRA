.PHONY: build docker-build test clean run license license-check fmt lint

# Build the webhook binary
build:
	go build -o bin/webhook cmd/webhook/main.go

# Build Docker image
docker-build:
	docker build --no-cache -t hami-dra-webhook:latest .

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
