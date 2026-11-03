# Build the manager binary.
# BUILDPLATFORM keeps the Go toolchain native; TARGETOS/TARGETARCH cross-compile for each image platform.
FROM --platform=$BUILDPLATFORM golang:1.27.3@sha256:2920bb124f1a2e12a1d39f0cab756ae0068441ee1da7655dec7f5e374944095b AS builder

ARG TARGETOS
ARG TARGETARCH

ENV GO111MODULE=on
ENV CGO_ENABLED=0

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY pkg/ pkg/

# Build
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -a -o webhook cmd/webhook/main.go

FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a
# ponytail: pinned digest can lag CVE fixes in apk repos; upgrade at build time to pick up patched packages
RUN apk update && apk upgrade --no-cache && adduser -D -u 1000 webhook
COPY --from=builder /workspace/webhook /bin/webhook
USER webhook
CMD ["/bin/webhook"]
