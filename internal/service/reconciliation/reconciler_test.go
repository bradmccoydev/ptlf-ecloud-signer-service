package reconciliation

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/signer-service/internal/pkg/harbor"
	"github.com/signer-service/internal/service/gate"
)

// --- Mock: Harbor Client ---

type mockHarborClient struct {
	listArtifactsFn func(ctx context.Context, project string) ([]harbor.Artifact, error)
	hasSignatureFn  func(ctx context.Context, project, repo, digest string) (bool, error)
	getScanReportFn func(ctx context.Context, project, repo, digest string) (*harbor.ScanReport, error)
}

func (m *mockHarborClient) ListArtifacts(ctx context.Context, project string) ([]harbor.Artifact, error) {
	if m.listArtifactsFn != nil {
		return m.listArtifactsFn(ctx, project)
	}
	return nil, nil
}

func (m *mockHarborClient) HasSignature(ctx context.Context, project, repo, digest string) (bool, error) {
	if m.hasSignatureFn != nil {
		return m.hasSignatureFn(ctx, project, repo, digest)
	}
	return false, nil
}

func (m *mockHarborClient) GetScanReport(ctx context.Context, project, repo, digest string) (*harbor.ScanReport, error) {
	if m.getScanReportFn != nil {
		return m.getScanReportFn(ctx, project, repo, digest)
	}
	return &harbor.ScanReport{}, nil
}

// --- Mock: ArtifactProcessor ---

type mockProcessor struct {
	mu    sync.Mutex
	calls []gate.ArtifactRef
	err   error
}

func (m *mockProcessor) ProcessArtifact(_ context.Context, ref gate.ArtifactRef, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, ref)
	return m.err
}

func (m *mockProcessor) getCalls() []gate.ArtifactRef {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]gate.ArtifactRef, len(m.calls))
	copy(result, m.calls)
	return result
}

// --- Mock: Metrics ---

type mockMetrics struct {
	reconciliationSigned atomic.Int32
	reconciliationErrors atomic.Int32
}

func (m *mockMetrics) IncrementReconciliationSigned() {
	m.reconciliationSigned.Add(1)
}

func (m *mockMetrics) IncrementReconciliationErrors() {
	m.reconciliationErrors.Add(1)
}

// --- Tests ---

// TestRunSweep_SkipWhenPreviousSweepInProgress verifies that if a previous sweep
// is still in progress (mutex locked), RunSweep returns immediately without
// calling ListArtifacts.
// Validates: Requirements 6.1
func TestRunSweep_SkipWhenPreviousSweepInProgress(t *testing.T) {
	listArtifactsCalled := atomic.Int32{}

	harborClient := &mockHarborClient{
		listArtifactsFn: func(_ context.Context, _ string) ([]harbor.Artifact, error) {
			listArtifactsCalled.Add(1)
			return nil, nil
		},
	}

	processor := &mockProcessor{}
	metrics := &mockMetrics{}

	core, logs := observer.New(zap.WarnLevel)
	logger := zap.New(core)

	r := NewReconciler(harborClient, processor, ReconcilerConfig{
		Interval: 15 * time.Minute,
		Timeout:  10 * time.Minute,
		Projects: []string{"platform"},
	}, metrics, logger)

	// Lock the running mutex to simulate a sweep in progress.
	r.running.Lock()

	// Call RunSweep — should return immediately.
	err := r.RunSweep(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Unlock so the test can clean up.
	r.running.Unlock()

	// Verify ListArtifacts was never called.
	if listArtifactsCalled.Load() != 0 {
		t.Errorf("expected ListArtifacts not to be called, but it was called %d times", listArtifactsCalled.Load())
	}

	// Verify the skip warning was logged.
	found := false
	for _, entry := range logs.All() {
		if entry.Message == "reconciliation sweep skipped: previous sweep still in progress" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected skip warning log message not found")
	}
}

// TestRunSweep_TimeoutCancellation verifies that when the configured Timeout is
// very short and the Harbor client is slow, the sweep aborts gracefully.
// Validates: Requirements 6.6
func TestRunSweep_TimeoutCancellation(t *testing.T) {
	harborClient := &mockHarborClient{
		listArtifactsFn: func(ctx context.Context, _ string) ([]harbor.Artifact, error) {
			// Simulate a slow API call — wait longer than the timeout.
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Second):
				return []harbor.Artifact{{Digest: "sha256:abc", Repo: "myrepo"}}, nil
			}
		},
	}

	processor := &mockProcessor{}
	metrics := &mockMetrics{}
	logger := zap.NewNop()

	r := NewReconciler(harborClient, processor, ReconcilerConfig{
		Interval: 15 * time.Minute,
		Timeout:  50 * time.Millisecond, // Very short timeout to trigger cancellation.
		Projects: []string{"platform"},
	}, metrics, logger)

	start := time.Now()
	err := r.RunSweep(context.Background())
	elapsed := time.Since(start)

	// The sweep should have returned an error (context deadline exceeded from ListArtifacts)
	// OR returned nil if the context check happens before the call.
	// Given the implementation, ListArtifacts is called with the sweep context,
	// so it will return ctx.Err() which bubbles up as an error.
	// The reconciler returns the error and increments reconciliation_errors_total.
	if err != nil {
		// Verify it completed quickly (well under the 5s sleep).
		if elapsed > 2*time.Second {
			t.Errorf("sweep took too long (%v), expected quick timeout", elapsed)
		}
	} else {
		// If nil, the context check at the top of the loop caught it.
		if elapsed > 2*time.Second {
			t.Errorf("sweep took too long (%v), expected quick timeout", elapsed)
		}
	}

	// Verify no processor calls were made.
	if len(processor.getCalls()) != 0 {
		t.Errorf("expected no ProcessArtifact calls, got %d", len(processor.getCalls()))
	}
}

// TestRunSweep_SuccessfulSweepSummary verifies that when 3 artifacts are returned
// (2 unsigned, 1 signed), exactly 2 ProcessArtifact calls are made and
// reconciliation_signed_total is incremented twice.
// Validates: Requirements 6.5, 6.7
func TestRunSweep_SuccessfulSweepSummary(t *testing.T) {
	artifacts := []harbor.Artifact{
		{Digest: "sha256:aaa", Repo: "repo-a"},
		{Digest: "sha256:bbb", Repo: "repo-b"},
		{Digest: "sha256:ccc", Repo: "repo-c"},
	}

	harborClient := &mockHarborClient{
		listArtifactsFn: func(_ context.Context, _ string) ([]harbor.Artifact, error) {
			return artifacts, nil
		},
		hasSignatureFn: func(_ context.Context, _, _, digest string) (bool, error) {
			// sha256:bbb already has a signature.
			return digest == "sha256:bbb", nil
		},
	}

	processor := &mockProcessor{}
	metrics := &mockMetrics{}

	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	r := NewReconciler(harborClient, processor, ReconcilerConfig{
		Interval: 15 * time.Minute,
		Timeout:  10 * time.Minute,
		Projects: []string{"platform"},
	}, metrics, logger)

	err := r.RunSweep(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify exactly 2 ProcessArtifact calls (for sha256:aaa and sha256:ccc).
	calls := processor.getCalls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 ProcessArtifact calls, got %d", len(calls))
	}

	// Verify the correct artifacts were processed.
	digests := map[string]bool{}
	for _, c := range calls {
		digests[c.Digest] = true
	}
	if !digests["sha256:aaa"] {
		t.Error("expected sha256:aaa to be processed")
	}
	if !digests["sha256:ccc"] {
		t.Error("expected sha256:ccc to be processed")
	}
	if digests["sha256:bbb"] {
		t.Error("sha256:bbb should not have been processed (already signed)")
	}

	// Verify reconciliation_signed_total was incremented twice.
	if metrics.reconciliationSigned.Load() != 2 {
		t.Errorf("expected reconciliation_signed_total=2, got %d", metrics.reconciliationSigned.Load())
	}

	// Verify sweep summary log was emitted.
	found := false
	for _, entry := range logs.All() {
		if entry.Message == "reconciliation sweep completed" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'reconciliation sweep completed' log message not found")
	}
}

// TestRunSweep_HarborAPIFailure verifies that when ListArtifacts returns an error,
// reconciliation_errors_total is incremented and the error is returned.
// Validates: Requirements 6.5
func TestRunSweep_HarborAPIFailure(t *testing.T) {
	harborClient := &mockHarborClient{
		listArtifactsFn: func(_ context.Context, _ string) ([]harbor.Artifact, error) {
			return nil, errors.New("harbor API unreachable")
		},
	}

	processor := &mockProcessor{}
	metrics := &mockMetrics{}

	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)

	r := NewReconciler(harborClient, processor, ReconcilerConfig{
		Interval: 15 * time.Minute,
		Timeout:  10 * time.Minute,
		Projects: []string{"platform"},
	}, metrics, logger)

	err := r.RunSweep(context.Background())

	// Verify error was returned.
	if err == nil {
		t.Fatal("expected error from RunSweep, got nil")
	}

	// Verify reconciliation_errors_total was incremented.
	if metrics.reconciliationErrors.Load() != 1 {
		t.Errorf("expected reconciliation_errors_total=1, got %d", metrics.reconciliationErrors.Load())
	}

	// Verify no ProcessArtifact calls were made.
	if len(processor.getCalls()) != 0 {
		t.Errorf("expected no ProcessArtifact calls, got %d", len(processor.getCalls()))
	}

	// Verify error log was emitted.
	found := false
	for _, entry := range logs.All() {
		if entry.Message == "reconciliation sweep aborted: Harbor API error" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'reconciliation sweep aborted: Harbor API error' log message not found")
	}
}
