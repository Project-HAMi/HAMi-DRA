# Build the fake driver binary
FROM golang:1.27.2@sha256:820153096fda4de6ac1b7194d12e416b7dd8f4e9a33d17e0345b111b945162d8 AS builder

ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOOS=linux

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/

RUN go build -ldflags="-s -w" -a -o fake-driver ./cmd/fake-driver && echo "Building GOARCH of $GOARCH.."

FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6
# ponytail: pinned digest can lag CVE fixes in apk repos; upgrade at build time to pick up patched packages
RUN apk update && apk upgrade --no-cache
COPY --from=builder /workspace/fake-driver /bin/fake-driver
ENTRYPOINT ["/bin/fake-driver"]
