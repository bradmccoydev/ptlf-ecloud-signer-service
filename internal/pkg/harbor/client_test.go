package harbor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// testClientConfig returns a ClientConfig with short retry delays suitable for tests.
func testClientConfig(baseURL string) ClientConfig {
	return ClientConfig{
		BaseURL:        baseURL,
		Username:       "robot$signer",
		Password:       "test-password",
		MaxRetries:     3,
		RetryBaseDelay: 10 * time.Millisecond,
		RetryMaxDelay:  50 * time.Millisecond,
	}
}

func TestGetScanReport_Success(t *testing.T) {
	report := &ScanReport{
		Vulnerabilities: []Vulnerability{
			{ID: "CVE-2024-0001", Severity: "Medium", Package: "openssl", Version: "3.0.1"},
			{ID: "CVE-2024-0002", Severity: "Low", Package: "zlib", Version: "1.2.11"},
		},
		Scanner: Scanner{Name: "Trivy", Version: "0.50.0"},
	}
	// Harbor returns a map of scanner-name → report
	reportMap := map[string]*ScanReport{
		"application/vnd.security.vulnerability.report; version=1.1": report,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify basic auth
		user, pass, ok := r.BasicAuth()
		if !ok || user != "robot$signer" || pass != "test-password" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(reportMap)
	}))
	defer server.Close()

	client := NewClient(testClientConfig(server.URL))
	result, err := client.GetScanReport(context.Background(), "platform", "myapp", "sha256:abc123")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil scan report")
	}
	if len(result.Vulnerabilities) != 2 {
		t.Fatalf("expected 2 vulnerabilities, got %d", len(result.Vulnerabilities))
	}
	if result.Vulnerabilities[0].ID != "CVE-2024-0001" {
		t.Errorf("expected CVE-2024-0001, got %s", result.Vulnerabilities[0].ID)
	}
	if result.Scanner.Name != "Trivy" {
		t.Errorf("expected scanner name Trivy, got %s", result.Scanner.Name)
	}
}

func TestGetScanReport_RetryOn5xx(t *testing.T) {
	var attempts int32

	report := &ScanReport{
		Vulnerabilities: []Vulnerability{
			{ID: "CVE-2024-0001", Severity: "Low", Package: "curl", Version: "7.80"},
		},
		Scanner: Scanner{Name: "Trivy", Version: "0.50.0"},
	}
	reportMap := map[string]*ScanReport{
		"application/vnd.security.vulnerability.report; version=1.1": report,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(reportMap)
	}))
	defer server.Close()

	client := NewClient(testClientConfig(server.URL))
	result, err := client.GetScanReport(context.Background(), "platform", "myapp", "sha256:abc123")
	if err != nil {
		t.Fatalf("expected no error after retry, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil scan report after retry")
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected 2 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestGetScanReport_NoRetryOn4xx(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(testClientConfig(server.URL))
	_, err := client.GetScanReport(context.Background(), "platform", "myapp", "sha256:notfound")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("expected 1 attempt (no retry on 4xx), got %d", atomic.LoadInt32(&attempts))
	}
}

func TestGetScanReport_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay longer than the context deadline
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(testClientConfig(server.URL))

	// Use a short context deadline to trigger timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.GetScanReport(ctx, "platform", "myapp", "sha256:abc123")
	if err == nil {
		t.Fatal("expected error due to context timeout")
	}
	// The error should be context-related (deadline exceeded)
	if ctx.Err() == nil {
		t.Error("expected context to be done")
	}
}

func TestHasSignature_ReturnsTrue(t *testing.T) {
	accessories := []accessory{
		{Type: "signature.cosign", Digest: "sha256:sig123"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(accessories)
	}))
	defer server.Close()

	client := NewClient(testClientConfig(server.URL))
	hasSig, err := client.HasSignature(context.Background(), "platform", "myapp", "sha256:abc123")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !hasSig {
		t.Error("expected HasSignature to return true")
	}
}

func TestHasSignature_ReturnsFalse_EmptyAccessories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]accessory{})
	}))
	defer server.Close()

	client := NewClient(testClientConfig(server.URL))
	hasSig, err := client.HasSignature(context.Background(), "platform", "myapp", "sha256:abc123")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if hasSig {
		t.Error("expected HasSignature to return false for empty accessories")
	}
}

func TestHasSignature_ReturnsFalse_NoCosignType(t *testing.T) {
	accessories := []accessory{
		{Type: "sbom.spdx", Digest: "sha256:sbom456"},
		{Type: "provenance.slsa", Digest: "sha256:prov789"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(accessories)
	}))
	defer server.Close()

	client := NewClient(testClientConfig(server.URL))
	hasSig, err := client.HasSignature(context.Background(), "platform", "myapp", "sha256:abc123")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if hasSig {
		t.Error("expected HasSignature to return false when no cosign signature type present")
	}
}

func TestHasSignature_RetryOn5xx(t *testing.T) {
	var attempts int32

	accessories := []accessory{
		{Type: "signature.cosign", Digest: "sha256:sig123"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(accessories)
	}))
	defer server.Close()

	client := NewClient(testClientConfig(server.URL))
	hasSig, err := client.HasSignature(context.Background(), "platform", "myapp", "sha256:abc123")
	if err != nil {
		t.Fatalf("expected no error after retries, got: %v", err)
	}
	if !hasSig {
		t.Error("expected HasSignature to return true after successful retry")
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestListArtifacts_FiltersSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/v2.0/projects/platform/repositories":
			// Return repos
			repos := []repository{
				{Name: "platform/myapp"},
				{Name: "platform/webapp"},
			}
			json.NewEncoder(w).Encode(repos)

		case r.URL.Path == "/api/v2.0/projects/platform/repositories/myapp/artifacts":
			// Return artifacts for myapp - one with Success scan, one without
			artifacts := []harborArtifact{
				{
					Digest: "sha256:aaa",
					Tags:   []Tag{{Name: "v1.0"}},
					ScanOverview: map[string]scanSummary{
						"trivy": {ScanStatus: "Success"},
					},
					ProjectID: 1,
				},
				{
					Digest: "sha256:bbb",
					Tags:   []Tag{{Name: "v0.9"}},
					ScanOverview: map[string]scanSummary{
						"trivy": {ScanStatus: "Running"},
					},
					ProjectID: 1,
				},
			}
			json.NewEncoder(w).Encode(artifacts)

		case r.URL.Path == "/api/v2.0/projects/platform/repositories/webapp/artifacts":
			// Return artifacts for webapp - all with Success
			artifacts := []harborArtifact{
				{
					Digest: "sha256:ccc",
					Tags:   []Tag{{Name: "latest"}},
					ScanOverview: map[string]scanSummary{
						"trivy": {ScanStatus: "Success"},
					},
					ProjectID: 1,
				},
			}
			json.NewEncoder(w).Encode(artifacts)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(testClientConfig(server.URL))
	artifacts, err := client.ListArtifacts(context.Background(), "platform")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should only include artifacts with scan status "Success" (aaa and ccc)
	if len(artifacts) != 2 {
		t.Fatalf("expected 2 artifacts with Success scan status, got %d", len(artifacts))
	}

	digests := make(map[string]bool)
	for _, a := range artifacts {
		digests[a.Digest] = true
	}
	if !digests["sha256:aaa"] {
		t.Error("expected artifact sha256:aaa to be included")
	}
	if !digests["sha256:ccc"] {
		t.Error("expected artifact sha256:ccc to be included")
	}
	if digests["sha256:bbb"] {
		t.Error("expected artifact sha256:bbb to be excluded (scan not Success)")
	}
}
