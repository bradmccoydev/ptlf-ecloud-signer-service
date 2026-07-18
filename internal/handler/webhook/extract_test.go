package webhook

import (
	"testing"

	"github.com/signer-service/internal/pkg/harbor"
	"github.com/signer-service/internal/service/gate"
)

func TestExtractArtifact_ValidPayload(t *testing.T) {
	payload := &harbor.WebhookPayload{
		Type: "SCANNING_COMPLETED",
		EventData: harbor.WebhookEventData{
			Resources: []harbor.WebhookResource{
				{
					Digest: "sha256:abc123def456",
					Tag:    "latest",
				},
			},
			Repository: harbor.WebhookRepository{
				Name:      "library/nginx",
				Namespace: "library",
			},
		},
	}

	ref, err := ExtractArtifact(payload)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expected := gate.ArtifactRef{
		Project: "library",
		Repo:    "library/nginx",
		Digest:  "sha256:abc123def456",
	}

	if ref != expected {
		t.Errorf("expected %+v, got %+v", expected, ref)
	}
}

func TestExtractArtifact_NilPayload(t *testing.T) {
	_, err := ExtractArtifact(nil)
	if err == nil {
		t.Fatal("expected error for nil payload, got nil")
	}
	if err.Error() != "payload is nil" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExtractArtifact_EmptyResources(t *testing.T) {
	payload := &harbor.WebhookPayload{
		Type: "SCANNING_COMPLETED",
		EventData: harbor.WebhookEventData{
			Resources: []harbor.WebhookResource{},
			Repository: harbor.WebhookRepository{
				Name:      "library/nginx",
				Namespace: "library",
			},
		},
	}

	_, err := ExtractArtifact(payload)
	if err == nil {
		t.Fatal("expected error for empty resources, got nil")
	}
	if err.Error() != "payload contains no resources" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExtractArtifact_MissingDigest(t *testing.T) {
	payload := &harbor.WebhookPayload{
		Type: "SCANNING_COMPLETED",
		EventData: harbor.WebhookEventData{
			Resources: []harbor.WebhookResource{
				{
					Digest: "",
					Tag:    "latest",
				},
			},
			Repository: harbor.WebhookRepository{
				Name:      "library/nginx",
				Namespace: "library",
			},
		},
	}

	_, err := ExtractArtifact(payload)
	if err == nil {
		t.Fatal("expected error for missing digest, got nil")
	}
	if err.Error() != "resource digest is empty" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExtractArtifact_MissingNamespace(t *testing.T) {
	payload := &harbor.WebhookPayload{
		Type: "SCANNING_COMPLETED",
		EventData: harbor.WebhookEventData{
			Resources: []harbor.WebhookResource{
				{
					Digest: "sha256:abc123def456",
					Tag:    "latest",
				},
			},
			Repository: harbor.WebhookRepository{
				Name:      "library/nginx",
				Namespace: "",
			},
		},
	}

	_, err := ExtractArtifact(payload)
	if err == nil {
		t.Fatal("expected error for missing namespace, got nil")
	}
	if err.Error() != "repository namespace (project) is empty" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExtractArtifact_MissingRepoName(t *testing.T) {
	payload := &harbor.WebhookPayload{
		Type: "SCANNING_COMPLETED",
		EventData: harbor.WebhookEventData{
			Resources: []harbor.WebhookResource{
				{
					Digest: "sha256:abc123def456",
					Tag:    "latest",
				},
			},
			Repository: harbor.WebhookRepository{
				Name:      "",
				Namespace: "library",
			},
		},
	}

	_, err := ExtractArtifact(payload)
	if err == nil {
		t.Fatal("expected error for missing repo name, got nil")
	}
	if err.Error() != "repository name is empty" {
		t.Errorf("unexpected error message: %v", err)
	}
}
