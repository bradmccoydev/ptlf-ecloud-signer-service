# Runbook: High Error Rate

## Alert
**GateFailuresElevated** — `gate_failures_total` rate exceeding thresholds.

## Impact
Images may not be getting signed. Depending on the reason, this could indicate infrastructure issues or a spike in vulnerable images.

## Investigation

```bash
# Check which reasons are firing
kubectl exec -it <pod> -n platform-signing -- curl -s http://localhost:9090/metrics | grep gate_failures_total

# Check recent gate failure logs
kubectl logs -l app.kubernetes.io/name=signer-service -n platform-signing --tail=100 | grep "gate decision: fail"
```

## Resolution by Reason

### `vulnerability_exceeded` (Expected behaviour)
- Not an error — images have High/Critical CVEs
- Check if a new base image introduced vulnerabilities
- Notify teams to update their images

### `scan_fetch_error`
- Harbor API may be unreachable or slow
- Check Harbor health: `curl https://registry.platform.cuscal.io/api/v2.0/health`
- Check NetworkPolicy egress rules

### `signing_error`
- Fulcio or Rekor issues — see specific runbooks
- Check `fulcio_errors_total` and `rekor_errors_total` metrics

## Escalation
If `signing_error` or `scan_fetch_error` persists > 15 minutes, escalate to on-call.
