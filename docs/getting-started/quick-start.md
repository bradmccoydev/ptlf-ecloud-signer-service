# Quick Start

Get the signer service running locally in under 5 minutes.

## Prerequisites

- Go 1.22+ installed
- Docker (for container builds)
- Access to a Harbor registry (or mock for local dev)

## Clone and Build

```bash
git clone https://github.com/cuscal/ptlf-ecloud-signer-service.git
cd ptlf-ecloud-signer-service

# Build the binary
make build

# Run tests
make test
```

## Local Configuration

Create a `.env` file with the required environment variables:

```bash
HARBOR_URL=https://registry.platform.cuscal.io
HARBOR_ROBOT_USER=robot$signer
HARBOR_ROBOT_PASSWORD=your-robot-password
FULCIO_URL=https://fulcio.platform.cuscal.io
REKOR_URL=https://rekor.platform.cuscal.io
WEBHOOK_SECRET=your-webhook-secret
LOG_LEVEL=debug
```

## Run Locally

```bash
# Source env vars and run
export $(cat .env | xargs)
./bin/signer-service
```

The service starts on:
- **Port 8080**: Webhook and health endpoints
- **Port 9090**: Prometheus metrics

## Test the Webhook

```bash
curl -X POST http://localhost:8080/webhook \
  -H "Content-Type: application/json" \
  -H "X-Harbor-Secret: your-webhook-secret" \
  -d '{
    "type": "SCANNING_COMPLETED",
    "event_data": {
      "resources": [{"digest": "sha256:abc123"}],
      "repository": {"name": "library/nginx", "namespace": "library"}
    }
  }'
```

## Next Steps

- [Installation Guide](installation.md) for production deployment
- [Configuration Reference](configuration.md) for all environment variables
- [Architecture Overview](../architecture/overview.md) for design details
