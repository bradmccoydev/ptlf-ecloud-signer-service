# Signer Service

A stateless Go microservice that automatically signs container images after they pass vulnerability scanning in Harbor, using private Sigstore infrastructure (Fulcio + Rekor).

## Overview

The Signer Service receives `SCANNING_COMPLETED` webhooks from Harbor, evaluates scan reports against a severity threshold (High), and signs passing images using keyless Sigstore signing. A periodic reconciliation sweep ensures eventual consistency regardless of webhook reliability.

## Features

- Webhook-triggered container image signing
- Severity gate evaluation (blocks High/Critical CVEs)
- Keyless signing via Fulcio projected SA tokens
- Transparency log recording via Rekor
- Reconciliation sweep every 15 minutes
- Idempotent webhook processing
- Prometheus metrics and OpenTelemetry tracing
- Graceful shutdown with 30s drain

## Quick Start

```bash
make build
make test
```

See [docs/getting-started/quick-start.md](docs/getting-started/quick-start.md) for full setup instructions.

## Architecture

```
Harbor → webhook → Signer Service → Gate Evaluation → Fulcio → Cosign → Rekor
```

See [docs/architecture/overview.md](docs/architecture/overview.md) for detailed design.

## Configuration

All configuration via environment variables. See [docs/getting-started/configuration.md](docs/getting-started/configuration.md).

## Deployment

Deployed via Helm chart to `platform-signing` namespace on EKS. See [docs/operations/deployment.md](docs/operations/deployment.md).

## Testing

```bash
make test           # All tests (99 tests, 8 property-based)
make test-property  # Property-based tests only
make verify         # Format, vet, lint, test
```

## Documentation

Full TechDocs available in the `docs/` folder and published to Backstage.

## License

Apache 2.0
