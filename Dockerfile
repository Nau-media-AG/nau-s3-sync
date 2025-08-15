# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY src/ src/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o s3-sync ./src

# Runtime stage
FROM alpine:3.18

# Install rclone and ca-certificates
RUN apk add --no-cache ca-certificates curl unzip \
    && curl -O https://downloads.rclone.org/rclone-current-linux-amd64.zip \
    && unzip rclone-current-linux-amd64.zip \
    && mv rclone-*/rclone /usr/local/bin/ \
    && chmod +x /usr/local/bin/rclone \
    && rm -rf rclone-* \
    && apk del curl unzip

# Create non-root user
RUN addgroup -g 65532 -S syncuser && \
    adduser -u 65532 -S syncuser -G syncuser

# Create directories with proper permissions
RUN mkdir -p /app /tmp/sync-locks /tmp/rclone-config && \
    chown -R syncuser:syncuser /app /tmp/sync-locks /tmp/rclone-config

# Copy the binary from builder stage
COPY --from=builder /app/s3-sync /app/s3-sync

# Switch to non-root user
USER syncuser

WORKDIR /app

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD pgrep s3-sync > /dev/null || exit 1

ENTRYPOINT ["./s3-sync"]