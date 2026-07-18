# Multi-stage build for Signer Service
# Build stage (Cuscal Chainguard Go image — git and CA certs included)
FROM cgr.dev/cuscal.io/go:latest AS builder

WORKDIR /app

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Static binary for chainguard/static runtime (no libc/CGO)
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -a -installsuffix cgo \
    -o signer-service ./cmd/

# Runtime — minimal distroless image; CA certs and tzdata included
FROM cgr.dev/cuscal.io/static:latest

# Copy the binary from builder
COPY --from=builder --chown=1000:1000 /app/signer-service /app/signer-service

# Match Helm securityContext (runAsUser 1000)
USER 1000:1000
WORKDIR /app

# Expose application and metrics ports
EXPOSE 8080 9090

ENTRYPOINT ["/app/signer-service"]
