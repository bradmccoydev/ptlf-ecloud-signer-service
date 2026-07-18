# Troubleshooting

## Common Issues

### Service Not Ready (503 on /ready)

**Symptoms**: Readiness probe failing, pods marked as not-ready.

**Check**:
```bash
kubectl exec -it <pod> -n platform-signing -- curl -s http://localhost:8080/ready | jq .
```

**Common Causes**:
1. **Harbor unreachable** — Check Harbor pods/service in the `harbor` namespace
2. **Fulcio unreachable** — Check Fulcio deployment in the `sigstore` namespace
3. **Rekor unreachable** — Check Rekor deployment in the `sigstore` namespace
4. **NetworkPolicy blocking** — Verify egress rules allow traffic to dependencies

### Webhook Returns 401

**Symptoms**: Harbor reports webhook delivery failure with 401 status.

**Check**:
1. Verify the `harbor-signer-webhook` ExternalSecret is synced:
   ```bash
   kubectl get externalsecret harbor-signer-webhook -n platform-signing
   ```
2. Verify the secret content matches Harbor's webhook config
3. Check logs for `webhook authentication failed` entries

### Images Not Being Signed

**Symptoms**: Images pass scanning but no cosign signature appears.

**Check**:
1. Verify webhook is configured in Harbor project settings
2. Check logs for `gate decision: fail` entries (may have vulnerabilities)
3. Check `gate_failures_total` metric for failure reasons
4. Verify the reconciliation sweep is running (check logs for `reconciliation sweep completed`)

### Fulcio Errors

**Symptoms**: `fulcio_errors_total` metric increasing.

**Check**:
1. Verify Fulcio is healthy: `curl https://fulcio.platform.cuscal.io/api/v1/rootCert`
2. Check projected SA token:
   ```bash
   kubectl exec -it <pod> -n platform-signing -- cat /var/run/secrets/tokens/fulcio-token | jwt decode -
   ```
3. Verify the token audience is `sigstore`
4. Check EKS OIDC issuer is configured correctly

### Rekor Errors

**Symptoms**: `rekor_errors_total` metric increasing.

**Check**:
1. Verify Rekor is healthy: `curl https://rekor.platform.cuscal.io/api/v1/log`
2. Check network connectivity from the pod
3. Review logs for specific error messages

### Worker Pool Full

**Symptoms**: `worker pool queue full, dropping job` warnings in logs.

**Check**:
1. High webhook volume or slow signing pipeline
2. Consider increasing `WORKER_COUNT` or `QUEUE_SIZE`
3. Check for slow Harbor API responses causing worker backup

### Reconciliation Not Running

**Symptoms**: `reconciliation_signed_total` not incrementing, unsigned images accumulating.

**Check**:
1. Look for `reconciliation sweep completed` in recent logs
2. Check for `reconciliation sweep skipped: previous sweep still in progress`
3. Check for `reconciliation sweep timed out` (sweep taking > 10 minutes)
4. Verify `HARBOR_PROJECTS` env var includes the expected projects

## Diagnostic Commands

```bash
# View recent logs
kubectl logs -l app.kubernetes.io/name=signer-service -n platform-signing --tail=100

# Check pod events
kubectl describe pod -l app.kubernetes.io/name=signer-service -n platform-signing

# Check metrics endpoint
kubectl exec -it <pod> -n platform-signing -- curl -s http://localhost:9090/metrics | grep -E "^(signatures_issued|gate_failures|fulcio_errors|rekor_errors)"

# Test webhook manually
kubectl exec -it <pod> -n platform-signing -- curl -X POST http://localhost:8080/webhook \
  -H "X-Harbor-Secret: $(kubectl get secret harbor-signer-webhook -n platform-signing -o jsonpath='{.data.webhook-secret}' | base64 -d)" \
  -H "Content-Type: application/json" \
  -d '{"type":"SCANNING_COMPLETED","event_data":{"resources":[{"digest":"sha256:test"}],"repository":{"name":"test/image","namespace":"test"}}}'
```
