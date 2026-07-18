# Architecture Overview

## System Context

The Signer Service is a stateless Go microservice deployed in the `platform-signing` namespace. It integrates with three external systems:

- **Harbor Registry** — Source of scanning webhooks and scan reports; destination for cosign signatures
- **Fulcio CA** — Issues short-lived signing certificates based on OIDC identity
- **Rekor** — Immutable transparency log recording all signing events

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    platform-signing namespace                      │
│                                                                   │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │              Signer Service Pod (x2 replicas)               │  │
│  │                                                              │  │
│  │  ┌──────────────┐  ┌─────────────┐  ┌──────────────────┐  │  │
│  │  │ Gin HTTP :8080│  │Metrics :9090│  │ Reconciliation   │  │  │
│  │  │              │  │             │  │ Ticker (15m)     │  │  │
│  │  └──────┬───────┘  └─────────────┘  └────────┬─────────┘  │  │
│  │         │                                      │            │  │
│  │         ▼                                      │            │  │
│  │  ┌──────────────┐                              │            │  │
│  │  │ Worker Pool  │◀─────────────────────────────┘            │  │
│  │  │ (5 workers)  │                                           │  │
│  │  └──────┬───────┘                                           │  │
│  │         │                                                    │  │
│  │         ▼                                                    │  │
│  │  ┌──────────────────────────────────────────────────────┐  │  │
│  │  │            Signing Pipeline                           │  │  │
│  │  │  1. Check existing signature (idempotency)           │  │  │
│  │  │  2. Fetch scan report from Harbor                    │  │  │
│  │  │  3. Evaluate severity gate                           │  │  │
│  │  │  4. Request Fulcio certificate                       │  │  │
│  │  │  5. Sign image with cosign                           │  │  │
│  │  │  6. Record in Rekor                                  │  │  │
│  │  └──────────────────────────────────────────────────────┘  │  │
│  └────────────────────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────────────────────┘
         │                    │                    │
         ▼                    ▼                    ▼
┌─────────────────┐  ┌────────────────┐  ┌────────────────┐
│  Harbor Registry │  │   Fulcio CA    │  │  Rekor Log     │
│  :443            │  │   :443         │  │  :443          │
└─────────────────┘  └────────────────┘  └────────────────┘
```

## Design Principles

1. **Stateless** — No database; Harbor holds scan reports, the registry holds signatures, Rekor provides audit trail
2. **Async Webhook Processing** — HTTP handler returns 200 immediately, enqueues work to bounded worker pool
3. **Keyless Signing** — Uses short-lived Fulcio certificates via projected SA token; no persistent keys to manage
4. **Idempotent** — Duplicate webhooks checked against existing signatures; no duplicate signing
5. **Eventually Consistent** — Reconciliation sweep catches anything missed by webhooks
6. **Fail-Safe** — On any failure, images remain unsigned (blocked by Harbor pull policy)

## Security Model

- Read-only root filesystem
- Non-root user (UID 1000)
- No persistent signing keys (keyless via Fulcio)
- Projected SA token with 1-hour expiry
- NetworkPolicy restricts ingress/egress
- ExternalSecrets for credential management
- Constant-time secret comparison for webhook auth
