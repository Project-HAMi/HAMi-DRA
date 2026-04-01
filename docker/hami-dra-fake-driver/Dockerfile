# Build the fake driver binary
FROM golang:1.25.0 AS builder

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

FROM alpine:3.18
COPY --from=builder /workspace/fake-driver /bin/fake-driver
ENTRYPOINT ["/bin/fake-driver"]
