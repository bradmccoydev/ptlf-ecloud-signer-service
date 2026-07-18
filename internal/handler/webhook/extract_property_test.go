package webhook

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/signer-service/internal/pkg/harbor"
)

// Feature: signer-service, Property 5: Webhook payload extraction completeness

// TestProperty5a_ValidPayloadsAlwaysExtractSuccessfully verifies that for any valid
// Harbor webhook payload containing a non-empty digest (prefixed with "sha256:"),
// a non-empty namespace (project), and a non-empty repository name, the extraction
// function returns no error and the returned ArtifactRef fields match the input.
// **Validates: Requirements 1.3, 1.4, 1.7**
func TestProperty5a_ValidPayloadsAlwaysExtractSuccessfully(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("valid payloads always extract successfully", prop.ForAll(
		func(digest string, namespace string, repoName string) bool {
			// Prefix digest with sha256: to make it realistic
			fullDigest := "sha256:" + digest

			payload := &harbor.WebhookPayload{
				Type: "SCANNING_COMPLETED",
				EventData: harbor.WebhookEventData{
					Resources: []harbor.WebhookResource{
						{
							Digest: fullDigest,
							Tag:    "latest",
						},
					},
					Repository: harbor.WebhookRepository{
						Name:      repoName,
						Namespace: namespace,
					},
				},
			}

			ref, err := ExtractArtifact(payload)
			if err != nil {
				t.Logf("Expected no error for valid payload, got: %v (digest=%q, namespace=%q, repo=%q)",
					err, fullDigest, namespace, repoName)
				return false
			}

			if ref.Project != namespace {
				t.Logf("Expected Project=%q, got %q", namespace, ref.Project)
				return false
			}
			if ref.Repo != repoName {
				t.Logf("Expected Repo=%q, got %q", repoName, ref.Repo)
				return false
			}
			if ref.Digest != fullDigest {
				t.Logf("Expected Digest=%q, got %q", fullDigest, ref.Digest)
				return false
			}

			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }), // digest suffix
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }), // namespace
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }), // repoName
	))

	properties.TestingRun(t)
}

// TestProperty5b_PayloadsMissingRequiredFieldsAlwaysReturnError verifies that for any
// payload where one required field is zeroed out (empty digest, empty namespace,
// empty repo name, or empty resources slice), the extraction function always returns
// an error.
// **Validates: Requirements 1.3, 1.4, 1.7**
func TestProperty5b_PayloadsMissingRequiredFieldsAlwaysReturnError(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("payloads missing required fields always return error", prop.ForAll(
		func(digest string, namespace string, repoName string, missingField int) bool {
			// Prefix digest with sha256: to make it realistic
			fullDigest := "sha256:" + digest

			// Start with a valid payload
			payload := &harbor.WebhookPayload{
				Type: "SCANNING_COMPLETED",
				EventData: harbor.WebhookEventData{
					Resources: []harbor.WebhookResource{
						{
							Digest: fullDigest,
							Tag:    "latest",
						},
					},
					Repository: harbor.WebhookRepository{
						Name:      repoName,
						Namespace: namespace,
					},
				},
			}

			// Zero out one required field based on missingField selector
			switch missingField {
			case 0:
				// Empty resources slice
				payload.EventData.Resources = []harbor.WebhookResource{}
			case 1:
				// Empty digest
				payload.EventData.Resources[0].Digest = ""
			case 2:
				// Empty namespace
				payload.EventData.Repository.Namespace = ""
			case 3:
				// Empty repo name
				payload.EventData.Repository.Name = ""
			}

			_, err := ExtractArtifact(payload)
			if err == nil {
				t.Logf("Expected error for payload missing field %d, got nil (digest=%q, namespace=%q, repo=%q)",
					missingField, fullDigest, namespace, repoName)
				return false
			}

			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }), // digest suffix
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }), // namespace
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }), // repoName
		gen.IntRange(0, 3), // missingField selector (0=resources, 1=digest, 2=namespace, 3=repo)
	))

	properties.TestingRun(t)
}
