package sigstore

import (
	"context"
	"errors"
	"testing"
	"time"
)

// --- Mock implementations ---

// mockFulcioClient implements FulcioClient with configurable per-call results.
type mockFulcioClient struct {
	callCount int
	results   []fulcioCallResult
}

type fulcioCallResult struct {
	cert *SigningCertificate
	err  error
}

func (m *mockFulcioClient) GetCertificate(_ context.Context, _ string, _ string) (*SigningCertificate, error) {
	idx := m.callCount
	m.callCount++
	if idx < len(m.results) {
		return m.results[idx].cert, m.results[idx].err
	}
	return nil, errors.New("unexpected call to FulcioClient")
}

// mockRekorClient implements RekorClient with configurable per-call results.
type mockRekorClient struct {
	callCount int
	results   []rekorCallResult
}

type rekorCallResult struct {
	resp *RekorResponse
	err  error
}

func (m *mockRekorClient) Upload(_ context.Context, _ string, _ *RekorEntry) (*RekorResponse, error) {
	idx := m.callCount
	m.callCount++
	if idx < len(m.results) {
		return m.results[idx].resp, m.results[idx].err
	}
	return nil, errors.New("unexpected call to RekorClient")
}

// mockCosignSigner implements CosignSigner, always returning success.
type mockCosignSigner struct {
	called bool
}

func (m *mockCosignSigner) SignImage(_ context.Context, _ CosignSignOptions) (*CosignSignResult, error) {
	m.called = true
	return &CosignSignResult{Signature: []byte("mock-signature")}, nil
}

// mockTokenProvider implements TokenProvider with configurable responses.
type mockTokenProvider struct {
	token         string
	readErr       error
	validateErr   error
	validateCalls int
}

func (m *mockTokenProvider) ReadToken(_ string) (string, error) {
	if m.readErr != nil {
		return "", m.readErr
	}
	return m.token, nil
}

func (m *mockTokenProvider) ValidateToken(_ string) error {
	m.validateCalls++
	return m.validateErr
}

// mockMetricsRecorder implements MetricsRecorder with counters.
type mockMetricsRecorder struct {
	fulcioErrors    int
	rekorErrors     int
	signingFailures int
}

func (m *mockMetricsRecorder) IncrementFulcioErrors()    { m.fulcioErrors++ }
func (m *mockMetricsRecorder) IncrementRekorErrors()     { m.rekorErrors++ }
func (m *mockMetricsRecorder) IncrementSigningFailures() { m.signingFailures++ }

// --- Helper ---

func testRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    50 * time.Millisecond,
	}
}

func defaultSigningOptions() SigningOptions {
	return SigningOptions{
		ImageRef:    "registry.platform.cuscal.io/platform/myapp@sha256:abc123",
		TokenPath:   "/var/run/secrets/tokens/fulcio-token",
		FulcioURL:   "https://fulcio.platform.cuscal.io",
		RekorURL:    "https://rekor.platform.cuscal.io",
		Annotations: map[string]string{"scan-report": "digest1", "trivy-db": "v2", "policy": "high"},
	}
}

func successCert() *SigningCertificate {
	return &SigningCertificate{
		CertPEM:    []byte("cert-pem"),
		ChainPEM:   []byte("chain-pem"),
		PrivateKey: []byte("private-key"),
	}
}

// --- Tests ---

// TestSign_SuccessfulFlow verifies that when all components succeed,
// Sign returns a valid SigningResult with RekorEntryUUID and SignedAt.
func TestSign_SuccessfulFlow(t *testing.T) {
	fulcio := &mockFulcioClient{
		results: []fulcioCallResult{
			{cert: successCert(), err: nil},
		},
	}
	rekor := &mockRekorClient{
		results: []rekorCallResult{
			{resp: &RekorResponse{EntryUUID: "entry-uuid-123"}, err: nil},
		},
	}
	cosign := &mockCosignSigner{}
	tokens := &mockTokenProvider{token: "valid-token"}
	metrics := &mockMetricsRecorder{}

	signer := NewDefaultSigner(
		WithFulcioClient(fulcio),
		WithRekorClient(rekor),
		WithCosignSigner(cosign),
		WithTokenProvider(tokens),
		WithMetricsRecorder(metrics),
		WithRetryConfig(testRetryConfig()),
	)

	result, err := signer.Sign(context.Background(), defaultSigningOptions())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.RekorEntryUUID != "entry-uuid-123" {
		t.Errorf("expected RekorEntryUUID 'entry-uuid-123', got '%s'", result.RekorEntryUUID)
	}

	if result.SignedAt.IsZero() {
		t.Error("expected non-zero SignedAt timestamp")
	}

	// Verify SignedAt is recent (within 5 seconds of now).
	if time.Since(result.SignedAt) > 5*time.Second {
		t.Errorf("SignedAt is too far in the past: %v", result.SignedAt)
	}

	if !cosign.called {
		t.Error("expected cosign SignImage to be called")
	}

	// No errors should have been recorded.
	if metrics.fulcioErrors != 0 {
		t.Errorf("expected 0 fulcio errors, got %d", metrics.fulcioErrors)
	}
	if metrics.rekorErrors != 0 {
		t.Errorf("expected 0 rekor errors, got %d", metrics.rekorErrors)
	}
	if metrics.signingFailures != 0 {
		t.Errorf("expected 0 signing failures, got %d", metrics.signingFailures)
	}
}

// TestSign_FulcioRetryAndFailure verifies that when Fulcio fails all 3 attempts,
// an error is returned and the correct metrics are incremented.
func TestSign_FulcioRetryAndFailure(t *testing.T) {
	fulcioErr := errors.New("fulcio connection refused")
	fulcio := &mockFulcioClient{
		results: []fulcioCallResult{
			{cert: nil, err: fulcioErr},
			{cert: nil, err: fulcioErr},
			{cert: nil, err: fulcioErr},
		},
	}
	rekor := &mockRekorClient{}
	cosign := &mockCosignSigner{}
	tokens := &mockTokenProvider{token: "valid-token"}
	metrics := &mockMetricsRecorder{}

	signer := NewDefaultSigner(
		WithFulcioClient(fulcio),
		WithRekorClient(rekor),
		WithCosignSigner(cosign),
		WithTokenProvider(tokens),
		WithMetricsRecorder(metrics),
		WithRetryConfig(testRetryConfig()),
	)

	result, err := signer.Sign(context.Background(), defaultSigningOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Fatal("expected nil result on failure")
	}

	// Fulcio errors: one per failed attempt = 3.
	if metrics.fulcioErrors != 3 {
		t.Errorf("expected 3 fulcio errors, got %d", metrics.fulcioErrors)
	}

	// Signing failure: emitted once after all retries are exhausted.
	if metrics.signingFailures != 1 {
		t.Errorf("expected 1 signing failure, got %d", metrics.signingFailures)
	}

	// Cosign should not have been called.
	if cosign.called {
		t.Error("expected cosign SignImage NOT to be called")
	}

	// Rekor should not have been called.
	if rekor.callCount != 0 {
		t.Errorf("expected 0 rekor calls, got %d", rekor.callCount)
	}
}

// TestSign_RekorRetryAndFailure verifies that when Rekor fails all 3 attempts
// (Fulcio and cosign succeed), an error is returned with correct metrics.
func TestSign_RekorRetryAndFailure(t *testing.T) {
	fulcio := &mockFulcioClient{
		results: []fulcioCallResult{
			{cert: successCert(), err: nil},
		},
	}
	rekorErr := errors.New("rekor unavailable")
	rekor := &mockRekorClient{
		results: []rekorCallResult{
			{resp: nil, err: rekorErr},
			{resp: nil, err: rekorErr},
			{resp: nil, err: rekorErr},
		},
	}
	cosign := &mockCosignSigner{}
	tokens := &mockTokenProvider{token: "valid-token"}
	metrics := &mockMetricsRecorder{}

	signer := NewDefaultSigner(
		WithFulcioClient(fulcio),
		WithRekorClient(rekor),
		WithCosignSigner(cosign),
		WithTokenProvider(tokens),
		WithMetricsRecorder(metrics),
		WithRetryConfig(testRetryConfig()),
	)

	result, err := signer.Sign(context.Background(), defaultSigningOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Fatal("expected nil result on failure")
	}

	// Cosign should have been called (Fulcio succeeded).
	if !cosign.called {
		t.Error("expected cosign SignImage to be called")
	}

	// Rekor errors: one per failed attempt = 3.
	if metrics.rekorErrors != 3 {
		t.Errorf("expected 3 rekor errors, got %d", metrics.rekorErrors)
	}

	// Signing failure: emitted once after all Rekor retries exhausted.
	if metrics.signingFailures != 1 {
		t.Errorf("expected 1 signing failure, got %d", metrics.signingFailures)
	}

	// No Fulcio errors (it succeeded first try).
	if metrics.fulcioErrors != 0 {
		t.Errorf("expected 0 fulcio errors, got %d", metrics.fulcioErrors)
	}
}

// TestSign_FulcioRetryEventualSuccess verifies that when Fulcio fails the first
// 2 attempts but succeeds on the 3rd, signing completes successfully.
func TestSign_FulcioRetryEventualSuccess(t *testing.T) {
	fulcioErr := errors.New("fulcio temporary error")
	fulcio := &mockFulcioClient{
		results: []fulcioCallResult{
			{cert: nil, err: fulcioErr},
			{cert: nil, err: fulcioErr},
			{cert: successCert(), err: nil},
		},
	}
	rekor := &mockRekorClient{
		results: []rekorCallResult{
			{resp: &RekorResponse{EntryUUID: "rekor-entry-456"}, err: nil},
		},
	}
	cosign := &mockCosignSigner{}
	tokens := &mockTokenProvider{token: "valid-token"}
	metrics := &mockMetricsRecorder{}

	signer := NewDefaultSigner(
		WithFulcioClient(fulcio),
		WithRekorClient(rekor),
		WithCosignSigner(cosign),
		WithTokenProvider(tokens),
		WithMetricsRecorder(metrics),
		WithRetryConfig(testRetryConfig()),
	)

	result, err := signer.Sign(context.Background(), defaultSigningOptions())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.RekorEntryUUID != "rekor-entry-456" {
		t.Errorf("expected RekorEntryUUID 'rekor-entry-456', got '%s'", result.RekorEntryUUID)
	}

	// 2 Fulcio errors (first two failed attempts).
	if metrics.fulcioErrors != 2 {
		t.Errorf("expected 2 fulcio errors, got %d", metrics.fulcioErrors)
	}

	// No signing failure (eventual success).
	if metrics.signingFailures != 0 {
		t.Errorf("expected 0 signing failures, got %d", metrics.signingFailures)
	}

	// Fulcio should have been called 3 times total.
	if fulcio.callCount != 3 {
		t.Errorf("expected 3 fulcio calls, got %d", fulcio.callCount)
	}
}

// TestSign_ExpiredTokenRejection verifies that when ValidateToken returns an error,
// Sign returns immediately without calling Fulcio, Rekor, or cosign.
func TestSign_ExpiredTokenRejection(t *testing.T) {
	fulcio := &mockFulcioClient{}
	rekor := &mockRekorClient{}
	cosign := &mockCosignSigner{}
	tokens := &mockTokenProvider{
		token:       "expired-token",
		validateErr: errors.New("token expired at 2024-01-01T00:00:00Z"),
	}
	metrics := &mockMetricsRecorder{}

	signer := NewDefaultSigner(
		WithFulcioClient(fulcio),
		WithRekorClient(rekor),
		WithCosignSigner(cosign),
		WithTokenProvider(tokens),
		WithMetricsRecorder(metrics),
		WithRetryConfig(testRetryConfig()),
	)

	result, err := signer.Sign(context.Background(), defaultSigningOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Fatal("expected nil result on token validation failure")
	}

	// Error message should indicate token issue.
	if !containsSubstring(err.Error(), "expired or invalid") {
		t.Errorf("expected error to mention 'expired or invalid', got: %v", err)
	}

	// Fulcio should NOT have been called.
	if fulcio.callCount != 0 {
		t.Errorf("expected 0 fulcio calls, got %d", fulcio.callCount)
	}

	// Rekor should NOT have been called.
	if rekor.callCount != 0 {
		t.Errorf("expected 0 rekor calls, got %d", rekor.callCount)
	}

	// Cosign should NOT have been called.
	if cosign.called {
		t.Error("expected cosign SignImage NOT to be called")
	}

	// No metrics should be incremented for an auth failure.
	if metrics.fulcioErrors != 0 {
		t.Errorf("expected 0 fulcio errors, got %d", metrics.fulcioErrors)
	}
	if metrics.rekorErrors != 0 {
		t.Errorf("expected 0 rekor errors, got %d", metrics.rekorErrors)
	}
	if metrics.signingFailures != 0 {
		t.Errorf("expected 0 signing failures, got %d", metrics.signingFailures)
	}
}

// containsSubstring checks if s contains substr.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && contains(s, substr))
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
