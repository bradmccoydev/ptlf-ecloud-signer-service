// Package observability provides Prometheus metric definitions and OpenTelemetry
// tracing initialisation for the signer service.
package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics defines all Prometheus metrics exposed by the signer service.
// It implements the signing.SigningMetrics, sigstore.MetricsRecorder,
// and reconciliation.ReconcilerMetrics interfaces.
type Metrics struct {
	// SignaturesIssued counts successful signing operations, labelled by source and image.
	SignaturesIssued *prometheus.CounterVec

	// GateFailures counts gate failure decisions, labelled by reason and image.
	GateFailures *prometheus.CounterVec

	// WebhookLatency measures elapsed time from webhook receipt to final outcome.
	WebhookLatency prometheus.Histogram

	// SigningDuration measures the time taken for the actual signing operation (Fulcio + cosign + Rekor),
	// labelled by image.
	SigningDuration *prometheus.HistogramVec

	// FulcioErrors counts Fulcio request failures after all retry attempts are exhausted.
	FulcioErrors prometheus.Counter

	// FulcioLatency measures round-trip time for Fulcio certificate requests.
	FulcioLatency prometheus.Histogram

	// RekorErrors counts Rekor request failures after all retry attempts are exhausted.
	RekorErrors prometheus.Counter

	// RekorLatency measures round-trip time for Rekor transparency log uploads.
	RekorLatency prometheus.Histogram

	// ReconciliationSigned counts images successfully signed during reconciliation sweeps.
	ReconciliationSigned prometheus.Counter

	// ReconciliationErrors counts reconciliation sweep errors (e.g., Harbor API failures).
	ReconciliationErrors prometheus.Counter

	// ReconciliationDuration measures the total duration of each reconciliation sweep.
	ReconciliationDuration prometheus.Histogram

	// ArtifactsSkippedAlreadySigned counts artifacts skipped because they already have a signature,
	// labelled by image.
	ArtifactsSkippedAlreadySigned *prometheus.CounterVec

	// InFlightSigningOps tracks the number of currently in-progress signing operations.
	InFlightSigningOps prometheus.Gauge
}

// NewMetrics creates and registers all Prometheus metrics with the provided registerer.
// If reg is nil, prometheus.DefaultRegisterer is used.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}

	m := &Metrics{
		SignaturesIssued: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "signatures_issued_total",
				Help: "Total number of successful image signing operations.",
			},
			[]string{"source", "image"},
		),

		GateFailures: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gate_failures_total",
				Help: "Total number of gate failure decisions by reason.",
			},
			[]string{"reason", "image"},
		),

		WebhookLatency: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "webhook_latency_seconds",
				Help:    "Elapsed time from webhook HTTP request receipt to final outcome.",
				Buckets: prometheus.DefBuckets,
			},
		),

		SigningDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "signing_duration_seconds",
				Help:    "Time taken for the signing pipeline (Fulcio cert + cosign + Rekor upload).",
				Buckets: []float64{0.5, 1, 2, 5, 10, 20, 30, 60},
			},
			[]string{"image"},
		),

		FulcioErrors: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "fulcio_errors_total",
				Help: "Total number of Fulcio request failures after retry exhaustion.",
			},
		),

		FulcioLatency: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "fulcio_request_duration_seconds",
				Help:    "Round-trip time for Fulcio certificate requests.",
				Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 5, 10},
			},
		),

		RekorErrors: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "rekor_errors_total",
				Help: "Total number of Rekor request failures after retry exhaustion.",
			},
		),

		RekorLatency: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "rekor_upload_duration_seconds",
				Help:    "Round-trip time for Rekor transparency log uploads.",
				Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 5, 10},
			},
		),

		ReconciliationSigned: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "reconciliation_signed_total",
				Help: "Total number of images signed during reconciliation sweeps.",
			},
		),

		ReconciliationErrors: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "reconciliation_errors_total",
				Help: "Total number of reconciliation sweep errors.",
			},
		),

		ReconciliationDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "reconciliation_sweep_duration_seconds",
				Help:    "Total duration of each reconciliation sweep.",
				Buckets: []float64{1, 5, 15, 30, 60, 120, 300, 600},
			},
		),

		ArtifactsSkippedAlreadySigned: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "artifacts_skipped_already_signed_total",
				Help: "Total number of artifacts skipped because a signature already exists.",
			},
			[]string{"image"},
		),

		InFlightSigningOps: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "inflight_signing_operations",
				Help: "Number of signing operations currently in progress.",
			},
		),
	}

	reg.MustRegister(
		m.SignaturesIssued,
		m.GateFailures,
		m.WebhookLatency,
		m.SigningDuration,
		m.FulcioErrors,
		m.FulcioLatency,
		m.RekorErrors,
		m.RekorLatency,
		m.ReconciliationSigned,
		m.ReconciliationErrors,
		m.ReconciliationDuration,
		m.ArtifactsSkippedAlreadySigned,
		m.InFlightSigningOps,
	)

	return m
}

// --- signing.SigningMetrics interface ---

// IncrementSignaturesIssued increments the signatures_issued_total counter
// with the given source and image labels.
func (m *Metrics) IncrementSignaturesIssued(source, image string) {
	m.SignaturesIssued.WithLabelValues(source, image).Inc()
}

// IncrementGateFailures increments the gate_failures_total counter
// with the given reason and image labels.
func (m *Metrics) IncrementGateFailures(reason, image string) {
	m.GateFailures.WithLabelValues(reason, image).Inc()
}

// ObserveWebhookLatency records the elapsed duration in the webhook_latency_seconds histogram.
func (m *Metrics) ObserveWebhookLatency(duration time.Duration) {
	m.WebhookLatency.Observe(duration.Seconds())
}

// ObserveSigningDuration records the signing pipeline duration for the given image.
func (m *Metrics) ObserveSigningDuration(image string, duration time.Duration) {
	m.SigningDuration.WithLabelValues(image).Observe(duration.Seconds())
}

// IncrementArtifactsSkipped increments the artifacts_skipped_already_signed_total counter.
func (m *Metrics) IncrementArtifactsSkipped(image string) {
	m.ArtifactsSkippedAlreadySigned.WithLabelValues(image).Inc()
}

// IncrementInFlightSigningOps increments the in-flight signing operations gauge.
func (m *Metrics) IncrementInFlightSigningOps() {
	m.InFlightSigningOps.Inc()
}

// DecrementInFlightSigningOps decrements the in-flight signing operations gauge.
func (m *Metrics) DecrementInFlightSigningOps() {
	m.InFlightSigningOps.Dec()
}

// --- sigstore.MetricsRecorder interface ---

// IncrementFulcioErrors increments the fulcio_errors_total counter.
func (m *Metrics) IncrementFulcioErrors() {
	m.FulcioErrors.Inc()
}

// ObserveFulcioLatency records the Fulcio request round-trip time.
func (m *Metrics) ObserveFulcioLatency(duration time.Duration) {
	m.FulcioLatency.Observe(duration.Seconds())
}

// IncrementRekorErrors increments the rekor_errors_total counter.
func (m *Metrics) IncrementRekorErrors() {
	m.RekorErrors.Inc()
}

// ObserveRekorLatency records the Rekor upload round-trip time.
func (m *Metrics) ObserveRekorLatency(duration time.Duration) {
	m.RekorLatency.Observe(duration.Seconds())
}

// IncrementSigningFailures is a no-op placeholder; actual signing_failures_total
// can be added as a separate metric if needed. The sigstore package increments
// this on exhausted retries; for now it maps to a gate failure with signing_error reason.
func (m *Metrics) IncrementSigningFailures() {
	// Signing failures are tracked via gate_failures_total with reason=signing_error.
	// This method satisfies the MetricsRecorder interface from the sigstore package.
	m.GateFailures.WithLabelValues("signing_error", "").Inc()
}

// --- reconciliation.ReconcilerMetrics interface ---

// IncrementReconciliationSigned increments the reconciliation_signed_total counter.
func (m *Metrics) IncrementReconciliationSigned() {
	m.ReconciliationSigned.Inc()
}

// IncrementReconciliationErrors increments the reconciliation_errors_total counter.
func (m *Metrics) IncrementReconciliationErrors() {
	m.ReconciliationErrors.Inc()
}

// ObserveReconciliationDuration records the total duration of a reconciliation sweep.
func (m *Metrics) ObserveReconciliationDuration(duration time.Duration) {
	m.ReconciliationDuration.Observe(duration.Seconds())
}
