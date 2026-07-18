package gate

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: signer-service, Property 2: Severity parsing round-trip
// **Validates: Requirements 2.2**

// TestPropertySeverityParsingRoundTrip verifies that for each valid severity string
// (None, Low, Medium, High, Critical), parsing to the Severity enum and converting
// back to a string produces the original value.
func TestPropertySeverityParsingRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator: randomly select from the set of valid severity strings
	validSeverities := []interface{}{"None", "Low", "Medium", "High", "Critical"}
	genValidSeverity := gen.OneConstOf(validSeverities...)

	properties.Property("ParseSeverity then String round-trips to original", prop.ForAll(
		func(s interface{}) bool {
			original := s.(string)

			// Parse the string to the Severity enum
			sev, err := ParseSeverity(original)
			if err != nil {
				return false
			}

			// Convert back to string
			reconstructed := sev.String()

			// Assert original equals reconstructed
			return original == reconstructed
		},
		genValidSeverity,
	))

	properties.TestingRun(t)
}
