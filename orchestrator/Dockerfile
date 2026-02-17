# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files and vendor directory from orchestrator/
COPY orchestrator/go.mod orchestrator/go.sum ./
COPY orchestrator/vendor/ ./vendor/

# Copy source code from orchestrator/
COPY orchestrator/ .

# Build arguments for version info
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT

# Build binary using vendored modules
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -mod=vendor \
    -ldflags "-w -s -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}" \
    -o /orchestrator \
    ./cmd/orchestrator

# Runtime stage
FROM alpine:latest

# Install runtime dependencies (ffmpeg includes ffprobe for embedded subtitle detection)
RUN apk add --no-cache ca-certificates tzdata ffmpeg

# Create non-root user
RUN addgroup -g 568 -S subgen && \
    adduser -u 568 -S subgen -G subgen

# Copy binary from builder
COPY --from=builder /orchestrator /usr/local/bin/orchestrator

# Set ownership
RUN chown subgen:subgen /usr/local/bin/orchestrator

# Switch to non-root user
USER subgen

# Expose ports
# 9000: HTTP webhooks
# 9090: Prometheus metrics
EXPOSE 9000 9090

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/orchestrator", "--health"] || exit 1

# Run orchestrator
ENTRYPOINT ["/usr/local/bin/orchestrator"]
