package gate

import (
	"github.com/signer-service/internal/pkg/harbor"
)

// Evaluator evaluates scan reports against the severity threshold.
type Evaluator interface {
	Evaluate(report *harbor.ScanReport) GateDecision
}

// DefaultEvaluator implements the Evaluator interface using a configurable severity threshold.
type DefaultEvaluator struct {
	Threshold Severity
}

// NewEvaluator creates a new DefaultEvaluator with the given severity threshold.
func NewEvaluator(threshold Severity) *DefaultEvaluator {
	return &DefaultEvaluator{Threshold: threshold}
}

// Evaluate inspects the scan report's vulnerabilities against the configured threshold.
// Returns a GateDecision indicating whether the artifact passes or fails the gate.
func (e *DefaultEvaluator) Evaluate(report *harbor.ScanReport) GateDecision {
	// Nil report is treated as a malformed/fetch-error scenario — fail without retry.
	if report == nil {
		return GateDecision{
			Pass:   false,
			Reason: "scan_fetch_error",
		}
	}

	// Initialize CVE counts for all known severity levels.
	cveCounts := map[string]int{
		SeverityNone.String():     0,
		SeverityLow.String():      0,
		SeverityMedium.String():   0,
		SeverityHigh.String():     0,
		SeverityCritical.String(): 0,
	}

	var violations []ViolationDetail

	for _, vuln := range report.Vulnerabilities {
		sev, err := ParseSeverity(vuln.Severity)
		if err != nil {
			// Malformed severity string — treat report as malformed, fail without retry.
			return GateDecision{
				Pass:   false,
				Reason: "vulnerability_exceeded",
				CVECounts: map[string]int{
					SeverityNone.String():     0,
					SeverityLow.String():      0,
					SeverityMedium.String():   0,
					SeverityHigh.String():     0,
					SeverityCritical.String(): 0,
				},
			}
		}

		cveCounts[sev.String()]++

		if sev >= e.Threshold {
			violations = append(violations, ViolationDetail{
				CVEID:    vuln.ID,
				Severity: sev.String(),
			})
		}
	}

	if len(violations) > 0 {
		return GateDecision{
			Pass:       false,
			Reason:     "vulnerability_exceeded",
			CVECounts:  cveCounts,
			Violations: violations,
		}
	}

	return GateDecision{
		Pass:      true,
		CVECounts: cveCounts,
	}
}
