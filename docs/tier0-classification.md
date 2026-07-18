# Tier 0 Classification — Signer Service

## Service Overview

| Field | Value |
|-------|-------|
| **Service Name** | ptlf-ecloud-signer-service |
| **Tiering Level** | Tier 0 — Mission Critical |
| **Description** | Container image signing service using private Sigstore infrastructure (Fulcio + Rekor) |
| **Owner** | DevSecOps / Platform Engineering |
| **Classification Date** | July 2025 |
| **Framework** | Cuscal Tiering Framework |

---

## Recovery Objectives

| Metric | Target | Description |
|--------|--------|-------------|
| **Availability** | 99.97% | Maximum 2.6 hours downtime per annum |
| **Maximum Allowable Outage (MAO)** | < 4 hours (240 minutes) | Longest period the service may be unavailable before unacceptable business impact |
| **Recovery Time Objective (RTO)** | ≤ 2 hours (120 minutes) | Target time to restore the service after a disaster |
| **Recovery Point Objective (RPO)** | N/A | Service is stateless; no data persistence required |
| **Operational Resumption Time (ORT)** | ≤ 15 minutes | Target time to restore the service after a non-DR incident |

---

## Incident Severity Mapping

| Severity Level | Classification | Description | Escalation Criteria |
|----------------|---------------|-------------|---------------------|
| **Severity 1** | Critical | Complete service unavailability — zero healthy pods, webhook endpoint unreachable, or all signing operations failing | Immediate escalation to on-call engineer and engineering manager; bridge call initiated within 5 minutes |
| **Severity 2** | High | Partial degradation — Fulcio or Rekor failures causing signing backlog, elevated error rates exceeding SLO thresholds | Escalation to on-call engineer; engineering manager notified within 15 minutes if not resolved |
| **Severity 3** | Medium | Minor degradation — reconciliation sweep failures, elevated latency within SLO bounds, intermittent errors below threshold | On-call engineer investigates; escalation if issue persists beyond 30 minutes |
| **Severity 4** | Low | Informational — configuration warnings, non-impacting anomalies, OTel export failures | Logged for review during business hours; no immediate action required |

---

## Priority Response Times

| Priority | Response Time | Investigation Start | Resolution Target | Applicable Severity |
|----------|--------------|--------------------|--------------------|---------------------|
| **P1** | 0–15 minutes | < 30 minutes | Within MAO (< 4 hours) | Severity 1 |
| **P2** | ≤ 30 minutes | < 1 hour | ≤ 4 hours | Severity 2 |
| **P3** | ≤ 2 hours | < 4 hours | ≤ 24 hours | Severity 3 |
| **P4** | Next business day | Best effort | ≤ 5 business days | Severity 4 |

---

## Service Dependencies

| Dependency | Criticality | Impact if Unavailable |
|------------|-------------|----------------------|
| **Harbor Registry** | Critical | Cannot fetch scan reports or check existing signatures. All signing operations halt. Webhook processing fails with gate_failures_total (scan_fetch_error). |
| **Fulcio CA** | Critical | Cannot obtain signing certificates. All signing operations fail after retry exhaustion. Images remain unsigned. |
| **Rekor Transparency Log** | Critical | Cannot record signing events. Signing operations fail after retry exhaustion. Images remain unsigned. |
| **EKS OIDC Issuer** | Critical | Projected SA tokens cannot be validated by Fulcio. All signing operations fail with authentication errors. |
| **OpenTelemetry Collector** | Low | Trace exports are dropped silently. No impact on signing operations. Operational visibility reduced. |

---

## Related Documentation

- [SLOs and SLIs](slos-and-slis.md)
- [Disaster Recovery](disaster-recovery.md)
- [Runbooks](runbooks/)
