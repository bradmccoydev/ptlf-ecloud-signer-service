# Runbook: Rekor Failures

## Alert
**RekorErrorsHigh** — `rekor_errors_total` rate > 5/min for 5 minutes.

## Impact
Signing events cannot be recorded in the transparency log. Signing operations fail after retry exhaustion.

## Investigation

```bash
# Check Rekor health
curl -s https://rekor.platform.cuscal.io/api/v1/log

# Check Rekor pods
kubectl get pods -n sigstore -l app=rekor

# Check signer-service logs for Rekor errors
kubectl logs -l app.kubernetes.io/name=signer-service -n platform-signing --tail=50 | grep -i rekor

# Check network connectivity
kubectl exec -it <pod> -n platform-signing -- curl -s -o /dev/null -w "%{http_code}" https://rekor.platform.cuscal.io/api/v1/log
```

## Common Causes

1. **Rekor service down** — Check Rekor deployment and its backend (trillian, MySQL/MariaDB)
2. **Rekor storage full** — Check backend database disk usage
3. **Network policy blocking** — Verify egress to sigstore namespace
4. **High load** — Rekor may be overloaded during large reconciliation sweeps

## Resolution

1. If Rekor is down: restart Rekor deployment, check trillian backend
2. If storage issue: expand PVC or clean old entries
3. If network issue: review NetworkPolicy rules
4. Once resolved: verify `rekor_errors_total` stops incrementing

## Escalation
If not resolved within 15 minutes, escalate — signing transparency is broken.
