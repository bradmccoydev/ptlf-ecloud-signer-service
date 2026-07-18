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
	// SignaturesIssued counts successful signing operations, labelled by source (webhook/reconciliation).
	SignaturesIssued *prometheus.CounterVec

	// GateFailures counts gate failure decisions, labelled by reason
	// (vulnerability_exceeded, scan_fetch_error, signing_error, fulcio_error, rekor_error).
	GateFailures *prometheus.CounterVec

	// WebhookLatency measures elapsed time from webhook receipt to final outcome.
	WebhookLatency prometheus.Histogram

	// FulcioErrors counts Fulcio request failures after all retry attempts are exhausted.
	FulcioErrors prometheus.Counter

	// RekorErrors counts Rekor request failures after all retry attempts are exhausted.
	RekorErrors prometheus.Counter

	// ReconciliationSigned counts images successfully signed during reconciliation sweeps.
	ReconciliationSigned prometheus.Counter

	// ReconciliationErrors counts reconciliation sweep errors (e.g., Harbor API failures).
	ReconciliationErrors prometheus.Counter
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
			[]string{"source"},
		),

		GateFailures: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gate_failures_total",
				Help: "Total number of gate failure decisions by reason.",
			},
			[]string{"reason"},
		),

		WebhookLatency: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "webhook_latency_seconds",
				Help:    "Elapsed time from webhook HTTP request receipt to final outcome.",
				Buckets: prometheus.DefBuckets,
			},
		),

		FulcioErrors: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "fulcio_errors_total",
				Help: "Total number of Fulcio request failures after retry exhaustion.",
			},
		),

		RekorErrors: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "rekor_errors_total",
				Help: "Total number of Rekor request failures after retry exhaustion.",
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
	}

	reg.MustRegister(
		m.SignaturesIssued,
		m.GateFailures,
		m.WebhookLatency,
		m.FulcioErrors,
		m.RekorErrors,
		m.ReconciliationSigned,
		m.ReconciliationErrors,
	)

	return m
}

// --- signing.SigningMetrics interface ---

// IncrementSignaturesIssued increments the signatures_issued_total counter
// with the given source label (webhook or reconciliation).
func (m *Metrics) IncrementSignaturesIssued(source string) {
	m.SignaturesIssued.WithLabelValues(source).Inc()
}

// IncrementGateFailures increments the gate_failures_total counter
// with the given reason label.
func (m *Metrics) IncrementGateFailures(reason string) {
	m.GateFailures.WithLabelValues(reason).Inc()
}

// ObserveWebhookLatency records the elapsed duration in the webhook_latency_seconds histogram.
func (m *Metrics) ObserveWebhookLatency(duration time.Duration) {
	m.WebhookLatency.Observe(duration.Seconds())
}

// --- sigstore.MetricsRecorder interface ---

// IncrementFulcioErrors increments the fulcio_errors_total counter.
func (m *Metrics) IncrementFulcioErrors() {
	m.FulcioErrors.Inc()
}

// IncrementRekorErrors increments the rekor_errors_total counter.
func (m *Metrics) IncrementRekorErrors() {
	m.RekorErrors.Inc()
}

// IncrementSigningFailures is a no-op placeholder; actual signing_failures_total
// can be added as a separate metric if needed. The sigstore package increments
// this on exhausted retries; for now it maps to a gate failure with signing_error reason.
func (m *Metrics) IncrementSigningFailures() {
	// Signing failures are tracked via gate_failures_total with reason=signing_error.
	// This method satisfies the MetricsRecorder interface from the sigstore package.
	m.GateFailures.WithLabelValues("signing_error").Inc()
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
