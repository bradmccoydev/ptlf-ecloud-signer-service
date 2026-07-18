package webhook

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: signer-service, Property 3: Webhook secret validation rejects invalid secrets
// **Validates: Requirements 1.1, 1.2**

// TestPropertyWebhookSecretValidation verifies that:
// - If provided == expected (both non-empty), validateSecret returns true
// - If provided != expected, validateSecret returns false
// - If provided is empty, validateSecret always returns false
// - If expected is empty, validateSecret always returns false
func TestPropertyWebhookSecretValidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator for non-empty strings (1-100 chars) — uses Identifier which never
	// generates empty strings, avoiding SuchThat filter discards.
	genNonEmptySecret := gen.Identifier().Map(func(s string) string {
		// Identifier always produces non-empty alphanumeric strings
		if len(s) > 100 {
			return s[:100]
		}
		return s
	})

	// Generator that produces two distinct non-empty strings
	genTwoDistinctSecrets := gopter.CombineGens(
		genNonEmptySecret,
		genNonEmptySecret,
	).Map(func(vals []interface{}) []string {
		a := vals[0].(string)
		b := vals[1].(string)
		// Ensure they differ by appending a suffix to one
		if a == b {
			b = b + "X"
		}
		return []string{a, b}
	})

	// Property: matching secrets return true
	properties.Property("matching non-empty secrets return true", prop.ForAll(
		func(secret string) bool {
			return validateSecret(secret, secret)
		},
		genNonEmptySecret,
	))

	// Property: non-matching secrets return false
	properties.Property("non-matching secrets return false", prop.ForAll(
		func(pair []string) bool {
			provided := pair[0]
			expected := pair[1]
			return !validateSecret(provided, expected)
		},
		genTwoDistinctSecrets,
	))

	// Property: empty provided always returns false
	properties.Property("empty provided always returns false", prop.ForAll(
		func(expected string) bool {
			return !validateSecret("", expected)
		},
		genNonEmptySecret,
	))

	// Property: empty expected always returns false
	properties.Property("empty expected always returns false", prop.ForAll(
		func(provided string) bool {
			return !validateSecret(provided, "")
		},
		genNonEmptySecret,
	))

	// Property: both empty returns false
	properties.Property("both empty returns false", prop.ForAll(
		func(_ int) bool {
			return !validateSecret("", "")
		},
		gen.Const(0),
	))

	// Property: arbitrary strings that differ from expected are always rejected,
	// and strings equal to expected are always accepted
	properties.Property("validation is correct for any provided string given a fixed expected", prop.ForAll(
		func(expected string, provided string) bool {
			result := validateSecret(provided, expected)
			if provided == expected {
				return result == true
			}
			// Empty provided is always false (already covered above), but also
			// handles the case where AnyString generates empty
			if len(provided) == 0 {
				return result == false
			}
			return result == false
		},
		genNonEmptySecret,
		gen.AnyString(),
	))

	properties.TestingRun(t)
}
