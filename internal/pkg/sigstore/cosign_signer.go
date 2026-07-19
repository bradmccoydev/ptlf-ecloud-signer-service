package sigstore

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
)

// DefaultCosignSigner implements CosignSigner by producing a cosign-compatible signature
// over the image manifest digest using the ephemeral Fulcio private key.
type DefaultCosignSigner struct{}

// NewDefaultCosignSigner creates a new DefaultCosignSigner.
func NewDefaultCosignSigner() *DefaultCosignSigner {
	return &DefaultCosignSigner{}
}

// cosignPayload is the Simple Signing payload format used by cosign.
type cosignPayload struct {
	Critical cosignCritical    `json:"critical"`
	Optional map[string]string `json:"optional"`
}

type cosignCritical struct {
	Identity cosignIdentity `json:"identity"`
	Image    cosignImage    `json:"image"`
	Type     string         `json:"type"`
}

type cosignIdentity struct {
	DockerReference string `json:"docker-reference"`
}

type cosignImage struct {
	DockerManifestDigest string `json:"docker-manifest-digest"`
}

// SignImage signs the specified image using the Simple Signing format and the
// ephemeral private key from the Fulcio certificate.
func (s *DefaultCosignSigner) SignImage(ctx context.Context, opts CosignSignOptions) (*CosignSignResult, error) {
	if opts.Certificate == nil || len(opts.Certificate.PrivateKey) == 0 {
		return nil, fmt.Errorf("signing certificate with private key is required")
	}

	// Parse the ephemeral private key.
	privKey, err := parseECPrivateKey(opts.Certificate.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ephemeral private key: %w", err)
	}

	// Extract the digest portion from the image ref (e.g., "sha256:abc123").
	digest := extractDigest(opts.ImageRef)
	if digest == "" {
		return nil, fmt.Errorf("image reference must include a digest: %s", opts.ImageRef)
	}

	// Extract the registry/repository portion (everything before @).
	dockerRef := extractDockerRef(opts.ImageRef)

	// Build the Simple Signing payload.
	payload := cosignPayload{
		Critical: cosignCritical{
			Identity: cosignIdentity{
				DockerReference: dockerRef,
			},
			Image: cosignImage{
				DockerManifestDigest: digest,
			},
			Type: "cosign container image signature",
		},
		Optional: opts.Annotations,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cosign payload: %w", err)
	}

	// Sign the payload with ECDSA-SHA256.
	payloadHash := sha256.Sum256(payloadBytes)
	signature, err := ecdsa.SignASN1(rand.Reader, privKey, payloadHash[:])
	if err != nil {
		return nil, fmt.Errorf("failed to sign payload: %w", err)
	}

	return &CosignSignResult{
		Signature: signature,
	}, nil
}

// parseECPrivateKey parses a PEM-encoded EC private key.
func parseECPrivateKey(pemData []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse EC private key: %w", err)
	}

	return key, nil
}

// extractDigest extracts the digest from an image reference like
// "registry.example.com/project/repo@sha256:abc123".
func extractDigest(imageRef string) string {
	for i := len(imageRef) - 1; i >= 0; i-- {
		if imageRef[i] == '@' {
			return imageRef[i+1:]
		}
	}
	return ""
}

// extractDockerRef extracts the docker reference (registry/repo) from an image reference,
// stripping the @digest portion.
func extractDockerRef(imageRef string) string {
	for i := len(imageRef) - 1; i >= 0; i-- {
		if imageRef[i] == '@' {
			return imageRef[:i]
		}
	}
	return imageRef
}
