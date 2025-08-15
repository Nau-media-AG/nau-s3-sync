# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Core Architecture

This is a **production-ready S3 synchronization tool** that runs as a Kubernetes CronJob. The architecture consists of three main components:

### 1. Go Application (`src/main.go`)
- **Single-file application** that orchestrates rclone for S3 sync operations
- **Configuration via environment variables** only - no config files or flags
- **Process flow**: Environment validation → rclone config generation → subprocess execution → structured logging
- **Key functions**: `loadConfig()` validates all required S3 credentials, `createRcloneConfig()` generates temporary rclone config, `runSync()` executes rclone subprocess
- **Logging**: Uses logrus with JSON formatter for Kubernetes-friendly structured output

### 2. Container & Build System
- **Multi-stage Docker build**: Go compilation in builder stage, minimal Alpine runtime
- **rclone integration**: Downloads and includes rclone binary in runtime image during build
- **Security**: Runs as non-root user (UID 65532), includes health checks
- **Build path**: `./src` directory (not `cmd/` - this was specifically changed)

### 3. Helm Chart Deployment (`chart/`)
- **Simplified architecture**: No ServiceAccount, RBAC, or PVC - intentionally removed for simplicity
- **Single environment secret**: All configuration via `environment-secret.yaml` template using `envFrom`
- **CronJob with overlap prevention**: Uses `concurrencyPolicy: Forbid` - no application-level locking needed
- **Template structure**: `cronjob.yaml` (main workload), `environment-secret.yaml` (config), `values.yaml` (defaults)

## Development Commands

### Build & Test
```bash
# Build and test Go application locally
go mod tidy
go build -o s3-sync ./src
./s3-sync  # Requires environment variables

# Build Docker image
docker build -t s3-sync:latest .

# Test container with dummy values
docker run --rm -e SOURCE_S3_ENDPOINT="test" -e SOURCE_ACCESS_KEY="test" \
  -e SOURCE_SECRET_KEY="test" -e SOURCE_BUCKET="test" \
  -e DEST_S3_ENDPOINT="test" -e DEST_ACCESS_KEY="test" \
  -e DEST_SECRET_KEY="test" -e DEST_BUCKET="test" \
  -e DRY_RUN="true" s3-sync:latest
```

### Helm Development
```bash
# Validate Helm templates
helm template test-release ./chart -f values.yaml

# Test deployment with dry-run
helm install --dry-run --debug test-release ./chart -f values.yaml

# Deploy to Kubernetes
helm install s3-sync ./chart -f values.yaml
```

### Kubernetes Operations
```bash
# Monitor CronJob
kubectl get cronjobs
kubectl get jobs -l app=s3-sync

# View logs
kubectl logs -l app=s3-sync --tail=100

# Manual job execution
kubectl create job --from=cronjob/s3-sync-cronjob manual-sync
```

## Configuration Architecture

### Environment-First Design
All configuration is via environment variables - no CLI flags, config files, or Kubernetes ConfigMaps. This simplifies the Helm chart and follows 12-factor app principles.

**Required Variables** (validation will fail if missing):
- `SOURCE_S3_ENDPOINT`, `SOURCE_ACCESS_KEY`, `SOURCE_SECRET_KEY`, `SOURCE_BUCKET`
- `DEST_S3_ENDPOINT`, `DEST_ACCESS_KEY`, `DEST_SECRET_KEY`, `DEST_BUCKET`

**Optional Variables** (with sensible defaults in `loadConfig()`):
- `DRY_RUN="false"`, `MAX_DELETE="1000"`, `RETRIES="3"`, `LOG_LEVEL="info"`

### Helm Values Structure
- `env` section maps directly to environment variables in the secret
- `cronjob` section controls Kubernetes CronJob behavior
- `image` section for container registry configuration
- No RBAC or ServiceAccount configuration (intentionally simplified)

## Key Implementation Details

### rclone Integration
- **Temporary config approach**: App generates rclone config file in `/tmp/rclone-config/` at runtime
- **Subprocess execution**: Uses `os/exec` to run rclone as external command, not as library
- **Sync vs Copy**: Uses `rclone sync` for one-way synchronization with deletion
- **Security**: rclone config file has 0600 permissions and is cleaned up after use

### Job Overlap Prevention
- **Kubernetes-level**: `concurrencyPolicy: Forbid` prevents multiple CronJobs
- **No application-level locking**: Previous file-based locking was removed as redundant
- **Timeout handling**: `activeDeadlineSeconds` provides job-level timeout

### Error Handling & Logging
- **Structured JSON logging**: All log output via logrus for Kubernetes log aggregation
- **Retry logic**: Built into rclone via `--retries` flag, not application-level
- **Exit codes**: Application exits with proper codes for Kubernetes job status

## Development Notes

- **Go modules**: Use `go mod tidy` when adding dependencies
- **Container security**: Always maintain non-root user and minimal Alpine base
- **Helm simplicity**: Resist adding back ServiceAccount/RBAC unless specifically needed
- **Environment variables**: All new configuration should follow environment-first pattern
- **Testing approach**: Use `DRY_RUN="true"` for safe testing against real S3 endpoints