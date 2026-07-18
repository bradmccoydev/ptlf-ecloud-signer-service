# Monitoring

## Prometheus Metrics

The signer service exposes metrics on port 9090 at `/metrics`, scraped by Prometheus via a ServiceMonitor (30s interval).

### Key Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `signatures_issued_total` | Counter | `source` | Successful signing operations |
| `gate_failures_total` | Counter | `reason` | Gate failure decisions |
| `webhook_latency_seconds` | Histogram | — | End-to-end webhook processing time |
| `fulcio_errors_total` | Counter | — | Fulcio request failures (after retries) |
| `rekor_errors_total` | Counter | — | Rekor request failures (after retries) |
| `reconciliation_signed_total` | Counter | — | Images signed during reconciliation |
| `reconciliation_errors_total` | Counter | — | Reconciliation sweep errors |

### Source Label Values

| Value | Meaning |
|-------|---------|
| `webhook` | Signing triggered by Harbor webhook |
| `reconciliation` | Signing triggered by periodic sweep |

### Reason Label Values

| Value | Meaning |
|-------|---------|
| `vulnerability_exceeded` | Image has High/Critical CVEs |
| `scan_fetch_error` | Failed to retrieve scan report from Harbor |
| `signing_error` | Signing operation failed after retries |
| `fulcio_error` | Fulcio certificate request failed |
| `rekor_error` | Rekor upload failed |

## Alerting Rules

### Critical Alerts

| Alert | Condition | Severity |
|-------|-----------|----------|
| SignerServiceDown | No healthy pods for > 1 minute | P1 |
| FulcioErrorsHigh | `fulcio_errors_total` rate > 5/min for 5m | P1 |
| RekorErrorsHigh | `rekor_errors_total` rate > 5/min for 5m | P1 |

### Warning Alerts

| Alert | Condition | Severity |
|-------|-----------|----------|
| GateFailuresElevated | `gate_failures_total{reason="scan_fetch_error"}` rate > 10/min | P2 |
| ReconciliationErrors | `reconciliation_errors_total` increment in last 30m | P2 |
| WebhookLatencyHigh | P99 webhook_latency_seconds > 5s for 5m | P2 |

## Distributed Tracing

Traces are exported via OTLP/HTTP to the platform OpenTelemetry collector. Each signing operation produces a trace with child spans:

- `ProcessArtifact` (parent)
  - `CheckExistingSignature`
  - `FetchScanReport`
  - `EvaluateGate`
  - `SignArtifact`

Search for traces by:
- `service.name = "signer-service"`
- `artifact.digest = "sha256:..."`
- `source = "webhook" | "reconciliation"`

## Structured Logging

All logs are JSON-formatted with standard fields:
- `ts` — ISO8601 timestamp
- `level` — Log level
- `msg` — Message
- `service` — "signer-service"
- `version` — Service version
- `environment` — Deployment environment

### Key Log Messages

| Message | Level | Meaning |
|---------|-------|---------|
| `webhook accepted, signing job enqueued` | INFO | Valid webhook processed |
| `artifact already signed, skipping` | INFO | Idempotency check succeeded |
| `artifact signed successfully` | INFO | Signing complete |
| `gate decision: fail` | INFO | Image failed severity gate |
| `reconciliation sweep completed` | INFO | Sweep finished |
| `worker pool queue full, dropping job` | WARN | Worker pool saturated |
| `signing operation failed` | ERROR | Pipeline failure |
