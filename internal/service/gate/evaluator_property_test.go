package gate

import (
	"fmt"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/signer-service/internal/pkg/harbor"
)

// Feature: signer-service, Property 1: Severity gate is monotonically strict
// **Validates: Requirements 2.3, 2.4**

// TestPropertySeverityGateMonotonicity verifies that the gate evaluator is monotonically strict:
// - If the evaluator returns pass, ALL vulnerabilities must have severity below High.
// - If ANY vulnerability has severity >= High, the evaluator must return fail.
func TestPropertySeverityGateMonotonicity(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Valid severity strings for generation
	validSeverities := []string{"None", "Low", "Medium", "High", "Critical"}

	// Generator for a single vulnerability with a random valid severity
	genVulnerability := gen.IntRange(0, len(validSeverities)-1).Map(func(idx int) harbor.Vulnerability {
		return harbor.Vulnerability{
			ID:       fmt.Sprintf("CVE-2024-%04d", idx),
			Severity: validSeverities[idx],
		}
	})

	// Generator for a slice of vulnerabilities (0 to 50 items)
	genVulnerabilities := gen.SliceOfN(50, genVulnerability).
		SuchThat(func(v []harbor.Vulnerability) bool { return true })

	properties.Property("pass implies all vulnerabilities below High", prop.ForAll(
		func(vulns []harbor.Vulnerability) bool {
			report := &harbor.ScanReport{
				Vulnerabilities: vulns,
			}
			eval := NewEvaluator(SeverityHigh)
			decision := eval.Evaluate(report)

			if decision.Pass {
				// If pass, every vulnerability must be below High
				for _, v := range vulns {
					sev, err := ParseSeverity(v.Severity)
					if err != nil {
						// Should not happen with valid severities, but if it does, fail
						return false
					}
					if sev >= SeverityHigh {
						return false
					}
				}
			}
			return true
		},
		genVulnerabilities,
	))

	properties.Property("any vulnerability >= High implies fail", prop.ForAll(
		func(vulns []harbor.Vulnerability) bool {
			report := &harbor.ScanReport{
				Vulnerabilities: vulns,
			}
			eval := NewEvaluator(SeverityHigh)
			decision := eval.Evaluate(report)

			// Check if any vulnerability is >= High
			hasHighOrAbove := false
			for _, v := range vulns {
				sev, err := ParseSeverity(v.Severity)
				if err != nil {
					continue
				}
				if sev >= SeverityHigh {
					hasHighOrAbove = true
					break
				}
			}

			if hasHighOrAbove {
				// Must fail
				return !decision.Pass
			}
			return true
		},
		genVulnerabilities,
	))

	properties.TestingRun(t)
}
