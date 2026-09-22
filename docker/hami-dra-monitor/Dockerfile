# Build the monitor binary.
# BUILDPLATFORM keeps the Go toolchain native; TARGETOS/TARGETARCH cross-compile for each image platform.
FROM --platform=$BUILDPLATFORM golang:1.27.1@sha256:f44f6e88636cfb311f9ebace870ded69d943f227bb3cb27d32ffd84ea18c43ea AS builder

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
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -a -o monitor cmd/monitor/main.go

FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6
# ponytail: pinned digest can lag CVE fixes in apk repos; upgrade at build time to pick up patched packages
RUN apk update && apk upgrade --no-cache && adduser -D -u 1000 monitor
COPY --from=builder /workspace/monitor /bin/monitor
USER monitor
CMD ["/bin/monitor"]
