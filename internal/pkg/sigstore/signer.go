// Package sigstore provides Cosign signing, Fulcio certificate, and Rekor upload capabilities.
package sigstore

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"
)

// SigningOptions contains all parameters needed to sign an image.
type SigningOptions struct {
	// ImageRef is the fully qualified image reference including digest.
	// e.g., "registry.platform.cuscal.io/platform/myapp@sha256:..."
	ImageRef string

	// TokenPath is the filesystem path to the projected SA token.
	// Default: /var/run/secrets/tokens/fulcio-token
	TokenPath string

	// FulcioURL is the Fulcio CA endpoint.
	FulcioURL string

	// RekorURL is the Rekor transparency log endpoint.
	RekorURL string

	// Annotations are key-value pairs to attach to the signature.
	// Expected keys: scan-report, trivy-db, policy, timestamp
	Annotations map[string]string
}

// SigningResult contains the outcome of a successful signing operation.
type SigningResult struct {
	// RekorEntryUUID is the unique identifier of the Rekor transparency log entry.
	RekorEntryUUID string

	// SignedAt is the timestamp when the signing operation completed.
	SignedAt time.Time
}

// Signer handles the signing pipeline: Fulcio cert request, cosign signing, Rekor upload.
type Signer interface {
	Sign(ctx context.Context, opts SigningOptions) (*SigningResult, error)
}

// FulcioClient abstracts interactions with the Fulcio CA.
type FulcioClient interface {
	// GetCertificate requests a short-lived signing certificate from Fulcio.
	// token is the projected SA token content, fulcioURL is the Fulcio endpoint.
	GetCertificate(ctx context.Context, token string, fulcioURL string) (*SigningCertificate, error)
}

// SigningCertificate represents a short-lived certificate issued by Fulcio.
type SigningCertificate struct {
	// CertPEM is the PEM-encoded certificate.
	CertPEM []byte
	// ChainPEM is the PEM-encoded certificate chain.
	ChainPEM []byte
	// PrivateKey is the ephemeral private key used for signing.
	PrivateKey []byte
}

// RekorClient abstracts interactions with the Rekor transparency log.
type RekorClient interface {
	// Upload records a signing event in the Rekor transparency log.
	Upload(ctx context.Context, rekorURL string, entry *RekorEntry) (*RekorResponse, error)
}

// RekorEntry contains the data to upload to the Rekor transparency log.
type RekorEntry struct {
	// ImageRef is the signed image reference.
	ImageRef string
	// Signature is the cosign signature bytes.
	Signature []byte
	// Certificate is the Fulcio certificate used for signing.
	Certificate []byte
}

// RekorResponse contains the result of a Rekor upload.
type RekorResponse struct {
	// EntryUUID is the unique identifier of the Rekor log entry.
	EntryUUID string
}

// CosignSigner abstracts the actual image signing operation.
type CosignSigner interface {
	// SignImage signs the specified image using the provided certificate and annotations.
	SignImage(ctx context.Context, opts CosignSignOptions) (*CosignSignResult, error)
}

// CosignSignOptions contains options for the cosign signing operation.
type CosignSignOptions struct {
	// ImageRef is the fully qualified image reference to sign.
	ImageRef string
	// Certificate is the signing certificate from Fulcio.
	Certificate *SigningCertificate
	// Annotations are key-value pairs to attach to the signature.
	Annotations map[string]string
}

// CosignSignResult contains the result of a cosign signing operation.
type CosignSignResult struct {
	// Signature is the raw signature bytes.
	Signature []byte
}

// TokenProvider abstracts reading and validating projected SA tokens.
type TokenProvider interface {
	// ReadToken reads the projected SA token from the specified path.
	ReadToken(path string) (string, error)
	// ValidateToken checks if the token is still valid (not expired).
	ValidateToken(token string) error
}

// MetricsRecorder abstracts metric counter increments for the signer.
type MetricsRecorder interface {
	// IncrementFulcioErrors increments the fulcio_errors_total counter.
	IncrementFulcioErrors()
	// IncrementRekorErrors increments the rekor_errors_total counter.
	IncrementRekorErrors()
	// IncrementSigningFailures increments the signing_failures_total counter.
	IncrementSigningFailures()
}

// RetryConfig defines parameters for exponential backoff retry.
type RetryConfig struct {
	// MaxAttempts is the maximum number of attempts (including the initial try).
	MaxAttempts int
	// BaseDelay is the initial delay between retries.
	BaseDelay time.Duration
	// MaxDelay is the maximum delay between retries.
	MaxDelay time.Duration
}

// DefaultRetryConfig returns the default retry configuration for Fulcio and Rekor.
// 3 attempts, 1s base delay, 8s max delay.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Second,
		MaxDelay:    8 * time.Second,
	}
}

// DefaultSigner orchestrates the full signing pipeline using pluggable components.
type DefaultSigner struct {
	fulcio        FulcioClient
	rekor         RekorClient
	cosign        CosignSigner
	tokenProvider TokenProvider
	metrics       MetricsRecorder
	retryConfig   RetryConfig
}

// DefaultSignerOption configures a DefaultSigner.
type DefaultSignerOption func(*DefaultSigner)

// WithFulcioClient sets the Fulcio client.
func WithFulcioClient(c FulcioClient) DefaultSignerOption {
	return func(s *DefaultSigner) { s.fulcio = c }
}

// WithRekorClient sets the Rekor client.
func WithRekorClient(c RekorClient) DefaultSignerOption {
	return func(s *DefaultSigner) { s.rekor = c }
}

// WithCosignSigner sets the cosign signer.
func WithCosignSigner(c CosignSigner) DefaultSignerOption {
	return func(s *DefaultSigner) { s.cosign = c }
}

// WithTokenProvider sets the token provider.
func WithTokenProvider(t TokenProvider) DefaultSignerOption {
	return func(s *DefaultSigner) { s.tokenProvider = t }
}

// WithMetricsRecorder sets the metrics recorder.
func WithMetricsRecorder(m MetricsRecorder) DefaultSignerOption {
	return func(s *DefaultSigner) { s.metrics = m }
}

// WithRetryConfig sets the retry configuration.
func WithRetryConfig(r RetryConfig) DefaultSignerOption {
	return func(s *DefaultSigner) { s.retryConfig = r }
}

// NewDefaultSigner creates a new DefaultSigner with the given options.
func NewDefaultSigner(opts ...DefaultSignerOption) *DefaultSigner {
	s := &DefaultSigner{
		retryConfig: DefaultRetryConfig(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Sign orchestrates the full signing pipeline:
// 1. Read and validate the projected SA token
// 2. Request a certificate from Fulcio (with retry)
// 3. Sign the image with cosign
// 4. Upload the entry to Rekor (with retry)
// 5. Return the signing result
func (s *DefaultSigner) Sign(ctx context.Context, opts SigningOptions) (*SigningResult, error) {
	// Step 1: Read and validate the projected SA token.
	token, err := s.tokenProvider.ReadToken(opts.TokenPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SA token from %s: %w", opts.TokenPath, err)
	}

	if err := s.tokenProvider.ValidateToken(token); err != nil {
		// Token expired/invalid — fail immediately without retrying with stale credentials.
		return nil, fmt.Errorf("SA token is expired or invalid: %w", err)
	}

	// Step 2: Request signing certificate from Fulcio with retry.
	cert, err := s.requestCertificateWithRetry(ctx, token, opts.FulcioURL)
	if err != nil {
		// All Fulcio retries exhausted — emit signing_failures_total.
		if s.metrics != nil {
			s.metrics.IncrementSigningFailures()
		}
		return nil, fmt.Errorf("fulcio certificate request failed after retries: %w", err)
	}

	// Step 3: Sign the image using cosign with the obtained certificate.
	// Ensure annotations include the required fields.
	annotations := ensureAnnotations(opts.Annotations)

	signResult, err := s.cosign.SignImage(ctx, CosignSignOptions{
		ImageRef:    opts.ImageRef,
		Certificate: cert,
		Annotations: annotations,
	})
	if err != nil {
		if s.metrics != nil {
			s.metrics.IncrementSigningFailures()
		}
		return nil, fmt.Errorf("cosign signing failed: %w", err)
	}

	// Step 4: Upload to Rekor transparency log with retry.
	rekorResp, err := s.uploadToRekorWithRetry(ctx, opts.RekorURL, &RekorEntry{
		ImageRef:    opts.ImageRef,
		Signature:   signResult.Signature,
		Certificate: cert.CertPEM,
	})
	if err != nil {
		// All Rekor retries exhausted — emit signing_failures_total.
		if s.metrics != nil {
			s.metrics.IncrementSigningFailures()
		}
		return nil, fmt.Errorf("rekor upload failed after retries: %w", err)
	}

	// Step 5: Return signing result.
	return &SigningResult{
		RekorEntryUUID: rekorResp.EntryUUID,
		SignedAt:       time.Now().UTC(),
	}, nil
}

// requestCertificateWithRetry attempts to get a certificate from Fulcio with exponential backoff.
func (s *DefaultSigner) requestCertificateWithRetry(ctx context.Context, token, fulcioURL string) (*SigningCertificate, error) {
	var lastErr error

	for attempt := 0; attempt < s.retryConfig.MaxAttempts; attempt++ {
		if attempt > 0 {
			delay := s.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled during Fulcio retry: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		cert, err := s.fulcio.GetCertificate(ctx, token, fulcioURL)
		if err == nil {
			return cert, nil
		}

		lastErr = err
		// Increment fulcio_errors_total on each failed attempt.
		if s.metrics != nil {
			s.metrics.IncrementFulcioErrors()
		}
	}

	return nil, fmt.Errorf("fulcio: all %d attempts failed: %w", s.retryConfig.MaxAttempts, lastErr)
}

// uploadToRekorWithRetry attempts to upload to Rekor with exponential backoff.
func (s *DefaultSigner) uploadToRekorWithRetry(ctx context.Context, rekorURL string, entry *RekorEntry) (*RekorResponse, error) {
	var lastErr error

	for attempt := 0; attempt < s.retryConfig.MaxAttempts; attempt++ {
		if attempt > 0 {
			delay := s.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled during Rekor retry: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		resp, err := s.rekor.Upload(ctx, rekorURL, entry)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		// Increment rekor_errors_total on each failed attempt.
		if s.metrics != nil {
			s.metrics.IncrementRekorErrors()
		}
	}

	return nil, fmt.Errorf("rekor: all %d attempts failed: %w", s.retryConfig.MaxAttempts, lastErr)
}

// calculateBackoff computes the delay for a given retry attempt using exponential backoff with jitter.
func (s *DefaultSigner) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: baseDelay * 2^(attempt-1)
	backoff := float64(s.retryConfig.BaseDelay) * math.Pow(2, float64(attempt-1))

	// Cap at max delay.
	if backoff > float64(s.retryConfig.MaxDelay) {
		backoff = float64(s.retryConfig.MaxDelay)
	}

	// Add jitter: ±25% of the backoff.
	jitter := backoff * 0.25 * (rand.Float64()*2 - 1) //nolint:gosec // jitter does not need crypto-strength randomness
	backoff += jitter

	// Ensure non-negative.
	if backoff < 0 {
		backoff = float64(s.retryConfig.BaseDelay)
	}

	return time.Duration(backoff)
}

// ensureAnnotations returns a copy of the annotations map ensuring the required keys are present.
// Required annotations: scan-report, trivy-db, policy, timestamp (RFC 3339 UTC).
func ensureAnnotations(annotations map[string]string) map[string]string {
	result := make(map[string]string, len(annotations)+1)
	for k, v := range annotations {
		result[k] = v
	}

	// Ensure timestamp annotation is present in RFC 3339 UTC.
	if _, ok := result["timestamp"]; !ok {
		result["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	}

	return result
}

// RequiredAnnotationKeys returns the set of annotation keys that must be present on signed images.
func RequiredAnnotationKeys() []string {
	return []string{"scan-report", "trivy-db", "policy", "timestamp"}
}

// ValidateAnnotations checks that all required annotation keys are present and the timestamp
// is a valid RFC 3339 UTC value.
func ValidateAnnotations(annotations map[string]string) error {
	required := RequiredAnnotationKeys()
	for _, key := range required {
		val, ok := annotations[key]
		if !ok {
			return fmt.Errorf("missing required annotation: %s", key)
		}
		if val == "" {
			return fmt.Errorf("empty required annotation: %s", key)
		}
	}

	// Validate timestamp format.
	ts := annotations["timestamp"]
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return fmt.Errorf("invalid timestamp annotation format (must be RFC 3339): %w", err)
	}

	// Ensure timestamp is in UTC.
	if t.Location() != time.UTC {
		return fmt.Errorf("timestamp annotation must be in UTC, got %s", t.Location())
	}

	return nil
}

// FileTokenProvider implements TokenProvider by reading tokens from the filesystem.
type FileTokenProvider struct{}

// ReadToken reads the projected SA token from the filesystem.
func (f *FileTokenProvider) ReadToken(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read token file %s: %w", path, err)
	}

	token := string(data)
	if token == "" {
		return "", fmt.Errorf("token file %s is empty", path)
	}

	return token, nil
}

// ValidateToken performs basic JWT expiry validation.
// In production, this parses the JWT claims to check the exp field.
// Returns an error if the token is expired or malformed.
func (f *FileTokenProvider) ValidateToken(token string) error {
	if token == "" {
		return fmt.Errorf("token is empty")
	}

	// Parse JWT to check expiry. A full JWT library would be used in production;
	// here we use a lightweight check by decoding the payload segment.
	expiry, err := extractJWTExpiry(token)
	if err != nil {
		return fmt.Errorf("failed to parse token expiry: %w", err)
	}

	if time.Now().After(expiry) {
		return fmt.Errorf("token expired at %s", expiry.Format(time.RFC3339))
	}

	return nil
}

// extractJWTExpiry decodes the JWT payload and extracts the exp claim.
func extractJWTExpiry(token string) (time.Time, error) {
	parts := splitJWT(token)
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("invalid JWT format: expected 3 parts, got %d", len(parts))
	}

	payload, err := base64Decode(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	exp, err := extractExpFromJSON(payload)
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(exp, 0), nil
}
