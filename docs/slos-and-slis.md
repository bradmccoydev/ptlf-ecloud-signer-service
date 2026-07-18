# Service Level Objectives and Indicators

## Overview

This document defines the SLOs and SLIs for the **ptlf-ecloud-signer-service**, classified as **Tier 0 — Mission Critical**. All SLOs are measured over a **rolling 30-day window**.

---

## Service Level Objectives

### 1. Availability SLO

| Attribute | Value |
|-----------|-------|
| **Target** | 99.97% |
| **Window** | Rolling 30 days |
| **Scope** | Webhook endpoint and health endpoints |

**Formula:**
```
Availability % = (1 - (count of 5xx responses / total requests)) × 100
```

**Allowed Downtime:** ~13 minutes per 30-day window.

---

### 2. Webhook Latency SLO

| Attribute | Value |
|-----------|-------|
| **Target** | P99 < 5 seconds |
| **Window** | Rolling 30 days |
| **Scope** | POST /webhook endpoint (time to HTTP 200 response) |

**Formula:**
```
histogram_quantile(0.99, rate(webhook_latency_seconds_bucket[30d])) < 5.0
```

---

### 3. Signing Success Rate SLO

| Attribute | Value |
|-----------|-------|
| **Target** | 99.9% of eligible images signed within 30 minutes |
| **Window** | Rolling 30 days |
| **Scope** | Images with passing scan that receive a signature |

**Formula:**
```
Signing Success % = (signatures_issued_total / (signatures_issued_total + gate_failures_total{reason=~"signing_error|fulcio_error|rekor_error"})) × 100
```

---

### 4. Error Rate SLO

| Attribute | Value |
|-----------|-------|
| **Target** | < 0.03% 5xx responses |
| **Window** | Rolling 30 days |
| **Scope** | All API requests |

---

## Service Level Indicators

| # | Metric Name | Type | Labels | Description |
|---|------------|------|--------|-------------|
| 1 | `signatures_issued_total` | Counter | `source` | Successful signing operations |
| 2 | `gate_failures_total` | Counter | `reason` | Gate failure decisions |
| 3 | `webhook_latency_seconds` | Histogram | — | Webhook processing latency |
| 4 | `fulcio_errors_total` | Counter | — | Fulcio failures after retries |
| 5 | `rekor_errors_total` | Counter | — | Rekor failures after retries |
| 6 | `reconciliation_signed_total` | Counter | — | Images signed during sweeps |
| 7 | `reconciliation_errors_total` | Counter | — | Sweep errors |

---

## Error Budget Policies

| Consumption Level | Threshold | Action |
|-------------------|-----------|--------|
| **Warning** | 50% consumed | Notification to service team |
| **Deploy Freeze** | 75% consumed | Only reliability/security fixes permitted |
| **Incident Declared** | 100% consumed | Engineering focus on reliability |

---

## Alerting Thresholds

| Alert | Condition | Severity |
|-------|-----------|----------|
| SignerServiceDown | No healthy pods > 1 minute | P1 |
| FulcioErrorsHigh | Rate > 5/min for 5m | P1 |
| RekorErrorsHigh | Rate > 5/min for 5m | P1 |
| WebhookLatencyHigh | P99 > 5s for 5m | P2 |
| ReconciliationErrors | Increment in last 30m | P2 |

---

## Revision History

| Date | Version | Author | Change |
|------|---------|--------|--------|
| 2025-07-01 | 1.0 | Platform Engineering | Initial SLO/SLI definitions |
