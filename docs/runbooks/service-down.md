# Runbook: Service Down

## Alert
**SignerServiceDown** — No healthy signer-service pods for > 1 minute.

## Impact
All container image signing operations halt. Images will remain unsigned, blocking deployments that require signed images.

## Investigation

```bash
# Check pod status
kubectl get pods -n platform-signing -l app.kubernetes.io/name=signer-service

# Check pod events
kubectl describe pod -l app.kubernetes.io/name=signer-service -n platform-signing

# Check recent logs
kubectl logs -l app.kubernetes.io/name=signer-service -n platform-signing --tail=50

# Check if ExternalSecrets are synced
kubectl get externalsecret -n platform-signing

# Check deployment status
kubectl get deployment signer-service -n platform-signing
```

## Resolution Steps

1. **OOMKilled** — Increase memory limits in values.yaml
2. **CrashLoopBackOff** — Check logs for startup errors (missing env vars, secret sync failures)
3. **ImagePullBackOff** — Verify image exists in registry, check imagePullSecrets
4. **ExternalSecret not synced** — Check ExternalSecrets operator and Vault connectivity
5. **Node issues** — Check node health and available resources

## Escalation
If not resolved within 15 minutes, escalate to Engineering Manager.
