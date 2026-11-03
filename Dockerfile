# Multi-stage build for minimal production image
# Build stage
FROM golang:1.27-alpine@sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb AS builder

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
# Base images are pinned by digest so Dependabot opens a PR (and an automated
# patch release follows) whenever upstream publishes a rebuilt image.
FROM gcr.io/distroless/static-debian13:nonroot@sha256:25ac8f0c3b2334a9eb05452266bd4697864a8c8ef0ef71e5e6480d7f92868329

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
