// Package webhook provides the HTTP handler for Harbor scanning webhooks.
package webhook

import (
	"errors"
	"fmt"

	"github.com/signer-service/internal/pkg/harbor"
	"github.com/signer-service/internal/service/gate"
)

// ExtractArtifact parses a WebhookPayload and extracts the ArtifactRef.
// Returns an error if required fields are missing or malformed.
func ExtractArtifact(payload *harbor.WebhookPayload) (gate.ArtifactRef, error) {
	if payload == nil {
		return gate.ArtifactRef{}, errors.New("payload is nil")
	}

	if len(payload.EventData.Resources) == 0 {
		return gate.ArtifactRef{}, errors.New("payload contains no resources")
	}

	digest := payload.EventData.Resources[0].Digest
	if digest == "" {
		return gate.ArtifactRef{}, errors.New("resource digest is empty")
	}

	project := payload.EventData.Repository.Namespace
	if project == "" {
		return gate.ArtifactRef{}, fmt.Errorf("repository namespace (project) is empty")
	}

	repo := payload.EventData.Repository.Name
	if repo == "" {
		return gate.ArtifactRef{}, fmt.Errorf("repository name is empty")
	}

	return gate.ArtifactRef{
		Project: project,
		Repo:    repo,
		Digest:  digest,
	}, nil
}
