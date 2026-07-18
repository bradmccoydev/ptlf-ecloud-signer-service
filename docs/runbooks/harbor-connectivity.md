# Runbook: Harbor Connectivity Failures

## Alert
**GateFailuresElevated** with reason `scan_fetch_error`, or readiness returning 503 for Harbor dependency.

## Impact
Cannot fetch scan reports. Webhook-triggered signing fails. Reconciliation sweep aborts.

## Investigation

```bash
# Check Harbor health from signer pod
kubectl exec -it <pod> -n platform-signing -- curl -s https://registry.platform.cuscal.io/api/v2.0/health

# Check readiness endpoint
kubectl exec -it <pod> -n platform-signing -- curl -s http://localhost:8080/ready | jq .

# Check Harbor pods
kubectl get pods -n harbor -l app=harbor

# Check signer logs for scan fetch errors
kubectl logs -l app.kubernetes.io/name=signer-service -n platform-signing --tail=50 | grep "scan_fetch_error"

# Check robot account credentials
kubectl get secret harbor-signer-credentials -n platform-signing -o jsonpath='{.data.robot-user}' | base64 -d
```

## Common Causes

1. **Harbor service down** — Check Harbor core, registry, and database pods
2. **Robot account expired/revoked** — Recreate robot account in Harbor admin
3. **Network policy blocking** — Verify egress to Harbor namespace on port 443
4. **Harbor API rate limiting** — Check if reconciliation sweep is overwhelming the API
5. **DNS resolution failure** — Verify DNS is working within the pod

## Resolution

1. If Harbor is down: escalate to Harbor administrators
2. If credentials issue: update the secret in Vault, trigger ExternalSecret sync
3. If network issue: review NetworkPolicy egress rules
4. If rate limiting: reduce reconciliation frequency or add backoff

## Escalation
If Harbor is confirmed down, escalate to the Harbor/Registry team immediately.
