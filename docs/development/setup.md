# Development Setup

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.22+ | Language runtime |
| Docker | 24+ | Container builds |
| Make | any | Build automation |
| golangci-lint | 1.55+ | Linting (optional) |

## Getting Started

```bash
# Clone the repo
git clone https://github.com/cuscal/ptlf-ecloud-signer-service.git
cd ptlf-ecloud-signer-service

# Download dependencies
go mod download

# Build
make build

# Run all tests
make test

# Run only property-based tests
make test-property

# Format and vet
make fmt vet
```

## Project Layout

```
cmd/main.go           — Application entry point
internal/             — Private application code
  config/             — Environment configuration
  handler/            — HTTP handlers (webhook, health, metrics)
  middleware/         — HTTP middleware
  service/            — Business logic (signing, gate, reconciliation)
  pkg/                — Shared packages (harbor, sigstore, observability, logger, workerpool)
argocharts/           — Helm chart for Kubernetes deployment
docs/                 — TechDocs documentation
.github/workflows/    — CI/CD pipeline
```

## Running Locally

Set required environment variables:

```bash
export HARBOR_URL=https://registry.platform.cuscal.io
export HARBOR_ROBOT_USER=robot\$signer
export HARBOR_ROBOT_PASSWORD=your-password
export FULCIO_URL=https://fulcio.platform.cuscal.io
export REKOR_URL=https://rekor.platform.cuscal.io
export WEBHOOK_SECRET=dev-secret
export LOG_LEVEL=debug

./bin/signer-service
```

## IDE Setup

### VS Code

Recommended extensions:
- `golang.go` — Go language support
- `redhat.vscode-yaml` — YAML for Helm charts
- `ms-kubernetes-tools.vscode-kubernetes-tools` — Kubernetes support

### GoLand

The project uses standard Go modules. Open the root directory and GoLand will auto-detect the module.
