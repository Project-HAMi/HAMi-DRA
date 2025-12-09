.PHONY: build docker-build test clean run

# Build the webhook binary
build:
	go build -o bin/webhook cmd/webhook/main.go

# Build Docker image
docker-build:
	docker build -t webhook:latest .

# Run tests
test:
	go test ./...

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

# Deploy to Kubernetes
deploy:
	kubectl apply -f deploy/deployment.yaml
	kubectl apply -f deploy/webhook-configuration.yaml

# Undeploy from Kubernetes
undeploy:
	kubectl delete -f deploy/webhook-configuration.yaml
	kubectl delete -f deploy/deployment.yaml

