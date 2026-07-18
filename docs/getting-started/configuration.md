# Configuration

All configuration is loaded from environment variables. Required variables must be set or the service will fail to start.

## Environment Variables

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `SERVICE_NAME` | `signer-service` | No | Service name for logging and tracing |
| `SERVICE_VERSION` | `1.0.0` | No | Service version |
| `ENVIRONMENT` | `production` | No | Environment name |
| `SERVER_PORT` | `8080` | No | Main HTTP server port |
| `METRICS_PORT` | `9090` | No | Prometheus metrics port |
| `LOG_LEVEL` | `info` | No | Log level (debug, info, warn, error) |
| `HARBOR_URL` | — | **Yes** | Harbor registry base URL |
| `HARBOR_ROBOT_USER` | — | **Yes** | Harbor robot account username |
| `HARBOR_ROBOT_PASSWORD` | — | **Yes** | Harbor robot account password |
| `FULCIO_URL` | — | **Yes** | Fulcio CA endpoint URL |
| `REKOR_URL` | — | **Yes** | Rekor transparency log URL |
| `TOKEN_PATH` | `/var/run/secrets/tokens/fulcio-token` | No | Path to projected SA token |
| `TOKEN_AUDIENCE` | `sigstore` | No | Token audience for Fulcio auth |
| `WEBHOOK_SECRET` | — | **Yes** | Shared secret for webhook authentication |
| `MAX_PAYLOAD_SIZE` | `1048576` (1MB) | No | Maximum webhook payload size in bytes |
| `WORKER_COUNT` | `5` | No | Number of concurrent signing workers |
| `QUEUE_SIZE` | `100` | No | Worker pool queue capacity |
| `RECONCILE_INTERVAL` | `15m` | No | Interval between reconciliation sweeps |
| `RECONCILE_TIMEOUT` | `10m` | No | Maximum duration for a single sweep |
| `HARBOR_PROJECTS` | `chainguard,charts,platform,applications,library` | No | Comma-separated list of Harbor projects to scan |
| `MAX_RETRIES` | `3` | No | Maximum retry attempts for external calls |
| `RETRY_BASE_DELAY` | `1s` | No | Base delay for exponential backoff |
| `RETRY_MAX_DELAY` | `10s` | No | Maximum delay between retries |
| `SEVERITY_THRESHOLD` | `High` | No | Minimum severity to block signing (None, Low, Medium, High, Critical) |
| `OTEL_EXPORTER_ENDPOINT` | `http://opentelemetry-collector.observability.svc.cluster.local:4318` | No | OTLP/HTTP collector endpoint |

## Secrets Management

Secrets are managed via ExternalSecrets operator syncing from Vault:

- **`harbor-signer-webhook`**: Contains the webhook shared secret
- **`harbor-signer-credentials`**: Contains the robot account username and password

These are automatically created by the ExternalSecret resources in the Helm chart.

## Severity Threshold

The severity threshold determines which vulnerabilities block signing:

| Threshold | Blocks |
|-----------|--------|
| `None` | All vulnerabilities block signing |
| `Low` | Low and above block signing |
| `Medium` | Medium and above block signing |
| **`High`** (default) | High and Critical block signing |
| `Critical` | Only Critical blocks signing |

## Projected SA Token

The service authenticates to Fulcio using a Kubernetes projected service account token with:
- **Audience**: `sigstore`
- **Expiry**: 3600 seconds (1 hour)
- **Mount path**: `/var/run/secrets/tokens/fulcio-token`

This is configured automatically by the Helm chart's deployment template.
