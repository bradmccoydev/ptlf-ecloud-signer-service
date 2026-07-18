// Package gate provides the severity gate evaluator that determines whether
// an artifact's scan report passes the signing threshold.
package gate

import (
	"fmt"
	"strings"
)

// Severity represents a vulnerability severity level as an integer enum.
type Severity int

const (
	// SeverityNone indicates no severity.
	SeverityNone Severity = 0
	// SeverityLow indicates a low-severity vulnerability.
	SeverityLow Severity = 1
	// SeverityMedium indicates a medium-severity vulnerability.
	SeverityMedium Severity = 2
	// SeverityHigh indicates a high-severity vulnerability.
	SeverityHigh Severity = 3
	// SeverityCritical indicates a critical-severity vulnerability.
	SeverityCritical Severity = 4
)

// String returns the canonical capitalized string representation of a Severity.
func (s Severity) String() string {
	switch s {
	case SeverityNone:
		return "None"
	case SeverityLow:
		return "Low"
	case SeverityMedium:
		return "Medium"
	case SeverityHigh:
		return "High"
	case SeverityCritical:
		return "Critical"
	default:
		return fmt.Sprintf("Unknown(%d)", int(s))
	}
}

// ParseSeverity converts a case-insensitive string to a Severity value.
// Returns an error if the string does not match a known severity level.
func ParseSeverity(s string) (Severity, error) {
	switch strings.ToLower(s) {
	case "none":
		return SeverityNone, nil
	case "low":
		return SeverityLow, nil
	case "medium":
		return SeverityMedium, nil
	case "high":
		return SeverityHigh, nil
	case "critical":
		return SeverityCritical, nil
	default:
		return SeverityNone, fmt.Errorf("unknown severity: %q", s)
	}
}

// GateDecision represents the outcome of evaluating a scan report against the severity threshold.
type GateDecision struct {
	// Pass indicates whether the artifact passed the gate.
	Pass bool
	// Reason describes why the gate failed (e.g., "vulnerability_exceeded").
	Reason string
	// Artifact identifies the artifact that was evaluated.
	Artifact ArtifactRef
	// CVECounts maps severity level strings to the number of CVEs at that level.
	CVECounts map[string]int
	// Violations lists individual CVEs that exceeded the threshold.
	Violations []ViolationDetail
}

// ViolationDetail describes a single CVE that violated the severity threshold.
type ViolationDetail struct {
	CVEID    string
	Severity string
}

// ArtifactRef identifies an artifact by project, repository, and digest.
type ArtifactRef struct {
	Project string
	Repo    string
	Digest  string
}
