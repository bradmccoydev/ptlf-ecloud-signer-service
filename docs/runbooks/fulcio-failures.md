# Runbook: Fulcio Failures

## Alert
**FulcioErrorsHigh** — `fulcio_errors_total` rate > 5/min for 5 minutes.

## Impact
No new signing certificates can be obtained. All image signing operations fail after retry exhaustion.

## Investigation

```bash
# Check Fulcio health
curl -s https://fulcio.platform.cuscal.io/api/v1/rootCert

# Check Fulcio pods
kubectl get pods -n sigstore -l app=fulcio

# Check signer-service logs for Fulcio errors
kubectl logs -l app.kubernetes.io/name=signer-service -n platform-signing --tail=50 | grep -i fulcio

# Verify projected SA token
kubectl exec -it <pod> -n platform-signing -- cat /var/run/secrets/tokens/fulcio-token | cut -d. -f2 | base64 -d | jq .exp

# Check network connectivity
kubectl exec -it <pod> -n platform-signing -- curl -s -o /dev/null -w "%{http_code}" https://fulcio.platform.cuscal.io/api/v1/rootCert
```

## Common Causes

1. **Fulcio service down** — Restart Fulcio pods or check its dependencies (ctlog, CA)
2. **Token expired/invalid** — Verify projected SA token audience is `sigstore`; check OIDC issuer
3. **Network policy blocking** — Verify egress to sigstore namespace is allowed
4. **EKS OIDC issuer misconfigured** — Verify `system:serviceaccount:platform-signing:signer` identity

## Resolution

1. If Fulcio is down: restart Fulcio deployment in sigstore namespace
2. If token issue: delete pod to force new token mount
3. If network issue: review NetworkPolicy rules
4. Once resolved: verify `fulcio_errors_total` stops incrementing

## Escalation
If not resolved within 15 minutes, escalate — images cannot be signed.
