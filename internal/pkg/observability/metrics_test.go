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
	if m.SigningDuration == nil {
		t.Error("SigningDuration histogram vec is nil")
	}
	if m.FulcioErrors == nil {
		t.Error("FulcioErrors counter is nil")
	}
	if m.FulcioLatency == nil {
		t.Error("FulcioLatency histogram is nil")
	}
	if m.RekorErrors == nil {
		t.Error("RekorErrors counter is nil")
	}
	if m.RekorLatency == nil {
		t.Error("RekorLatency histogram is nil")
	}
	if m.ReconciliationSigned == nil {
		t.Error("ReconciliationSigned counter is nil")
	}
	if m.ReconciliationErrors == nil {
		t.Error("ReconciliationErrors counter is nil")
	}
	if m.ReconciliationDuration == nil {
		t.Error("ReconciliationDuration histogram is nil")
	}
	if m.ArtifactsSkippedAlreadySigned == nil {
		t.Error("ArtifactsSkippedAlreadySigned counter vec is nil")
	}
	if m.InFlightSigningOps == nil {
		t.Error("InFlightSigningOps gauge is nil")
	}
}

func TestMetrics_IncrementSignaturesIssued(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg)

	m.IncrementSignaturesIssued("webhook", "platform/myapp")
	m.IncrementSignaturesIssued("reconciliation", "platform/myapp")
	m.IncrementSignaturesIssued("webhook", "platform/other")

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
			if len(metrics) != 3 {
				t.Fatalf("expected 3 label combinations, got %d", len(metrics))
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

	m.IncrementGateFailures("vulnerability_exceeded", "platform/myapp")
	m.IncrementGateFailures("scan_fetch_error", "platform/other")

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

func TestMetrics_SigningDuration(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg)

	m.ObserveSigningDuration("platform/myapp", 3*time.Second)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	found := false
	for _, f := range families {
		if f.GetName() == "signing_duration_seconds" {
			found = true
		}
	}
	if !found {
		t.Error("signing_duration_seconds metric not found in registry")
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

func TestMetrics_FulcioAndRekorLatency(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg)

	m.ObserveFulcioLatency(500 * time.Millisecond)
	m.ObserveRekorLatency(250 * time.Millisecond)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	foundFulcio := false
	foundRekor := false
	for _, f := range families {
		switch f.GetName() {
		case "fulcio_request_duration_seconds":
			foundFulcio = true
		case "rekor_upload_duration_seconds":
			foundRekor = true
		}
	}
	if !foundFulcio {
		t.Error("fulcio_request_duration_seconds metric not found in registry")
	}
	if !foundRekor {
		t.Error("rekor_upload_duration_seconds metric not found in registry")
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

func TestMetrics_ReconciliationDuration(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg)

	m.ObserveReconciliationDuration(45 * time.Second)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	found := false
	for _, f := range families {
		if f.GetName() == "reconciliation_sweep_duration_seconds" {
			found = true
		}
	}
	if !found {
		t.Error("reconciliation_sweep_duration_seconds metric not found in registry")
	}
}

func TestMetrics_ArtifactsSkipped(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg)

	m.IncrementArtifactsSkipped("platform/myapp")

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	found := false
	for _, f := range families {
		if f.GetName() == "artifacts_skipped_already_signed_total" {
			found = true
		}
	}
	if !found {
		t.Error("artifacts_skipped_already_signed_total metric not found in registry")
	}
}

func TestMetrics_InFlightSigningOps(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg)

	m.IncrementInFlightSigningOps()
	m.IncrementInFlightSigningOps()
	m.DecrementInFlightSigningOps()

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	found := false
	for _, f := range families {
		if f.GetName() == "inflight_signing_operations" {
			found = true
			metrics := f.GetMetric()
			if len(metrics) == 0 {
				t.Fatal("expected at least 1 metric, got 0")
			}
			val := metrics[0].GetGauge().GetValue()
			if val != 1 {
				t.Errorf("expected gauge value 1 (2 increments - 1 decrement), got %v", val)
			}
		}
	}
	if !found {
		t.Error("inflight_signing_operations metric not found in registry")
	}
}
