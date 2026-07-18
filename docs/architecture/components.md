# Components

## Package Structure

```
cmd/main.go                     — Entry point, DI wiring, server lifecycle
internal/
  config/                       — Environment-based configuration with validation
  handler/
    webhook/                    — POST /webhook handler, auth, payload extraction
    health/                     — GET /health, GET /ready with dependency checks
    metrics/                    — Dedicated metrics server on :9090
  middleware/                   — Recovery, logging, tracing, metrics middleware
  service/
    signing/                    — Signing pipeline orchestration
    gate/                       — Scan report severity evaluation
    reconciliation/             — Periodic sweep for missed images
  pkg/
    harbor/                     — Harbor API client (scan reports, artifacts, signatures)
    sigstore/                   — Cosign signing, Fulcio cert, Rekor upload
    observability/              — OTel tracing init, Prometheus metric definitions
    logger/                     — Zap structured logger
    workerpool/                 — Bounded goroutine worker pool
```

## Component Responsibilities

### Webhook Handler (`internal/handler/webhook/`)
- Authenticates incoming requests via `X-Harbor-Secret` header
- Validates payload size (1MB limit) and JSON structure
- Extracts artifact metadata (project, repo, digest)
- Discards non-SCANNING_COMPLETED events
- Enqueues signing jobs to the worker pool

### Gate Evaluator (`internal/service/gate/`)
- Evaluates scan reports against configurable severity threshold
- Produces pass/fail decisions with CVE counts and violation details
- Handles malformed reports gracefully (fail-safe)

### Signing Service (`internal/service/signing/`)
- Orchestrates the full signing pipeline
- Checks idempotency (existing signatures)
- Coordinates Harbor → Gate → Fulcio → Cosign → Rekor flow
- Emits metrics and traces for each step

### Reconciler (`internal/service/reconciliation/`)
- Runs on a 15-minute ticker
- Lists artifacts across all configured Harbor projects
- Signs any artifact with a passing scan but no signature
- Implements 10-minute sweep timeout and skip-if-running logic

### Harbor Client (`internal/pkg/harbor/`)
- Fetches vulnerability scan reports
- Lists artifacts with scan status filtering
- Checks for existing cosign signatures
- Implements exponential backoff retry

### Sigstore Signer (`internal/pkg/sigstore/`)
- Reads projected SA token for Fulcio authentication
- Requests short-lived signing certificates
- Signs images with annotations (scan-report, trivy-db, policy, timestamp)
- Uploads to Rekor transparency log
- Implements independent retry for Fulcio and Rekor

### Worker Pool (`internal/pkg/workerpool/`)
- Bounded goroutine pool (configurable worker count and queue size)
- Non-blocking submit with warning log on full queue
- Graceful shutdown with configurable drain timeout

### Health Handler (`internal/handler/health/`)
- Liveness: always returns 200
- Readiness: checks Harbor, Fulcio, Rekor connectivity
- Shutdown flag: returns 503 on SIGTERM
