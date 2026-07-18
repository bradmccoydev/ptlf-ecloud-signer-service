# Runbook: Reconciliation Failures

## Alert
**ReconciliationErrors** — `reconciliation_errors_total` incremented in the last 30 minutes.

## Impact
Images missed by webhook delivery will not be caught by the reconciliation sweep. Eventually consistent signing may be delayed.

## Investigation

```bash
# Check reconciliation-related logs
kubectl logs -l app.kubernetes.io/name=signer-service -n platform-signing --tail=100 | grep -i reconcil

# Check metrics
kubectl exec -it <pod> -n platform-signing -- curl -s http://localhost:9090/metrics | grep reconciliation

# Look for timeout warnings
kubectl logs -l app.kubernetes.io/name=signer-service -n platform-signing --tail=100 | grep "timed out"

# Look for skip warnings (previous sweep still running)
kubectl logs -l app.kubernetes.io/name=signer-service -n platform-signing --tail=100 | grep "still in progress"
```

## Common Causes

1. **Harbor API unreachable during sweep** — See [Harbor Connectivity](harbor-connectivity.md) runbook
2. **Sweep timeout (> 10 minutes)** — Too many artifacts to process in the timeout window
3. **Previous sweep still running** — Sweep taking longer than the 15-minute interval

## Resolution

### Harbor API errors
- Follow the [Harbor Connectivity](harbor-connectivity.md) runbook
- Once Harbor is restored, the next sweep (in ≤ 15 minutes) will catch up

### Timeout issues
- If the sweep consistently times out, consider:
  - Increasing `RECONCILE_TIMEOUT` (currently 10m)
  - Reducing `HARBOR_PROJECTS` to split work across instances
  - Investigating slow Harbor API responses

### Sweep overlap
- If sweeps consistently overlap (skip warnings):
  - Increase `RECONCILE_INTERVAL` (currently 15m)
  - Investigate why sweeps take > 15 minutes

## Note
Reconciliation failures are **not critical** — webhook-triggered signing still works. The reconciliation is a safety net for missed webhooks. However, persistent failures should be investigated to maintain eventual consistency guarantees.

## Escalation
Escalate if reconciliation failures persist > 1 hour (4 consecutive sweeps failing).
