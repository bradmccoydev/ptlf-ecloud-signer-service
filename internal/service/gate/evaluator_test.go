package gate

import (
	"testing"

	"github.com/signer-service/internal/pkg/harbor"
)

func TestEvaluate_NilReport(t *testing.T) {
	eval := NewEvaluator(SeverityHigh)
	decision := eval.Evaluate(nil)

	if decision.Pass {
		t.Fatal("expected fail for nil report")
	}
	if decision.Reason != "scan_fetch_error" {
		t.Errorf("reason = %q, want %q", decision.Reason, "scan_fetch_error")
	}
}

func TestEvaluate_EmptyVulnerabilities(t *testing.T) {
	eval := NewEvaluator(SeverityHigh)
	report := &harbor.ScanReport{
		Vulnerabilities: []harbor.Vulnerability{},
	}
	decision := eval.Evaluate(report)

	if !decision.Pass {
		t.Fatal("expected pass for empty vulnerability list")
	}
	if decision.Reason != "" {
		t.Errorf("reason = %q, want empty string", decision.Reason)
	}
	if len(decision.Violations) != 0 {
		t.Errorf("violations = %v, want empty", decision.Violations)
	}
	// All counts should be zero
	for sev, count := range decision.CVECounts {
		if count != 0 {
			t.Errorf("CVECounts[%q] = %d, want 0", sev, count)
		}
	}
}

func TestEvaluate_AllBelowThreshold(t *testing.T) {
	eval := NewEvaluator(SeverityHigh)
	report := &harbor.ScanReport{
		Vulnerabilities: []harbor.Vulnerability{
			{ID: "CVE-2024-0001", Severity: "Low"},
			{ID: "CVE-2024-0002", Severity: "Medium"},
			{ID: "CVE-2024-0003", Severity: "Low"},
			{ID: "CVE-2024-0004", Severity: "None"},
		},
	}
	decision := eval.Evaluate(report)

	if !decision.Pass {
		t.Fatal("expected pass when all vulns are below threshold")
	}
	if len(decision.Violations) != 0 {
		t.Errorf("violations = %v, want empty", decision.Violations)
	}
	if decision.CVECounts["Low"] != 2 {
		t.Errorf("CVECounts[Low] = %d, want 2", decision.CVECounts["Low"])
	}
	if decision.CVECounts["Medium"] != 1 {
		t.Errorf("CVECounts[Medium] = %d, want 1", decision.CVECounts["Medium"])
	}
	if decision.CVECounts["None"] != 1 {
		t.Errorf("CVECounts[None] = %d, want 1", decision.CVECounts["None"])
	}
}

func TestEvaluate_HighSeverityFails(t *testing.T) {
	eval := NewEvaluator(SeverityHigh)
	report := &harbor.ScanReport{
		Vulnerabilities: []harbor.Vulnerability{
			{ID: "CVE-2024-0001", Severity: "Low"},
			{ID: "CVE-2024-0002", Severity: "High"},
			{ID: "CVE-2024-0003", Severity: "Medium"},
		},
	}
	decision := eval.Evaluate(report)

	if decision.Pass {
		t.Fatal("expected fail when High vulnerability is present")
	}
	if decision.Reason != "vulnerability_exceeded" {
		t.Errorf("reason = %q, want %q", decision.Reason, "vulnerability_exceeded")
	}
	if len(decision.Violations) != 1 {
		t.Fatalf("violations count = %d, want 1", len(decision.Violations))
	}
	if decision.Violations[0].CVEID != "CVE-2024-0002" {
		t.Errorf("violation CVEID = %q, want %q", decision.Violations[0].CVEID, "CVE-2024-0002")
	}
	if decision.Violations[0].Severity != "High" {
		t.Errorf("violation severity = %q, want %q", decision.Violations[0].Severity, "High")
	}
}

func TestEvaluate_CriticalSeverityFails(t *testing.T) {
	eval := NewEvaluator(SeverityHigh)
	report := &harbor.ScanReport{
		Vulnerabilities: []harbor.Vulnerability{
			{ID: "CVE-2024-0001", Severity: "Critical"},
			{ID: "CVE-2024-0002", Severity: "Low"},
		},
	}
	decision := eval.Evaluate(report)

	if decision.Pass {
		t.Fatal("expected fail when Critical vulnerability is present")
	}
	if decision.Reason != "vulnerability_exceeded" {
		t.Errorf("reason = %q, want %q", decision.Reason, "vulnerability_exceeded")
	}
	if len(decision.Violations) != 1 {
		t.Fatalf("violations count = %d, want 1", len(decision.Violations))
	}
	if decision.Violations[0].CVEID != "CVE-2024-0001" {
		t.Errorf("violation CVEID = %q, want %q", decision.Violations[0].CVEID, "CVE-2024-0001")
	}
}

func TestEvaluate_MultipleViolations(t *testing.T) {
	eval := NewEvaluator(SeverityHigh)
	report := &harbor.ScanReport{
		Vulnerabilities: []harbor.Vulnerability{
			{ID: "CVE-2024-0001", Severity: "High"},
			{ID: "CVE-2024-0002", Severity: "Critical"},
			{ID: "CVE-2024-0003", Severity: "Medium"},
			{ID: "CVE-2024-0004", Severity: "High"},
		},
	}
	decision := eval.Evaluate(report)

	if decision.Pass {
		t.Fatal("expected fail with multiple violations")
	}
	if len(decision.Violations) != 3 {
		t.Fatalf("violations count = %d, want 3", len(decision.Violations))
	}
	if decision.CVECounts["High"] != 2 {
		t.Errorf("CVECounts[High] = %d, want 2", decision.CVECounts["High"])
	}
	if decision.CVECounts["Critical"] != 1 {
		t.Errorf("CVECounts[Critical] = %d, want 1", decision.CVECounts["Critical"])
	}
	if decision.CVECounts["Medium"] != 1 {
		t.Errorf("CVECounts[Medium] = %d, want 1", decision.CVECounts["Medium"])
	}
}

func TestEvaluate_MalformedSeverityFails(t *testing.T) {
	eval := NewEvaluator(SeverityHigh)
	report := &harbor.ScanReport{
		Vulnerabilities: []harbor.Vulnerability{
			{ID: "CVE-2024-0001", Severity: "Low"},
			{ID: "CVE-2024-0002", Severity: "InvalidSeverity"},
		},
	}
	decision := eval.Evaluate(report)

	if decision.Pass {
		t.Fatal("expected fail for malformed severity string")
	}
	if decision.Reason != "vulnerability_exceeded" {
		t.Errorf("reason = %q, want %q", decision.Reason, "vulnerability_exceeded")
	}
}

func TestEvaluate_CVECountsPopulated(t *testing.T) {
	eval := NewEvaluator(SeverityHigh)
	report := &harbor.ScanReport{
		Vulnerabilities: []harbor.Vulnerability{
			{ID: "CVE-2024-0001", Severity: "None"},
			{ID: "CVE-2024-0002", Severity: "Low"},
			{ID: "CVE-2024-0003", Severity: "Low"},
			{ID: "CVE-2024-0004", Severity: "Medium"},
			{ID: "CVE-2024-0005", Severity: "Medium"},
			{ID: "CVE-2024-0006", Severity: "Medium"},
		},
	}
	decision := eval.Evaluate(report)

	if !decision.Pass {
		t.Fatal("expected pass")
	}
	expected := map[string]int{
		"None":     1,
		"Low":      2,
		"Medium":   3,
		"High":     0,
		"Critical": 0,
	}
	for sev, want := range expected {
		got := decision.CVECounts[sev]
		if got != want {
			t.Errorf("CVECounts[%q] = %d, want %d", sev, got, want)
		}
	}
}

func TestEvaluate_CustomThreshold(t *testing.T) {
	// Use a lower threshold — Medium. Anything Medium or above should fail.
	eval := NewEvaluator(SeverityMedium)
	report := &harbor.ScanReport{
		Vulnerabilities: []harbor.Vulnerability{
			{ID: "CVE-2024-0001", Severity: "Low"},
			{ID: "CVE-2024-0002", Severity: "Medium"},
		},
	}
	decision := eval.Evaluate(report)

	if decision.Pass {
		t.Fatal("expected fail with Medium threshold and Medium vulnerability present")
	}
	if len(decision.Violations) != 1 {
		t.Fatalf("violations count = %d, want 1", len(decision.Violations))
	}
	if decision.Violations[0].CVEID != "CVE-2024-0002" {
		t.Errorf("violation CVEID = %q, want %q", decision.Violations[0].CVEID, "CVE-2024-0002")
	}
}

func TestEvaluate_CaseInsensitiveSeverity(t *testing.T) {
	eval := NewEvaluator(SeverityHigh)
	report := &harbor.ScanReport{
		Vulnerabilities: []harbor.Vulnerability{
			{ID: "CVE-2024-0001", Severity: "high"},
			{ID: "CVE-2024-0002", Severity: "CRITICAL"},
		},
	}
	decision := eval.Evaluate(report)

	if decision.Pass {
		t.Fatal("expected fail for case-insensitive high/critical")
	}
	if len(decision.Violations) != 2 {
		t.Fatalf("violations count = %d, want 2", len(decision.Violations))
	}
}
