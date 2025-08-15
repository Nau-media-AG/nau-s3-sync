# S3 Sync Tool

One-way S3 bucket synchronization using rclone in Kubernetes.

## Configuration

**Required:**
```yaml
env:
  SOURCE_S3_ENDPOINT: "https://..."
  SOURCE_ACCESS_KEY: "..."
  SOURCE_SECRET_KEY: "..."
  SOURCE_BUCKET: "..."
  DEST_S3_ENDPOINT: "https://..."
  DEST_ACCESS_KEY: "..."
  DEST_SECRET_KEY: "..."
  DEST_BUCKET: "..."
```

**Optional (with defaults):**
```yaml
env:
  DRY_RUN: "false"              # Set to "true" for testing
  MAX_DELETE: "1000"            # Max files to delete per sync
  RETRIES: "3"                  # Retry attempts
  BANDWIDTH_LIMIT: "50M"        # Bandwidth limit (empty = unlimited)
  LOG_LEVEL: "info"             # debug, info, warn, error

cronjob:
  schedule: "0 * * * *"         # Every hour (cron format)
```

## Features

- **One-way sync** with automatic deletion
- **Overlap prevention** via Kubernetes `concurrencyPolicy: Forbid`
- **Configurable scheduling** and resource limits
- **Comprehensive logging** with structured JSON output
- **Simple Helm deployment** with environment secret

## Testing

Enable dry-run mode to test without making changes:

```yaml
env:
  DRY_RUN: "true"
```

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Authentication errors | Verify S3 credentials in `values.yaml` |
| Network timeouts | Increase retries or add bandwidth limits |
| Resource limits exceeded | Increase memory/CPU in `values.yaml` |
