// Package main is the entry point for the signer-service. It wires all components
// via dependency injection, starts the HTTP servers, and handles graceful shutdown.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/signer-service/internal/config"
	"github.com/signer-service/internal/handler/health"
	"github.com/signer-service/internal/handler/metrics"
	"github.com/signer-service/internal/handler/webhook"
	"github.com/signer-service/internal/pkg/harbor"
	"github.com/signer-service/internal/pkg/logger"
	"github.com/signer-service/internal/pkg/observability"
	"github.com/signer-service/internal/pkg/sigstore"
	"github.com/signer-service/internal/pkg/workerpool"
	"github.com/signer-service/internal/service/gate"
	"github.com/signer-service/internal/service/reconciliation"
	"github.com/signer-service/internal/service/signing"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Load configuration.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// 2. Init structured logger.
	log, err := logger.New(cfg.ServiceName, cfg.ServiceVersion, cfg.Environment, cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("creating logger: %w", err)
	}
	defer log.Sync() //nolint:errcheck

	log.Info("starting signer-service",
		zap.String("version", cfg.ServiceVersion),
		zap.String("environment", cfg.Environment),
	)

	// 3. Init OpenTelemetry tracing.
	ctx := context.Background()
	shutdownTracer, err := observability.InitTraceProvider(ctx, cfg.ServiceName, cfg.ServiceVersion, cfg.OTelEndpoint)
	if err != nil {
		log.Warn("failed to init trace provider, tracing disabled", zap.Error(err))
		shutdownTracer = func(context.Context) error { return nil }
	}

	// 4. Init Prometheus metrics.
	m := observability.NewMetrics(nil)

	// 5. Create Harbor client.
	harborClient := harbor.NewClient(harbor.ClientConfig{
		BaseURL:        cfg.HarborURL,
		Username:       cfg.HarborRobot,
		Password:       cfg.HarborPassword,
		MaxRetries:     cfg.MaxRetries,
		RetryBaseDelay: cfg.RetryBaseDelay,
		RetryMaxDelay:  cfg.RetryMaxDelay,
	})

	// 6. Create gate evaluator.
	threshold, err := gate.ParseSeverity(cfg.SeverityThreshold)
	if err != nil {
		return fmt.Errorf("parsing severity threshold: %w", err)
	}
	evaluator := gate.NewEvaluator(threshold)

	// 7. Create Sigstore signer with concrete Fulcio, Rekor, and Cosign clients.
	signer := sigstore.NewDefaultSigner(
		sigstore.WithFulcioClient(sigstore.NewHTTPFulcioClient(nil)),
		sigstore.WithRekorClient(sigstore.NewHTTPRekorClient(nil)),
		sigstore.WithCosignSigner(sigstore.NewDefaultCosignSigner()),
		sigstore.WithTokenProvider(&sigstore.FileTokenProvider{}),
		sigstore.WithMetricsRecorder(m),
		sigstore.WithRetryConfig(sigstore.RetryConfig{
			MaxAttempts: cfg.MaxRetries,
			BaseDelay:   cfg.RetryBaseDelay,
			MaxDelay:    cfg.RetryMaxDelay,
		}),
	)

	// 8. Create signing service.
	signingService := signing.NewSigningService(
		harborClient,
		signer,
		evaluator,
		log,
		m,
		signing.SigningConfig{
			HarborURL:         cfg.HarborURL,
			FulcioURL:         cfg.FulcioURL,
			RekorURL:          cfg.RekorURL,
			TokenPath:         cfg.TokenPath,
			RegistryURL:       cfg.RegistryURL,
			SeverityThreshold: cfg.SeverityThreshold,
		},
	)

	// 9. Create worker pool.
	pool := workerpool.New(cfg.WorkerCount, cfg.QueueSize, log)

	// 10. Create job submitter (bridges webhook.JobSubmitter to worker pool + signing service).
	submitter := &jobSubmitter{
		pool:           pool,
		signingService: signingService,
		logger:         log,
	}

	// 11. Create health handler with HTTP checkers.
	healthCheckers := []health.DependencyChecker{
		health.NewHTTPChecker("harbor", cfg.HarborURL+"/api/v2.0/health", true),
		health.NewHTTPChecker("fulcio", cfg.FulcioURL+"/api/v1/rootCert", true),
		health.NewHTTPChecker("rekor", cfg.RekorURL+"/api/v1/log", true),
	}
	healthHandler := health.New(healthCheckers)

	// 12. Create webhook handler.
	webhookHandler := webhook.New(cfg, log, submitter)

	// 13. Set up Gin router.
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	root := router.Group("")
	healthHandler.RegisterRoutes(root)
	webhookHandler.RegisterRoutes(root)

	// 14. Start metrics server on port 9090.
	metricsServer := metrics.NewServer(cfg.MetricsPort)
	go func() {
		log.Info("metrics server starting", zap.Int("port", cfg.MetricsPort))
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("metrics server error", zap.Error(err))
		}
	}()

	// 15. Start reconciliation ticker.
	reconciler := reconciliation.NewReconciler(
		harborClient,
		signingService,
		reconciliation.ReconcilerConfig{
			Interval: cfg.ReconcileInterval,
			Timeout:  cfg.ReconcileTimeout,
			Projects: cfg.HarborProjects,
		},
		m,
		log,
	)
	reconciler.Start()

	// 16. Start main HTTP server on port 8080.
	mainServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.ServerPort),
		Handler: router,
	}
	go func() {
		log.Info("HTTP server starting", zap.Int("port", cfg.ServerPort))
		if err := mainServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server error", zap.Error(err))
		}
	}()

	// 17. Wait for SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	sig := <-quit

	log.Info("shutdown signal received", zap.String("signal", sig.String()))

	// --- Graceful Shutdown ---

	// Step 1: Mark readiness as 503 (stop receiving traffic from load balancer).
	healthHandler.SetShutdown()

	// Step 2: Shutdown main HTTP server (stop accepting new requests, drain active).
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := mainServer.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server shutdown error", zap.Error(err))
	}

	// Step 3: Drain worker pool with 30s grace period.
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer drainCancel()
	if err := pool.Shutdown(drainCtx); err != nil {
		log.Error("worker pool shutdown exceeded deadline, abandoning remaining jobs", zap.Error(err))
	}

	// Step 4: Stop reconciliation ticker.
	reconciler.Stop()

	// Step 5: Shutdown metrics server.
	metricsCtx, metricsCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer metricsCancel()
	if err := metricsServer.Shutdown(metricsCtx); err != nil {
		log.Error("metrics server shutdown error", zap.Error(err))
	}

	// Step 6: Shutdown OTel provider.
	otelCtx, otelCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer otelCancel()
	if err := shutdownTracer(otelCtx); err != nil {
		log.Error("OTel provider shutdown error", zap.Error(err))
	}

	log.Info("signer-service shutdown complete")
	return nil
}

// signingJob implements workerpool.Job and delegates execution to the signing service.
type signingJob struct {
	artifact       gate.ArtifactRef
	source         string
	signingService *signing.SigningService
	logger         *zap.Logger
}

// Execute runs the signing pipeline for the artifact.
func (j *signingJob) Execute(ctx context.Context) {
	if err := j.signingService.ProcessArtifact(ctx, j.artifact, j.source); err != nil {
		j.logger.Error("signing job failed",
			zap.String("project", j.artifact.Project),
			zap.String("repo", j.artifact.Repo),
			zap.String("digest", j.artifact.Digest),
			zap.String("source", j.source),
			zap.Error(err),
		)
	}
}

// jobSubmitter implements webhook.JobSubmitter by wrapping the worker pool and signing service.
type jobSubmitter struct {
	pool           *workerpool.WorkerPool
	signingService *signing.SigningService
	logger         *zap.Logger
}

// Submit creates a signingJob and submits it to the worker pool.
func (s *jobSubmitter) Submit(artifact gate.ArtifactRef, source string) {
	s.pool.Submit(&signingJob{
		artifact:       artifact,
		source:         source,
		signingService: s.signingService,
		logger:         s.logger,
	})
}
