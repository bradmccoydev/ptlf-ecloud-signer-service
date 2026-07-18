# Disaster Recovery — Signer Service

## Overview

The signer service is **stateless** — it has no database or persistent storage. DR is significantly simpler than stateful services. Recovery consists of redeploying the application and verifying external dependency connectivity.

| Objective | Target |
|-----------|--------|
| **RTO** | ≤ 2 hours (120 minutes) |
| **RPO** | N/A (stateless — no data loss possible) |
| **MAO** | < 4 hours (240 minutes) |

---

## Recovery Procedures

### Phase 1: Assessment (5 minutes)

1. Confirm failure scope (cluster, namespace, or pod level)
2. Verify cluster access: `kubectl cluster-info`
3. Verify Helm chart is accessible in the repository

### Phase 2: Redeploy (20 minutes)

```bash
# Via ArgoCD (preferred)
argocd app sync signer-service --force

# Or via Helm
helm upgrade --install signer-service \
  argocharts/signer-service/ \
  -f argocharts/signer-service/values.yaml \
  --namespace platform-signing \
  --create-namespace \
  --wait --timeout 10m
```

### Phase 3: Verify Dependencies (10 minutes)

```bash
# Check readiness (validates Harbor, Fulcio, Rekor connectivity)
kubectl exec -it <pod> -n platform-signing -- curl -s http://localhost:8080/ready | jq .

# Verify projected SA token is mounted
kubectl exec -it <pod> -n platform-signing -- ls -la /var/run/secrets/tokens/fulcio-token

# Verify ExternalSecrets synced
kubectl get externalsecret -n platform-signing
```

### Phase 4: Validate Signing (10 minutes)

```bash
# Trigger a test webhook
kubectl exec -it <pod> -n platform-signing -- curl -X POST http://localhost:8080/webhook \
  -H "X-Harbor-Secret: $(kubectl get secret harbor-signer-webhook -n platform-signing -o jsonpath='{.data.webhook-secret}' | base64 -d)" \
  -H "Content-Type: application/json" \
  -d '{"type":"SCANNING_COMPLETED","event_data":{"resources":[{"digest":"sha256:test123"}],"repository":{"name":"test/app","namespace":"test"}}}'

# Check metrics for activity
kubectl exec -it <pod> -n platform-signing -- curl -s http://localhost:9090/metrics | grep signatures_issued
```

---

## RTO Timeline

| Phase | Duration | Cumulative |
|-------|----------|-----------|
| Assessment | 5 min | 5 min |
| Redeploy | 20 min | 25 min |
| Verify Dependencies | 10 min | 35 min |
| Validate Signing | 10 min | **45 min** |

**Total: ~45 minutes** (well within 120-minute RTO).

The stateless nature of this service means recovery is fast — no database restore or data migration required.

---

## DR Drill Requirements

- Frequency: Every 6 months
- Environment: Non-production (staging)
- Procedure: Delete namespace, execute full recovery, validate signing

---

## Related Documentation

- [Tier 0 Classification](tier0-classification.md)
- [SLOs and SLIs](slos-and-slis.md)
- [Runbook: Service Down](runbooks/service-down.md)
