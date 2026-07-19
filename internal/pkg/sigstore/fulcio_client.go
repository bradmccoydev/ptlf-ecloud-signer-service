// Package sigstore provides Cosign signing, Fulcio certificate, and Rekor upload capabilities.
package sigstore

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPFulcioClient implements FulcioClient using the Fulcio REST API.
type HTTPFulcioClient struct {
	httpClient *http.Client
}

// NewHTTPFulcioClient creates a new Fulcio client with the given HTTP client.
// If httpClient is nil, a default client with 30s timeout is used.
func NewHTTPFulcioClient(httpClient *http.Client) *HTTPFulcioClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &HTTPFulcioClient{httpClient: httpClient}
}

// fulcioSigningCertRequest is the request body for Fulcio's /api/v2/signingCert endpoint.
type fulcioSigningCertRequest struct {
	Credentials      fulcioCredentials      `json:"credentials"`
	PublicKeyRequest fulcioPublicKeyRequest `json:"publicKeyRequest"`
}

type fulcioCredentials struct {
	OIDCIdentityToken string `json:"oidcIdentityToken"`
}

type fulcioPublicKeyRequest struct {
	PublicKey         fulcioPublicKey `json:"publicKey"`
	ProofOfPossession []byte          `json:"proofOfPossession"`
}

type fulcioPublicKey struct {
	Algorithm string `json:"algorithm"`
	Content   string `json:"content"`
}

// fulcioSigningCertResponse is the response from Fulcio's /api/v2/signingCert endpoint.
type fulcioSigningCertResponse struct {
	SignedCertificateEmbeddedSCT fulcioSignedCert `json:"signedCertificateEmbeddedSct"`
}

type fulcioSignedCert struct {
	Chain fulcioCertChain `json:"chain"`
}

type fulcioCertChain struct {
	Certificates []string `json:"certificates"`
}

// GetCertificate requests a short-lived signing certificate from Fulcio using the
// projected SA token as the OIDC identity token.
func (c *HTTPFulcioClient) GetCertificate(ctx context.Context, token string, fulcioURL string) (*SigningCertificate, error) {
	// Generate an ephemeral ECDSA key pair for signing.
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ephemeral key: %w", err)
	}

	// Encode the public key in PEM format for the Fulcio request.
	pubKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %w", err)
	}
	pubKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyDER,
	})

	// Sign the OIDC token as proof of possession of the private key.
	proofPayload := []byte(token)
	proofHash := hashForECDSA(proofPayload)
	proofSig, err := ecdsa.SignASN1(rand.Reader, privateKey, proofHash)
	if err != nil {
		return nil, fmt.Errorf("failed to sign proof of possession: %w", err)
	}

	// Build the Fulcio request.
	reqBody := fulcioSigningCertRequest{
		Credentials: fulcioCredentials{
			OIDCIdentityToken: token,
		},
		PublicKeyRequest: fulcioPublicKeyRequest{
			PublicKey: fulcioPublicKey{
				Algorithm: "ECDSA",
				Content:   string(pubKeyPEM),
			},
			ProofOfPossession: proofSig,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Fulcio request: %w", err)
	}

	// POST to Fulcio /api/v2/signingCert.
	endpoint := fmt.Sprintf("%s/api/v2/signingCert", fulcioURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create Fulcio request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Fulcio request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Fulcio response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("Fulcio returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse the certificate chain from the response.
	var certResp fulcioSigningCertResponse
	if err := json.Unmarshal(respBody, &certResp); err != nil {
		return nil, fmt.Errorf("failed to parse Fulcio response: %w", err)
	}

	certs := certResp.SignedCertificateEmbeddedSCT.Chain.Certificates
	if len(certs) == 0 {
		return nil, fmt.Errorf("Fulcio returned empty certificate chain")
	}

	// First cert is the leaf, rest is the chain.
	certPEM := []byte(certs[0])
	var chainPEM []byte
	for _, c := range certs[1:] {
		chainPEM = append(chainPEM, []byte(c)...)
	}

	// Encode the private key in PEM format.
	privKeyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}
	privKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privKeyDER,
	})

	return &SigningCertificate{
		CertPEM:    certPEM,
		ChainPEM:   chainPEM,
		PrivateKey: privKeyPEM,
	}, nil
}
