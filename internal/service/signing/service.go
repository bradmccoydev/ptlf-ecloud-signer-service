// Package signing orchestrates the full image signing pipeline for an artifact.
// It coordinates idempotency checks, scan report evaluation, gate decisions,
// and the actual signing operation with appropriate metrics and tracing.
package signing

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/signer-service/internal/pkg/harbor"
	"github.com/signer-service/internal/pkg/sigstore"
	"github.com/signer-service/internal/service/gate"
)

// SigningMetrics abstracts metric operations for the signing service.
type SigningMetrics interface {
	IncrementSignaturesIssued(source string)
	IncrementGateFailures(reason string)
	ObserveWebhookLatency(duration time.Duration)
}

// SigningConfig holds configuration for the signing service.
type SigningConfig struct {
	// HarborURL is the base URL of the Harbor registry.
	HarborURL string
	// FulcioURL is the Fulcio CA endpoint.
	FulcioURL string
	// RekorURL is the Rekor transparency log endpoint.
	RekorURL string
	// TokenPath is the path to the projected SA token for Fulcio authentication.
	TokenPath string
	// RegistryURL is the registry hostname for image references (e.g., "registry.platform.cuscal.io").
	RegistryURL string
}

// SigningService orchestrates the full signing pipeline for an artifact.
type SigningService struct {
	harborClient harbor.Client
	signer       sigstore.Signer
	evaluator    gate.Evaluator
	logger       *zap.Logger
	metrics      SigningMetrics
	config       SigningConfig
	tracer       trace.Tracer
}

// NewSigningService creates a new SigningService with all required dependencies.
func NewSigningService(
	harborClient harbor.Client,
	signer sigstore.Signer,
	evaluator gate.Evaluator,
	logger *zap.Logger,
	metrics SigningMetrics,
	config SigningConfig,
) *SigningService {
	return &SigningService{
		harborClient: harborClient,
		signer:       signer,
		evaluator:    evaluator,
		logger:       logger,
		metrics:      metrics,
		config:       config,
		tracer:       otel.Tracer("signing-service"),
	}
}

// ProcessArtifact runs the signing pipeline for a single artifact.
// The source parameter indicates the trigger ("webhook" or "reconciliation")
// and is used as the label value for the signatures_issued_total metric.
//
// Pipeline steps:
// 1. Check if signature already exists (idempotency) — skip if already signed.
// 2. Fetch scan report from Harbor.
// 3. Evaluate report via gate evaluator.
// 4. If fail: emit gate_failures_total, log structured decision, return nil.
// 5. If pass: sign image, increment signatures_issued_total, log success.
func (s *SigningService) ProcessArtifact(ctx context.Context, ref gate.ArtifactRef, source string) error {
	ctx, span := s.tracer.Start(ctx, "ProcessArtifact",
		trace.WithAttributes(
			attribute.String("artifact.project", ref.Project),
			attribute.String("artifact.repo", ref.Repo),
			attribute.String("artifact.digest", ref.Digest),
			attribute.String("source", source),
		),
	)
	defer span.End()

	// Step 1: Idempotency check — skip if already signed.
	alreadySigned, err := s.checkExistingSignature(ctx, ref)
	if err != nil {
		// On error checking signature, proceed as if unsigned (per design: retry exhausted → proceed).
		s.logger.Warn("failed to check existing signature, proceeding with signing",
			zap.String("project", ref.Project),
			zap.String("repo", ref.Repo),
			zap.String("digest", ref.Digest),
			zap.Error(err),
		)
	}
	if alreadySigned {
		s.logger.Info("artifact already signed, skipping",
			zap.String("project", ref.Project),
			zap.String("repo", ref.Repo),
			zap.String("digest", ref.Digest),
			zap.String("source", source),
		)
		span.SetAttributes(attribute.Bool("skipped.already_signed", true))
		return nil
	}

	// Step 2: Fetch scan report from Harbor.
	report, err := s.fetchScanReport(ctx, ref)
	if err != nil {
		s.metrics.IncrementGateFailures("scan_fetch_error")
		s.logger.Error("failed to fetch scan report",
			zap.String("project", ref.Project),
			zap.String("repo", ref.Repo),
			zap.String("digest", ref.Digest),
			zap.String("source", source),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, "scan report fetch failed")
		return fmt.Errorf("failed to fetch scan report for %s/%s@%s: %w", ref.Project, ref.Repo, ref.Digest, err)
	}

	// Step 3: Evaluate report via gate evaluator.
	decision := s.evaluateReport(ctx, report)
	decision.Artifact = ref

	// Step 4: Handle gate failure.
	if !decision.Pass {
		s.metrics.IncrementGateFailures(decision.Reason)
		s.logGateFailure(ref, decision, source)
		span.SetAttributes(
			attribute.Bool("gate.pass", false),
			attribute.String("gate.reason", decision.Reason),
		)
		// Gate failure is expected behaviour — not an error.
		return nil
	}

	// Step 5: Gate passed — sign the artifact.
	imageRef := fmt.Sprintf("%s/%s/%s@%s", s.config.RegistryURL, ref.Project, ref.Repo, ref.Digest)

	result, err := s.signArtifact(ctx, imageRef)
	if err != nil {
		s.metrics.IncrementGateFailures("signing_error")
		s.logger.Error("signing operation failed",
			zap.String("project", ref.Project),
			zap.String("repo", ref.Repo),
			zap.String("digest", ref.Digest),
			zap.String("source", source),
			zap.String("image_ref", imageRef),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, "signing failed")
		return fmt.Errorf("signing failed for %s: %w", imageRef, err)
	}

	// Success: increment metric and log.
	s.metrics.IncrementSignaturesIssued(source)
	s.logger.Info("artifact signed successfully",
		zap.String("project", ref.Project),
		zap.String("repo", ref.Repo),
		zap.String("digest", ref.Digest),
		zap.String("source", source),
		zap.String("rekor_entry_uuid", result.RekorEntryUUID),
		zap.Time("signed_at", result.SignedAt),
		zap.String("image_ref", imageRef),
	)
	span.SetAttributes(
		attribute.Bool("gate.pass", true),
		attribute.String("signing.rekor_entry_uuid", result.RekorEntryUUID),
	)
	span.SetStatus(codes.Ok, "signed successfully")

	return nil
}

// checkExistingSignature checks if the artifact already has a cosign signature.
func (s *SigningService) checkExistingSignature(ctx context.Context, ref gate.ArtifactRef) (bool, error) {
	ctx, span := s.tracer.Start(ctx, "CheckExistingSignature",
		trace.WithAttributes(
			attribute.String("artifact.digest", ref.Digest),
		),
	)
	defer span.End()

	hasSig, err := s.harborClient.HasSignature(ctx, ref.Project, ref.Repo, ref.Digest)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "signature check failed")
		return false, err
	}

	span.SetAttributes(attribute.Bool("has_signature", hasSig))
	return hasSig, nil
}

// fetchScanReport retrieves the vulnerability scan report from Harbor.
func (s *SigningService) fetchScanReport(ctx context.Context, ref gate.ArtifactRef) (*harbor.ScanReport, error) {
	ctx, span := s.tracer.Start(ctx, "FetchScanReport",
		trace.WithAttributes(
			attribute.String("artifact.project", ref.Project),
			attribute.String("artifact.repo", ref.Repo),
			attribute.String("artifact.digest", ref.Digest),
		),
	)
	defer span.End()

	report, err := s.harborClient.GetScanReport(ctx, ref.Project, ref.Repo, ref.Digest)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "scan report fetch failed")
		return nil, err
	}

	span.SetAttributes(attribute.Int("vulnerabilities.count", len(report.Vulnerabilities)))
	return report, nil
}

// evaluateReport runs the gate evaluator against the scan report.
func (s *SigningService) evaluateReport(ctx context.Context, report *harbor.ScanReport) gate.GateDecision {
	_, span := s.tracer.Start(ctx, "EvaluateGate")
	defer span.End()

	decision := s.evaluator.Evaluate(report)

	span.SetAttributes(
		attribute.Bool("gate.pass", decision.Pass),
		attribute.String("gate.reason", decision.Reason),
	)

	return decision
}

// signArtifact performs the actual signing operation via the Sigstore signer.
func (s *SigningService) signArtifact(ctx context.Context, imageRef string) (*sigstore.SigningResult, error) {
	ctx, span := s.tracer.Start(ctx, "SignArtifact",
		trace.WithAttributes(
			attribute.String("image_ref", imageRef),
		),
	)
	defer span.End()

	opts := sigstore.SigningOptions{
		ImageRef:  imageRef,
		TokenPath: s.config.TokenPath,
		FulcioURL: s.config.FulcioURL,
		RekorURL:  s.config.RekorURL,
		Annotations: map[string]string{
			"scan-report": "sha256:pending",
			"trivy-db":    "latest",
			"policy":      "high",
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		},
	}

	result, err := s.signer.Sign(ctx, opts)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "signing operation failed")
		return nil, err
	}

	span.SetAttributes(attribute.String("rekor_entry_uuid", result.RekorEntryUUID))
	span.SetStatus(codes.Ok, "signed")
	return result, nil
}

// logGateFailure logs a structured JSON decision for a gate failure.
func (s *SigningService) logGateFailure(ref gate.ArtifactRef, decision gate.GateDecision, source string) {
	fields := []zap.Field{
		zap.String("project", ref.Project),
		zap.String("repo", ref.Repo),
		zap.String("digest", ref.Digest),
		zap.String("source", source),
		zap.String("reason", decision.Reason),
		zap.Bool("pass", false),
	}

	// Include CVE counts and violations for vulnerability-based failures.
	if decision.Reason == "vulnerability_exceeded" {
		fields = append(fields,
			zap.Any("cve_counts", decision.CVECounts),
			zap.Any("violations", decision.Violations),
		)
	}

	s.logger.Info("gate decision: fail", fields...)
}
