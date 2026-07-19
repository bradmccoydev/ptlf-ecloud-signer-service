package sigstore

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPRekorClient implements RekorClient using the Rekor REST API.
type HTTPRekorClient struct {
	httpClient *http.Client
}

// NewHTTPRekorClient creates a new Rekor client with the given HTTP client.
// If httpClient is nil, a default client with 30s timeout is used.
func NewHTTPRekorClient(httpClient *http.Client) *HTTPRekorClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &HTTPRekorClient{httpClient: httpClient}
}

// rekorCreateEntryRequest represents a HashedRekord entry for Rekor's /api/v1/log/entries endpoint.
type rekorCreateEntryRequest struct {
	APIVersion string                `json:"apiVersion"`
	Kind       string                `json:"kind"`
	Spec       rekorHashedRekordSpec `json:"spec"`
}

type rekorHashedRekordSpec struct {
	Signature rekorSignatureSpec `json:"signature"`
	Data      rekorDataSpec      `json:"data"`
}

type rekorSignatureSpec struct {
	Content   string             `json:"content"`
	PublicKey rekorPublicKeySpec `json:"publicKey"`
}

type rekorPublicKeySpec struct {
	Content string `json:"content"`
}

type rekorDataSpec struct {
	Hash rekorHashSpec `json:"hash"`
}

type rekorHashSpec struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

// Upload records a signing event in the Rekor transparency log by creating a HashedRekord entry.
func (c *HTTPRekorClient) Upload(ctx context.Context, rekorURL string, entry *RekorEntry) (*RekorResponse, error) {
	// Encode signature and certificate as base64 for the Rekor API.
	sigB64 := base64.StdEncoding.EncodeToString(entry.Signature)
	certB64 := base64.StdEncoding.EncodeToString(entry.Certificate)

	// Compute SHA-256 hash of the image reference for the data field.
	imageHash := hashForECDSA([]byte(entry.ImageRef))
	hashHex := fmt.Sprintf("%x", imageHash)

	reqBody := rekorCreateEntryRequest{
		APIVersion: "0.0.1",
		Kind:       "hashedrekord",
		Spec: rekorHashedRekordSpec{
			Signature: rekorSignatureSpec{
				Content: sigB64,
				PublicKey: rekorPublicKeySpec{
					Content: certB64,
				},
			},
			Data: rekorDataSpec{
				Hash: rekorHashSpec{
					Algorithm: "sha256",
					Value:     hashHex,
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Rekor request: %w", err)
	}

	// POST to Rekor /api/v1/log/entries.
	endpoint := fmt.Sprintf("%s/api/v1/log/entries", rekorURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create Rekor request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Rekor request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Rekor response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("Rekor returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	// Rekor returns a map of UUID → entry. Extract the first (only) UUID.
	var entryMap map[string]json.RawMessage
	if err := json.Unmarshal(respBody, &entryMap); err != nil {
		return nil, fmt.Errorf("failed to parse Rekor response: %w", err)
	}

	for uuid := range entryMap {
		return &RekorResponse{EntryUUID: uuid}, nil
	}

	return nil, fmt.Errorf("Rekor returned empty entry map")
}
