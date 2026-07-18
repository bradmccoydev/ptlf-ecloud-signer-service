package observability_test

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/signer-service/internal/pkg/observability"
	"github.com/signer-service/internal/pkg/sigstore"
	"github.com/signer-service/internal/service/reconciliation"
	"github.com/signer-service/internal/service/signing"
)

// Compile-time interface satisfaction checks.
var _ signing.SigningMetrics = (*observability.Metrics)(nil)
var _ sigstore.MetricsRecorder = (*observability.Metrics)(nil)
var _ reconciliation.ReconcilerMetrics = (*observability.Metrics)(nil)

func TestNewMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg)

	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}
	if m.SignaturesIssued == nil {
		t.Error("SignaturesIssued counter vec is nil")
	}
	if m.GateFailures == nil {
		t.Error("GateFailures counter vec is nil")
	}
	if m.WebhookLatency == nil {
		t.Error("WebhookLatency histogram is nil")
	}
	if m.FulcioErrors == nil {
		t.Error("FulcioErrors counter is nil")
	}
	if m.RekorErrors == nil {
		t.Error("RekorErrors counter is nil")
	}
	if m.ReconciliationSigned == nil {
		t.Error("ReconciliationSigned counter is nil")
	}
	if m.ReconciliationErrors == nil {
		t.Error("ReconciliationErrors counter is nil")
	}
}

func TestMetrics_IncrementSignaturesIssued(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg)

	m.IncrementSignaturesIssued("webhook")
	m.IncrementSignaturesIssued("reconciliation")
	m.IncrementSignaturesIssued("webhook")

	// Gather and check.
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	found := false
	for _, f := range families {
		if f.GetName() == "signatures_issued_total" {
			found = true
			metrics := f.GetMetric()
			if len(metrics) != 2 {
				t.Fatalf("expected 2 label combinations, got %d", len(metrics))
			}
		}
	}
	if !found {
		t.Error("signatures_issued_total metric not found in registry")
	}
}

func TestMetrics_IncrementGateFailures(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg)

	m.IncrementGateFailures("vulnerability_exceeded")
	m.IncrementGateFailures("scan_fetch_error")

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	found := false
	for _, f := range families {
		if f.GetName() == "gate_failures_total" {
			found = true
			metrics := f.GetMetric()
			if len(metrics) < 2 {
				t.Fatalf("expected at least 2 label combinations, got %d", len(metrics))
			}
		}
	}
	if !found {
		t.Error("gate_failures_total metric not found in registry")
	}
}

func TestMetrics_ObserveWebhookLatency(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg)

	m.ObserveWebhookLatency(150 * time.Millisecond)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	found := false
	for _, f := range families {
		if f.GetName() == "webhook_latency_seconds" {
			found = true
			metrics := f.GetMetric()
			if len(metrics) == 0 {
				t.Fatal("expected at least 1 metric, got 0")
			}
			h := metrics[0].GetHistogram()
			if h == nil {
				t.Fatal("expected histogram metric")
			}
			if h.GetSampleCount() != 1 {
				t.Errorf("expected sample count 1, got %d", h.GetSampleCount())
			}
		}
	}
	if !found {
		t.Error("webhook_latency_seconds metric not found in registry")
	}
}

func TestMetrics_FulcioAndRekorErrors(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg)

	m.IncrementFulcioErrors()
	m.IncrementRekorErrors()

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	foundFulcio := false
	foundRekor := false
	for _, f := range families {
		switch f.GetName() {
		case "fulcio_errors_total":
			foundFulcio = true
		case "rekor_errors_total":
			foundRekor = true
		}
	}
	if !foundFulcio {
		t.Error("fulcio_errors_total metric not found in registry")
	}
	if !foundRekor {
		t.Error("rekor_errors_total metric not found in registry")
	}
}

func TestMetrics_ReconciliationCounters(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg)

	m.IncrementReconciliationSigned()
	m.IncrementReconciliationErrors()

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	foundSigned := false
	foundErrors := false
	for _, f := range families {
		switch f.GetName() {
		case "reconciliation_signed_total":
			foundSigned = true
		case "reconciliation_errors_total":
			foundErrors = true
		}
	}
	if !foundSigned {
		t.Error("reconciliation_signed_total metric not found in registry")
	}
	if !foundErrors {
		t.Error("reconciliation_errors_total metric not found in registry")
	}
}
