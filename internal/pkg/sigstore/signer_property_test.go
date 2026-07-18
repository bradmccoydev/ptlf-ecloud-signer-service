package sigstore

import (
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: signer-service, Property 8: Annotation completeness on signed images
// **Validates: Requirements 3.4**

// TestPropertyAnnotationCompleteness verifies that for any successful signing operation,
// the resulting annotations contain exactly the required keys: scan-report, trivy-db,
// policy, and a valid RFC 3339 UTC timestamp.
func TestPropertyAnnotationCompleteness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator for non-empty alphanumeric strings (annotation values)
	genNonEmptyString := gen.AlphaString().SuchThat(func(s string) bool {
		return len(s) > 0
	})

	// Property: ensureAnnotations + ValidateAnnotations produces valid result
	// when all required keys (scan-report, trivy-db, policy) are provided with non-empty values.
	properties.Property("annotations with all required keys pass validation after ensureAnnotations", prop.ForAll(
		func(scanReport, trivyDB, policy string) bool {
			annotations := map[string]string{
				"scan-report": scanReport,
				"trivy-db":    trivyDB,
				"policy":      policy,
			}

			// ensureAnnotations adds timestamp if missing
			result := ensureAnnotations(annotations)

			// Assert: result contains exactly the four required keys
			requiredKeys := RequiredAnnotationKeys()
			if len(result) != len(requiredKeys) {
				return false
			}
			for _, key := range requiredKeys {
				if _, ok := result[key]; !ok {
					return false
				}
			}

			// Assert: ValidateAnnotations returns nil (no error)
			if err := ValidateAnnotations(result); err != nil {
				return false
			}

			// Assert: timestamp is valid RFC 3339 UTC
			ts := result["timestamp"]
			parsedTime, err := time.Parse(time.RFC3339, ts)
			if err != nil {
				return false
			}
			if parsedTime.Location() != time.UTC {
				return false
			}

			return true
		},
		genNonEmptyString,
		genNonEmptyString,
		genNonEmptyString,
	))

	// Property: if any required key is missing, ValidateAnnotations returns an error.
	properties.Property("missing required key causes ValidateAnnotations to fail", prop.ForAll(
		func(scanReport, trivyDB, policy string, keyToRemove int) bool {
			annotations := map[string]string{
				"scan-report": scanReport,
				"trivy-db":    trivyDB,
				"policy":      policy,
				"timestamp":   time.Now().UTC().Format(time.RFC3339),
			}

			// Remove one of the required keys based on keyToRemove index
			keysToTest := []string{"scan-report", "trivy-db", "policy", "timestamp"}
			removeKey := keysToTest[keyToRemove%len(keysToTest)]
			delete(annotations, removeKey)

			// Assert: ValidateAnnotations returns an error
			err := ValidateAnnotations(annotations)
			return err != nil
		},
		genNonEmptyString,
		genNonEmptyString,
		genNonEmptyString,
		gen.IntRange(0, 3),
	))

	// Property: ensureAnnotations preserves existing timestamp if already present.
	properties.Property("ensureAnnotations preserves existing valid timestamp", prop.ForAll(
		func(scanReport, trivyDB, policy string) bool {
			existingTimestamp := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
			annotations := map[string]string{
				"scan-report": scanReport,
				"trivy-db":    trivyDB,
				"policy":      policy,
				"timestamp":   existingTimestamp,
			}

			result := ensureAnnotations(annotations)

			// Timestamp should be preserved (not overwritten)
			if result["timestamp"] != existingTimestamp {
				return false
			}

			// ValidateAnnotations should still pass
			return ValidateAnnotations(result) == nil
		},
		genNonEmptyString,
		genNonEmptyString,
		genNonEmptyString,
	))

	properties.TestingRun(t)
}
