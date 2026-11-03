# Multi-stage build for minimal production image
# Build stage. Runs on the CI runner's native platform and cross-compiles for
# the target, so the arm64 image isn't built under QEMU emulation.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS builder
ARG TARGETOS
ARG TARGETARCH

# Install build dependencies
RUN apk add --no-cache ca-certificates

WORKDIR /build

# Copy dependency files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary with optimizations. No -a: it forced the whole standard library
# to be recompiled on every build.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags '-w -s' -o avweather_cache .

# Runtime stage - distroless for security
FROM gcr.io/distroless/static-debian13:nonroot

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
