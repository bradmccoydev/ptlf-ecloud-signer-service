# Health Endpoints

## GET /health

Liveness probe endpoint. Returns 200 if the process is running.

```bash
curl http://signer-service:8080/health
```

Response:
```json
{"status": "ok"}
```

## GET /ready

Readiness probe endpoint. Returns 200 only when all critical dependencies are reachable.

### Dependencies Checked

| Dependency | Critical | Check Method |
|------------|----------|-------------|
| Harbor | Yes | HEAD request to `/api/v2.0/health` |
| Fulcio | Yes | HEAD request to `/api/v1/rootCert` |
| Rekor | Yes | HEAD request to `/api/v1/log` |

### Responses

**Ready (200):**
```json
{"status": "ready"}
```

**Not Ready — dependency down (503):**
```json
{
  "status": "unavailable",
  "dependencies": ["fulcio"]
}
```

**Not Ready — shutting down (503):**
```json
{"status": "shutting_down"}
```

### Shutdown Behaviour

When the service receives SIGTERM:
1. The readiness endpoint immediately returns 503
2. The load balancer stops routing new traffic
3. In-flight signing operations complete (30s grace period)
4. Process exits

## Prometheus Metrics

**GET** `http://signer-service:9090/metrics`

Returns all Prometheus metrics in the standard exposition format. Served on a dedicated port (9090) separate from the main application server.

Key metrics exposed:

| Metric | Type | Labels |
|--------|------|--------|
| `signatures_issued_total` | Counter | `source` (webhook/reconciliation) |
| `gate_failures_total` | Counter | `reason` |
| `webhook_latency_seconds` | Histogram | — |
| `fulcio_errors_total` | Counter | — |
| `rekor_errors_total` | Counter | — |
| `reconciliation_signed_total` | Counter | — |
| `reconciliation_errors_total` | Counter | — |
