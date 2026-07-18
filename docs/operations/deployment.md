# Deployment

## Overview

The signer service is deployed to the `platform-signing` namespace on the genesis EKS cluster via ArgoCD.

## Deployment Topology

- **Replicas**: 2 (PodDisruptionBudget: minAvailable=1)
- **Namespace**: `platform-signing`
- **Service Account**: `signer` (trust identity: `system:serviceaccount:platform-signing:signer`)
- **Ports**: 8080 (HTTP), 9090 (metrics)

## Helm Chart

Located at `argocharts/signer-service/`:

```bash
# Render templates locally
helm template signer-service argocharts/signer-service/ -f argocharts/signer-service/values.yaml

# Deploy manually (prefer ArgoCD)
helm upgrade --install signer-service \
  argocharts/signer-service/ \
  --namespace platform-signing \
  --create-namespace
```

## ArgoCD

The service is managed by an ArgoCD Application that auto-syncs from the `main` branch.

```bash
# Check sync status
argocd app get signer-service

# Force sync
argocd app sync signer-service --force

# Rollback
argocd app rollback signer-service
```

## CI/CD Pipeline

On push to `main`:
1. Tests run (all 99 tests with race detector)
2. Multi-arch Docker image built (amd64 + arm64)
3. Image pushed to `registry.platform.cuscal.io/platform/signer-service`
4. Image signed with cosign
5. Helm values updated with new image tag

## Security Context

The deployment enforces:
- Read-only root filesystem
- Non-root user (UID 1000)
- No privilege escalation
- All capabilities dropped
- Seccomp profile: RuntimeDefault

## Projected SA Token

The deployment mounts a projected service account token at `/var/run/secrets/tokens/fulcio-token` with:
- Audience: `sigstore`
- Expiry: 3600 seconds (auto-refreshed by kubelet)

## NetworkPolicy

Restricts traffic to:
- **Ingress**: Harbor core pods only (port 8080), Prometheus (port 9090)
- **Egress**: Harbor, Fulcio, Rekor, KMS (port 443), OTel collector (port 4318), DNS (port 53)
