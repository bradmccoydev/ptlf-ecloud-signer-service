// Feature: signer-service, Property 6: Gate decision reason labelling is exhaustive
package gate

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/signer-service/internal/pkg/harbor"
)

// validReasons is the exhaustive set of valid gate failure reason labels.
var validReasons = map[string]bool{
	"vulnerability_exceeded": true,
	"scan_fetch_error":       true,
	"signing_error":          true,
	"fulcio_error":           true,
	"rekor_error":            true,
}

// Validates: Requirements 4.1
func TestProperty6_GateDecisionReasonLabelling(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator for valid severity strings (known good values).
	validSeverities := []interface{}{"None", "Low", "Medium", "High", "Critical"}
	genValidSeverity := gen.OneConstOf(validSeverities...)

	// Generator for malformed severity strings.
	genMalformedSeverity := gen.AnyString().SuchThat(func(v interface{}) bool {
		s := v.(string)
		_, err := ParseSeverity(s)
		return err != nil
	})

	// Generator for a vulnerability with a valid severity.
	genValidVuln := gopter.CombineGens(
		gen.AlphaString(),
		genValidSeverity,
	).Map(func(vals []interface{}) harbor.Vulnerability {
		return harbor.Vulnerability{
			ID:       "CVE-" + vals[0].(string),
			Severity: vals[1].(string),
		}
	})

	// Generator for a vulnerability with a malformed severity.
	genMalformedVuln := gopter.CombineGens(
		gen.AlphaString(),
		genMalformedSeverity,
	).Map(func(vals []interface{}) harbor.Vulnerability {
		return harbor.Vulnerability{
			ID:       "CVE-" + vals[0].(string),
			Severity: vals[1].(string),
		}
	})

	// Property: nil report produces reason in valid set
	properties.Property("nil report reason is in valid set", prop.ForAll(
		func(threshold int) bool {
			sev := Severity(threshold)
			eval := NewEvaluator(sev)
			decision := eval.Evaluate(nil)

			if decision.Pass {
				return false // nil should never pass
			}
			return validReasons[decision.Reason]
		},
		gen.IntRange(0, 4),
	))

	// Property: reports with valid vulnerabilities produce valid reasons or pass with empty reason
	properties.Property("valid vulnerability reports have valid reason labels", prop.ForAll(
		func(vulns []harbor.Vulnerability) bool {
			eval := NewEvaluator(SeverityHigh)
			report := &harbor.ScanReport{Vulnerabilities: vulns}
			decision := eval.Evaluate(report)

			if decision.Pass {
				// Passing decisions must have empty reason
				return decision.Reason == ""
			}
			// Failing decisions must have a reason from the valid set
			return validReasons[decision.Reason]
		},
		gen.SliceOf(genValidVuln),
	))

	// Property: reports with malformed severity produce valid reason labels
	properties.Property("malformed severity reports have valid reason labels", prop.ForAll(
		func(malformedVuln harbor.Vulnerability, validVulns []harbor.Vulnerability) bool {
			// Insert the malformed vuln into the list
			vulns := append(validVulns, malformedVuln)
			eval := NewEvaluator(SeverityHigh)
			report := &harbor.ScanReport{Vulnerabilities: vulns}
			decision := eval.Evaluate(report)

			if decision.Pass {
				// Should not pass with malformed data, but if it does, reason must be empty
				return decision.Reason == ""
			}
			return validReasons[decision.Reason]
		},
		genMalformedVuln,
		gen.SliceOf(genValidVuln),
	))

	// Property: for any passing decision, reason is always empty string
	properties.Property("passing decisions always have empty reason", prop.ForAll(
		func(vulns []harbor.Vulnerability) bool {
			eval := NewEvaluator(SeverityHigh)
			report := &harbor.ScanReport{Vulnerabilities: vulns}
			decision := eval.Evaluate(report)

			if decision.Pass {
				return decision.Reason == ""
			}
			// Not a pass — we don't constrain this case here
			return true
		},
		gen.SliceOf(genValidVuln),
	))

	properties.TestingRun(t)
}
