# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make build-base ca-certificates

WORKDIR /app

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build arguments for version information
ARG VERSION="1.0.0"
ARG COMMIT="HEAD"
ARG BUILD_TIME=""

# Build the server binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}" \
    -a -installsuffix cgo \
    -o pubsubgo-server ./cmd/server

# Build the CLI binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}" \
    -a -installsuffix cgo \
    -o pubsub-cli ./cmd/cli

# Final runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata curl

# Create non-root user
RUN addgroup -g 1000 pubsub && \
    adduser -D -u 1000 -G pubsub pubsub

# Create necessary directories
RUN mkdir -p /app/data /app/config /app/logs && \
    chown -R pubsub:pubsub /app

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /app/pubsubgo-server /usr/local/bin/pubsubgo-server
COPY --from=builder /app/pubsub-cli /usr/local/bin/pubsub-cli

# Copy configuration
COPY config.yaml /app/config/config.yaml

# Switch to non-root user
USER pubsub

# Expose ports
EXPOSE 8080 9091

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Default command
ENTRYPOINT ["pubsubgo-server"]
CMD ["-config", "/app/config/config.yaml"]