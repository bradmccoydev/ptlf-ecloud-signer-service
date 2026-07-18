# Signer Service

A stateless Go microservice that automatically signs container images after they pass vulnerability scanning in Harbor, using private Sigstore infrastructure (Fulcio + Rekor).

## Overview

The Signer Service ensures that only vulnerability-free images carry a Cuscal signature, enabling admission controllers and pull policies to enforce supply chain integrity. It receives `SCANNING_COMPLETED` webhooks from Harbor, evaluates scan results against a severity threshold (High), and signs passing images using keyless Sigstore signing.

## Key Features

- **Webhook-Triggered Signing**: Automatically signs images when Harbor completes a vulnerability scan
- **Severity Gate**: Rejects images with High or Critical vulnerabilities
- **Keyless Signing**: Uses short-lived Fulcio certificates via projected service account tokens — no persistent keys
- **Transparency Log**: All signatures recorded immutably in Rekor
- **Reconciliation Sweep**: Periodic sweep (every 15 minutes) catches images missed by webhook delivery
- **Idempotent Processing**: Duplicate webhooks produce no additional signatures
- **Observability**: Full OpenTelemetry tracing, Prometheus metrics, and structured JSON logging
- **High Availability**: 2 replicas with PodDisruptionBudget and graceful shutdown

## Quick Links

- [Quick Start Guide](getting-started/quick-start.md)
- [Architecture Overview](architecture/overview.md)
- [Webhook API](api/webhook.md)
- [Deployment Guide](operations/deployment.md)
- [SLOs and SLIs](slos-and-slis.md)
- [Tier 0 Classification](tier0-classification.md)
- [Disaster Recovery](disaster-recovery.md)
- [Runbooks](runbooks/)

## Technology Stack

- **Language**: Go 1.25
- **Framework**: Gin Web Framework
- **Signing**: Sigstore (cosign, Fulcio, Rekor)
- **Registry**: Harbor
- **Observability**: OpenTelemetry, Prometheus
- **Testing**: Standard Go testing, gopter for property-based testing
- **Deployment**: Kubernetes, Helm, ArgoCD

## How It Works

```
Harbor → SCANNING_COMPLETED webhook → Signer Service
                                          ↓
                                   Validate secret
                                          ↓
                                   Check if already signed (skip if yes)
                                          ↓
                                   Fetch scan report from Harbor
                                          ↓
                                   Evaluate against severity threshold
                                          ↓
                              PASS: Sign with Fulcio + Rekor
                              FAIL: Log decision, leave unsigned
```

## Operational Classification

This service is classified as **Tier 0 — Mission Critical** under the Cuscal Tiering Framework.

| Metric | Target |
|--------|--------|
| Availability | 99.97% |
| RTO | ≤ 2 hours |
| RPO | N/A (stateless) |
| Webhook Latency (P99) | < 5s |

See [Tier 0 Classification](tier0-classification.md) and [SLOs and SLIs](slos-and-slis.md) for full details.
