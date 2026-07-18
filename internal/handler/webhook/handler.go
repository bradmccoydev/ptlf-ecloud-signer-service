// Package webhook implements the Harbor webhook handler that receives
// SCANNING_COMPLETED events and enqueues signing jobs.
package webhook

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/signer-service/internal/config"
	"github.com/signer-service/internal/pkg/harbor"
	"github.com/signer-service/internal/service/gate"
)

// JobSubmitter abstracts the mechanism for enqueuing signing work.
type JobSubmitter interface {
	Submit(artifact gate.ArtifactRef, source string)
}

// Handler serves the POST /webhook endpoint.
type Handler struct {
	cfg    *config.Config
	logger *zap.Logger
	jobs   JobSubmitter
}

// New creates a new webhook Handler.
func New(cfg *config.Config, logger *zap.Logger, jobs JobSubmitter) *Handler {
	return &Handler{
		cfg:    cfg,
		logger: logger,
		jobs:   jobs,
	}
}

// RegisterRoutes attaches the webhook handler to the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/webhook", h.HandleWebhook)
}

// HandleWebhook processes incoming Harbor webhook requests.
func (h *Handler) HandleWebhook(c *gin.Context) {
	// 1. Authenticate: validate shared secret via constant-time comparison.
	secret := c.GetHeader("X-Harbor-Secret")
	if !validateSecret(secret, h.cfg.WebhookSecret) {
		h.logger.Warn("webhook authentication failed",
			zap.String("source_ip", c.ClientIP()),
			zap.Time("timestamp", time.Now().UTC()),
			zap.String("reason", "invalid_secret"),
		)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// 2. Read body with size limit.
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, h.cfg.MaxPayloadSize+1))
	if err != nil {
		h.logger.Error("failed to read request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}
	if int64(len(body)) > h.cfg.MaxPayloadSize {
		h.logger.Warn("webhook payload exceeds max size",
			zap.String("source_ip", c.ClientIP()),
			zap.Int64("max_size", h.cfg.MaxPayloadSize),
			zap.Int("received_size", len(body)),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload too large"})
		return
	}

	// 3. Parse JSON payload.
	var payload harbor.WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.Warn("failed to parse webhook payload",
			zap.String("source_ip", c.ClientIP()),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	// 4. Discard non-SCANNING_COMPLETED events.
	if payload.Type != "SCANNING_COMPLETED" {
		c.JSON(http.StatusOK, gin.H{"status": "ignored", "reason": "event type not relevant"})
		return
	}

	// 5. Extract artifact metadata and validate required fields.
	ref, err := ExtractArtifact(&payload)
	if err != nil {
		h.logger.Warn("webhook payload missing required fields",
			zap.String("source_ip", c.ClientIP()),
			zap.Error(err),
		)
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	// 6. Enqueue signing job.
	h.jobs.Submit(ref, "webhook")

	h.logger.Info("webhook accepted, signing job enqueued",
		zap.String("project", ref.Project),
		zap.String("repo", ref.Repo),
		zap.String("digest", ref.Digest),
	)

	c.JSON(http.StatusOK, gin.H{"status": "accepted"})
}

// validateSecret performs a constant-time comparison between the provided
// and expected webhook secrets.
func validateSecret(provided, expected string) bool {
	if len(provided) == 0 || len(expected) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}
