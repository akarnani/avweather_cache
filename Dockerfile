# Multi-stage build for minimal production image
# Build stage
FROM golang:1.26-alpine AS builder

# Install build dependencies
RUN apk add --no-cache ca-certificates

WORKDIR /build

# Copy dependency files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-w -s' -o avweather_cache .

# Runtime stage - distroless for security
FROM gcr.io/distroless/static-debian12:nonroot

# Copy CA certificates from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary
COPY --from=builder /build/avweather_cache /avweather_cache

# Copy default config (can be overridden by env vars)
COPY config.yaml /config.yaml

# Expose metrics and API port
EXPOSE 8080

# Run as non-root user (UID 65532 from distroless nonroot)
USER 65532:65532

ENTRYPOINT ["/avweather_cache"]
