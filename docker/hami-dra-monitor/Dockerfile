# Build the monitor binary.
# BUILDPLATFORM keeps the Go toolchain native; TARGETOS/TARGETARCH cross-compile for each image platform.
FROM --platform=$BUILDPLATFORM golang:1.26.6@sha256:0d1d3a794be25f809dd2cb3160d8c73276c4056a9f8242a138e908ddeee7b6b6 AS builder

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

FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
RUN adduser -D -u 1000 monitor
COPY --from=builder /workspace/monitor /bin/monitor
USER monitor
CMD ["/bin/monitor"]
