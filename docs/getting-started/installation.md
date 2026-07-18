# Installation

## Kubernetes Deployment (Recommended)

The signer service is deployed via Helm chart to the `platform-signing` namespace.

### Prerequisites

- Kubernetes cluster with EKS OIDC issuer configured
- Fulcio and Rekor deployed on the cluster
- Harbor registry accessible
- ExternalSecrets operator configured with Vault backend
- ArgoCD for GitOps deployment

### Deploy with ArgoCD

```bash
argocd app sync signer-service
```

### Deploy with Helm (Manual)

```bash
helm upgrade --install signer-service \
  argocharts/signer-service/ \
  -f argocharts/signer-service/values.yaml \
  --namespace platform-signing \
  --create-namespace \
  --wait --timeout 5m
```

### Verify Deployment

```bash
# Check pods are running
kubectl get pods -n platform-signing -l app.kubernetes.io/name=signer-service

# Check readiness
kubectl exec -it <pod-name> -n platform-signing -- curl -s http://localhost:8080/ready

# Check metrics are exposed
kubectl exec -it <pod-name> -n platform-signing -- curl -s http://localhost:9090/metrics | head -20
```

## Docker

```bash
# Build
make docker-build

# Run
docker run -p 8080:8080 -p 9090:9090 \
  -e HARBOR_URL=https://registry.platform.cuscal.io \
  -e HARBOR_ROBOT_USER=robot\$signer \
  -e HARBOR_ROBOT_PASSWORD=secret \
  -e FULCIO_URL=https://fulcio.platform.cuscal.io \
  -e REKOR_URL=https://rekor.platform.cuscal.io \
  -e WEBHOOK_SECRET=my-secret \
  platform/signer-service:latest
```

## Harbor Webhook Configuration

Configure Harbor to send `SCANNING_COMPLETED` webhooks:

1. Navigate to Harbor → Project Settings → Webhooks
2. Add a new webhook:
   - **Event Type**: Scanning completed
   - **Endpoint URL**: `http://signer-service.platform-signing.svc.cluster.local:8080/webhook`
   - **Auth Header**: Set `X-Harbor-Secret` with the shared secret value

Repeat for each Harbor project (chainguard, charts, platform, applications, library).
