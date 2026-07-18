// Package health implements the health and readiness endpoints for the signer-service.
// GET /health returns 200 if the process is running.
// GET /ready returns 200 only when all critical dependencies (Harbor, Fulcio, Rekor)
// are reachable; 503 otherwise. It also returns 503 immediately when a shutdown
// signal (SIGTERM) has been received.
package health

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// DependencyChecker checks reachability of an external dependency.
type DependencyChecker interface {
	// Name returns a human-readable identifier for the dependency.
	Name() string
	// Check verifies the dependency is reachable. Returns nil on success.
	Check(ctx context.Context) error
	// Critical indicates whether this dependency must be healthy for the
	// service to be considered ready.
	Critical() bool
}

// Handler serves the /health and /ready endpoints.
type Handler struct {
	checkers     []DependencyChecker
	shutdownFlag *atomic.Bool
}

// New creates a new health Handler with the given dependency checkers.
func New(checkers []DependencyChecker) *Handler {
	return &Handler{
		checkers:     checkers,
		shutdownFlag: &atomic.Bool{},
	}
}

// RegisterRoutes attaches the health and readiness handlers to the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/health", h.HandleHealth)
	rg.GET("/ready", h.HandleReady)
}

// HandleHealth returns 200 if the process is running. This is a simple liveness check.
func (h *Handler) HandleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// HandleReady returns 200 when all critical dependencies are reachable, or 503 otherwise.
// It immediately returns 503 if the service is shutting down.
func (h *Handler) HandleReady(c *gin.Context) {
	// If shutdown has been signalled, respond 503 immediately.
	if h.shutdownFlag.Load() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "shutting_down"})
		return
	}

	// Check each dependency with a 5-second timeout.
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	var failed []string
	for _, checker := range h.checkers {
		if err := checker.Check(ctx); err != nil {
			if checker.Critical() {
				failed = append(failed, checker.Name())
			}
		}
	}

	if len(failed) > 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":       "unavailable",
			"dependencies": failed,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

// SetShutdown marks the service as shutting down. Once set, the readiness
// endpoint will return 503 for all subsequent requests. This should be
// called by main on receipt of SIGTERM.
func (h *Handler) SetShutdown() {
	h.shutdownFlag.Store(true)
}

// IsShuttingDown returns whether the shutdown flag has been set.
func (h *Handler) IsShuttingDown() bool {
	return h.shutdownFlag.Load()
}

// HTTPChecker verifies connectivity to an HTTP endpoint by performing a
// HEAD request and expecting a non-5xx response.
type HTTPChecker struct {
	name     string
	url      string
	client   *http.Client
	critical bool
}

// NewHTTPChecker creates a new HTTPChecker for the given dependency.
func NewHTTPChecker(name, url string, critical bool) *HTTPChecker {
	return &HTTPChecker{
		name: name,
		url:  url,
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
		critical: critical,
	}
}

// Name returns the dependency name.
func (c *HTTPChecker) Name() string {
	return c.name
}

// Check performs a HEAD request to the configured URL.
// Returns nil if the endpoint responds with a non-5xx status.
func (c *HTTPChecker) Check(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, c.url, nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return &DependencyError{
			Dependency: c.name,
			StatusCode: resp.StatusCode,
		}
	}

	return nil
}

// Critical returns whether this dependency is critical for readiness.
func (c *HTTPChecker) Critical() bool {
	return c.critical
}

// DependencyError represents a failed dependency check.
type DependencyError struct {
	Dependency string
	StatusCode int
}

func (e *DependencyError) Error() string {
	return e.Dependency + ": unhealthy (status " + http.StatusText(e.StatusCode) + ")"
}
