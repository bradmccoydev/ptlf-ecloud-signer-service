package reconciliation

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"go.uber.org/zap"

	"github.com/signer-service/internal/pkg/harbor"
	"github.com/signer-service/internal/service/gate"
)

// Feature: signer-service, Property 7: Reconciliation sweep skips already-signed artifacts
// **Validates: Requirements 5.1, 6.3**

// mockHarborClientForReconciliation returns a configurable set of artifacts
// and reports signature status based on a "signed set".
type mockHarborClientForReconciliation struct {
	artifacts map[string][]harbor.Artifact // project -> artifacts
	signedSet map[string]bool              // "project/repo@digest" -> true if signed
}

func (m *mockHarborClientForReconciliation) GetScanReport(_ context.Context, _, _, _ string) (*harbor.ScanReport, error) {
	return &harbor.ScanReport{}, nil
}

func (m *mockHarborClientForReconciliation) ListArtifacts(_ context.Context, project string) ([]harbor.Artifact, error) {
	return m.artifacts[project], nil
}

func (m *mockHarborClientForReconciliation) HasSignature(_ context.Context, project, repo, digest string) (bool, error) {
	key := fmt.Sprintf("%s/%s@%s", project, repo, digest)
	return m.signedSet[key], nil
}

// mockProcessorRecorder records which artifacts had ProcessArtifact called.
type mockProcessorRecorder struct {
	mu        sync.Mutex
	processed []gate.ArtifactRef
}

func (m *mockProcessorRecorder) ProcessArtifact(_ context.Context, ref gate.ArtifactRef, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.processed = append(m.processed, ref)
	return nil
}

func (m *mockProcessorRecorder) getProcessed() []gate.ArtifactRef {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]gate.ArtifactRef, len(m.processed))
	copy(result, m.processed)
	return result
}

// mockReconcilerMetrics is a no-op metrics implementation for testing.
type mockReconcilerMetrics struct{}

func (m *mockReconcilerMetrics) IncrementReconciliationSigned() {}
func (m *mockReconcilerMetrics) IncrementReconciliationErrors() {}

// TestPropertyReconciliationSkipsAlreadySigned verifies that the reconciliation sweep
// only invokes ProcessArtifact for artifacts that do NOT already have signatures,
// and never calls ProcessArtifact for artifacts that are already signed.
func TestPropertyReconciliationSkipsAlreadySigned(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator for artifact count (1-20)
	genArtifactCount := gen.IntRange(1, 20)

	// Generator for signed flags
	genSignedFlags := gen.SliceOfN(20, gen.Bool())

	properties.Property("reconciliation sweep only processes unsigned artifacts", prop.ForAll(
		func(artifactCount int, signedFlags []bool) bool {
			// Ensure we have enough generated flags
			if len(signedFlags) < artifactCount {
				return true // skip if generator didn't produce enough
			}

			project := "test-project"

			// Build unique artifacts with deterministic repos and digests to avoid
			// duplicate key issues during shrinking.
			artifacts := make([]harbor.Artifact, artifactCount)
			signedSet := make(map[string]bool)
			expectedUnsigned := make(map[string]bool)

			for i := 0; i < artifactCount; i++ {
				repo := fmt.Sprintf("repo-%d", i)
				digest := fmt.Sprintf("sha256:digest%d", i)
				isSigned := signedFlags[i]

				artifacts[i] = harbor.Artifact{
					Digest:     digest,
					Repo:       repo,
					ScanStatus: "Success",
				}

				key := fmt.Sprintf("%s/%s@%s", project, repo, digest)
				if isSigned {
					signedSet[key] = true
				} else {
					expectedUnsigned[key] = true
				}
			}

			// Set up mocks
			harborClient := &mockHarborClientForReconciliation{
				artifacts: map[string][]harbor.Artifact{
					project: artifacts,
				},
				signedSet: signedSet,
			}
			processor := &mockProcessorRecorder{}
			metrics := &mockReconcilerMetrics{}
			logger := zap.NewNop()

			reconciler := NewReconciler(
				harborClient,
				processor,
				ReconcilerConfig{
					Interval: 15 * time.Minute,
					Timeout:  10 * time.Minute,
					Projects: []string{project},
				},
				metrics,
				logger,
			)

			// Run the sweep
			err := reconciler.RunSweep(context.Background())
			if err != nil {
				return false
			}

			// Verify: ProcessArtifact was called exactly for the unsigned artifacts
			processed := processor.getProcessed()

			// Build a set of processed artifact keys
			processedSet := make(map[string]bool)
			for _, ref := range processed {
				key := fmt.Sprintf("%s/%s@%s", ref.Project, ref.Repo, ref.Digest)
				processedSet[key] = true
			}

			// Assert: every unsigned artifact was processed
			for key := range expectedUnsigned {
				if !processedSet[key] {
					return false
				}
			}

			// Assert: no signed artifact was processed
			for key := range signedSet {
				if processedSet[key] {
					return false
				}
			}

			// Assert: total processed count matches unsigned count
			if len(processed) != len(expectedUnsigned) {
				return false
			}

			return true
		},
		genArtifactCount,
		genSignedFlags,
	))

	properties.TestingRun(t)
}
