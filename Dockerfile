# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.mod
COPY go.sum go.sum

# Download dependencies
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY pkg/ pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o webhook cmd/webhook/main.go

# Runtime stage
FROM alpine:latest

WORKDIR /

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

COPY --from=builder /workspace/webhook .

ENTRYPOINT ["/webhook"]

