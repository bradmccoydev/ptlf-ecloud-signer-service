// Package reconciliation implements a periodic sweep that identifies images with
// passing scans but missing signatures and signs them, ensuring eventual consistency.
package reconciliation

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/signer-service/internal/pkg/harbor"
	"github.com/signer-service/internal/service/gate"
)

// ReconcilerConfig holds configuration for the reconciliation sweep.
type ReconcilerConfig struct {
	// Interval is the period between reconciliation sweeps (default 15m).
	Interval time.Duration
	// Timeout is the maximum duration for a single sweep (default 10m).
	Timeout time.Duration
	// Projects is the list of Harbor projects to scan.
	Projects []string
}

// ReconcilerMetrics abstracts metric operations for the reconciler.
type ReconcilerMetrics interface {
	IncrementReconciliationSigned()
	IncrementReconciliationErrors()
}

// ArtifactProcessor defines the interface for processing (signing) artifacts.
type ArtifactProcessor interface {
	ProcessArtifact(ctx context.Context, ref gate.ArtifactRef, source string) error
}

// Reconciler performs periodic sweeps across Harbor projects to find and sign
// artifacts that have a passing scan but no cosign signature.
type Reconciler struct {
	harborClient harbor.Client
	processor    ArtifactProcessor
	config       ReconcilerConfig
	metrics      ReconcilerMetrics
	logger       *zap.Logger
	running      sync.Mutex
	stopCh       chan struct{}
	done         chan struct{}
}

// NewReconciler creates a new Reconciler with the given dependencies.
func NewReconciler(
	harborClient harbor.Client,
	processor ArtifactProcessor,
	config ReconcilerConfig,
	metrics ReconcilerMetrics,
	logger *zap.Logger,
) *Reconciler {
	return &Reconciler{
		harborClient: harborClient,
		processor:    processor,
		config:       config,
		metrics:      metrics,
		logger:       logger,
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
	}
}

// Start begins the reconciliation ticker goroutine. It returns immediately.
func (r *Reconciler) Start() {
	go r.run()
}

// Stop signals the reconciliation ticker to stop and waits for it to finish.
func (r *Reconciler) Stop() {
	close(r.stopCh)
	<-r.done
}

// run is the internal ticker loop.
func (r *Reconciler) run() {
	defer close(r.done)

	ticker := time.NewTicker(r.config.Interval)
	defer ticker.Stop()

	r.logger.Info("reconciliation ticker started",
		zap.Duration("interval", r.config.Interval),
		zap.Duration("timeout", r.config.Timeout),
		zap.Strings("projects", r.config.Projects),
	)

	for {
		select {
		case <-r.stopCh:
			r.logger.Info("reconciliation ticker stopped")
			return
		case <-ticker.C:
			if err := r.RunSweep(context.Background()); err != nil {
				r.logger.Error("reconciliation sweep failed", zap.Error(err))
			}
		}
	}
}

// RunSweep executes a single reconciliation sweep. It is safe to call directly
// for testing purposes. If a previous sweep is still running, it skips execution.
func (r *Reconciler) RunSweep(ctx context.Context) error {
	// Try to acquire the mutex. If the previous sweep is still running, skip.
	if !r.running.TryLock() {
		r.logger.Warn("reconciliation sweep skipped: previous sweep still in progress")
		return nil
	}
	defer r.running.Unlock()

	// Create a context with the configured timeout.
	sweepCtx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()

	start := time.Now()
	var totalScanned int
	var totalSigned int

	r.logger.Info("reconciliation sweep started")

	// Iterate over each configured project.
	for _, project := range r.config.Projects {
		// Check if context has been cancelled (timeout or stop).
		if sweepCtx.Err() != nil {
			r.logger.Warn("reconciliation sweep timed out",
				zap.Duration("elapsed", time.Since(start)),
				zap.Int("artifacts_scanned", totalScanned),
				zap.Int("artifacts_signed", totalSigned),
			)
			return nil
		}

		artifacts, err := r.harborClient.ListArtifacts(sweepCtx, project)
		if err != nil {
			r.logger.Error("reconciliation sweep aborted: Harbor API error",
				zap.String("project", project),
				zap.Error(err),
			)
			r.metrics.IncrementReconciliationErrors()
			return err
		}

		for _, artifact := range artifacts {
			// Check context before processing each artifact.
			if sweepCtx.Err() != nil {
				r.logger.Warn("reconciliation sweep timed out",
					zap.Duration("elapsed", time.Since(start)),
					zap.Int("artifacts_scanned", totalScanned),
					zap.Int("artifacts_signed", totalSigned),
				)
				return nil
			}

			totalScanned++

			// Check if artifact already has a signature.
			hasSig, err := r.harborClient.HasSignature(sweepCtx, project, artifact.Repo, artifact.Digest)
			if err != nil {
				// Log but continue — per design, proceed as if unsigned on error.
				r.logger.Warn("failed to check signature during reconciliation, skipping artifact",
					zap.String("project", project),
					zap.String("repo", artifact.Repo),
					zap.String("digest", artifact.Digest),
					zap.Error(err),
				)
				continue
			}

			if hasSig {
				// Already signed, skip.
				continue
			}

			// Sign the artifact using the same pipeline as webhook-triggered signing.
			ref := gate.ArtifactRef{
				Project: project,
				Repo:    artifact.Repo,
				Digest:  artifact.Digest,
			}

			if err := r.processor.ProcessArtifact(sweepCtx, ref, "reconciliation"); err != nil {
				r.logger.Warn("reconciliation signing failed for artifact",
					zap.String("project", project),
					zap.String("repo", artifact.Repo),
					zap.String("digest", artifact.Digest),
					zap.Error(err),
				)
				continue
			}

			totalSigned++
			r.metrics.IncrementReconciliationSigned()
		}
	}

	duration := time.Since(start)

	// Check if the context timed out during the final iteration.
	if sweepCtx.Err() != nil {
		r.logger.Warn("reconciliation sweep timed out",
			zap.Duration("elapsed", duration),
			zap.Int("artifacts_scanned", totalScanned),
			zap.Int("artifacts_signed", totalSigned),
		)
		return nil
	}

	// Log sweep summary.
	r.logger.Info("reconciliation sweep completed",
		zap.Int("artifacts_scanned", totalScanned),
		zap.Int("artifacts_signed", totalSigned),
		zap.Float64("duration_seconds", duration.Seconds()),
	)

	return nil
}
