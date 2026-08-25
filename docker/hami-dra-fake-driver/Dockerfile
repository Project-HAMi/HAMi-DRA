# Build the fake driver binary
FROM golang:1.27.0@sha256:65b6f280bf050ec5af12716857e8ea8439d694dbba8f31ceeb7630670071f2bb AS builder

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

FROM alpine:3.22@sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce
COPY --from=builder /workspace/fake-driver /bin/fake-driver
ENTRYPOINT ["/bin/fake-driver"]
