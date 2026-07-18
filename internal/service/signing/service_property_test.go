package signing

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"go.uber.org/zap"

	"github.com/signer-service/internal/pkg/harbor"
	"github.com/signer-service/internal/pkg/sigstore"
	"github.com/signer-service/internal/service/gate"
)

// Feature: signer-service, Property 4: Signing idempotency — duplicate webhooks produce no additional signatures
// **Validates: Requirements 5.1, 5.4**

// mockHarborClientAlreadySigned always reports that a signature already exists.
type mockHarborClientAlreadySigned struct{}

func (m *mockHarborClientAlreadySigned) GetScanReport(_ context.Context, _, _, _ string) (*harbor.ScanReport, error) {
	// Should never be called when signature already exists.
	return &harbor.ScanReport{}, nil
}

func (m *mockHarborClientAlreadySigned) ListArtifacts(_ context.Context, _ string) ([]harbor.Artifact, error) {
	return nil, nil
}

func (m *mockHarborClientAlreadySigned) HasSignature(_ context.Context, _, _, _ string) (bool, error) {
	return true, nil
}

// mockSignerRecorder records whether Sign was called.
type mockSignerRecorder struct {
	signCalled atomic.Int32
}

func (m *mockSignerRecorder) Sign(_ context.Context, _ sigstore.SigningOptions) (*sigstore.SigningResult, error) {
	m.signCalled.Add(1)
	return &sigstore.SigningResult{
		RekorEntryUUID: "mock-uuid",
		SignedAt:       time.Now(),
	}, nil
}

// mockMetricsRecorder tracks IncrementSignaturesIssued calls.
type mockMetricsRecorder struct {
	signaturesIssued atomic.Int32
	gateFailures     atomic.Int32
}

func (m *mockMetricsRecorder) IncrementSignaturesIssued(_ string) {
	m.signaturesIssued.Add(1)
}

func (m *mockMetricsRecorder) IncrementGateFailures(_ string) {
	m.gateFailures.Add(1)
}

func (m *mockMetricsRecorder) ObserveWebhookLatency(_ time.Duration) {}

// mockEvaluatorPass always passes (should never be reached in this test).
type mockEvaluatorPass struct{}

func (m *mockEvaluatorPass) Evaluate(_ *harbor.ScanReport) gate.GateDecision {
	return gate.GateDecision{Pass: true}
}

// TestPropertySigningIdempotency verifies that when a signature already exists for an artifact,
// the signing pipeline produces zero new signatures, zero Rekor entries, and no metric increment.
func TestPropertySigningIdempotency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generators for random artifact reference components.
	// Use gen.Identifier() which always produces non-empty alphanumeric strings.
	genProject := gen.Identifier()
	genRepo := gen.Identifier()
	genDigest := gen.Identifier().Map(func(s string) string { return "sha256:" + s })

	properties.Property("duplicate webhook with existing signature produces no new signing activity", prop.ForAll(
		func(project, repo, digest string) bool {
			// Set up mocks
			harborClient := &mockHarborClientAlreadySigned{}
			signerMock := &mockSignerRecorder{}
			metricsMock := &mockMetricsRecorder{}
			evaluatorMock := &mockEvaluatorPass{}
			logger := zap.NewNop()

			svc := NewSigningService(
				harborClient,
				signerMock,
				evaluatorMock,
				logger,
				metricsMock,
				SigningConfig{
					HarborURL:   "https://harbor.example.com",
					FulcioURL:   "https://fulcio.example.com",
					RekorURL:    "https://rekor.example.com",
					TokenPath:   "/tmp/token",
					RegistryURL: "registry.example.com",
				},
			)

			ref := gate.ArtifactRef{
				Project: project,
				Repo:    repo,
				Digest:  digest,
			}

			err := svc.ProcessArtifact(context.Background(), ref, "webhook")

			// Assert: no error returned
			if err != nil {
				return false
			}
			// Assert: Signer.Sign was never called
			if signerMock.signCalled.Load() != 0 {
				return false
			}
			// Assert: metrics.IncrementSignaturesIssued was never called
			if metricsMock.signaturesIssued.Load() != 0 {
				return false
			}
			return true
		},
		genProject,
		genRepo,
		genDigest,
	))

	properties.TestingRun(t)
}
