package webhook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/signer-service/internal/config"
	"github.com/signer-service/internal/pkg/harbor"
	"github.com/signer-service/internal/service/gate"
)

// mockJobSubmitter records submissions for test verification.
type mockJobSubmitter struct {
	submitted []submittedJob
}

type submittedJob struct {
	artifact gate.ArtifactRef
	source   string
}

func (m *mockJobSubmitter) Submit(artifact gate.ArtifactRef, source string) {
	m.submitted = append(m.submitted, submittedJob{artifact: artifact, source: source})
}

func newTestHandler(secret string, maxPayload int64) (*Handler, *mockJobSubmitter) {
	cfg := &config.Config{
		WebhookSecret:  secret,
		MaxPayloadSize: maxPayload,
	}
	logger, _ := zap.NewNop(), error(nil)
	mock := &mockJobSubmitter{}
	h := New(cfg, logger, mock)
	return h, mock
}

func setupRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	group := r.Group("")
	h.RegisterRoutes(group)
	return r
}

func validPayloadJSON() []byte {
	payload := harbor.WebhookPayload{
		Type: "SCANNING_COMPLETED",
		EventData: harbor.WebhookEventData{
			Resources: []harbor.WebhookResource{
				{Digest: "sha256:abc123def456"},
			},
			Repository: harbor.WebhookRepository{
				Name:      "library/nginx",
				Namespace: "library",
			},
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

func TestHandleWebhook_InvalidSecret_Returns401(t *testing.T) {
	h, _ := newTestHandler("correct-secret", 1048576)
	router := setupRouter(h)

	body := validPayloadJSON()
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Harbor-Secret", "wrong-secret")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleWebhook_MissingSecret_Returns401(t *testing.T) {
	h, _ := newTestHandler("correct-secret", 1048576)
	router := setupRouter(h)

	body := validPayloadJSON()
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	// No X-Harbor-Secret header
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleWebhook_PayloadTooLarge_Returns400(t *testing.T) {
	h, _ := newTestHandler("my-secret", 100) // 100 bytes max
	router := setupRouter(h)

	// Create a body larger than 100 bytes
	largeBody := strings.Repeat("x", 200)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(largeBody))
	req.Header.Set("X-Harbor-Secret", "my-secret")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleWebhook_InvalidJSON_Returns400(t *testing.T) {
	h, _ := newTestHandler("my-secret", 1048576)
	router := setupRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader("not json"))
	req.Header.Set("X-Harbor-Secret", "my-secret")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleWebhook_NonScanningCompleted_Returns200(t *testing.T) {
	h, mock := newTestHandler("my-secret", 1048576)
	router := setupRouter(h)

	payload := harbor.WebhookPayload{
		Type: "PUSH_ARTIFACT",
		EventData: harbor.WebhookEventData{
			Resources: []harbor.WebhookResource{
				{Digest: "sha256:abc123"},
			},
			Repository: harbor.WebhookRepository{
				Name:      "library/nginx",
				Namespace: "library",
			},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Harbor-Secret", "my-secret")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if len(mock.submitted) != 0 {
		t.Errorf("expected no jobs submitted for non-SCANNING_COMPLETED event, got %d", len(mock.submitted))
	}
}

func TestHandleWebhook_MissingDigest_Returns422(t *testing.T) {
	h, _ := newTestHandler("my-secret", 1048576)
	router := setupRouter(h)

	payload := harbor.WebhookPayload{
		Type: "SCANNING_COMPLETED",
		EventData: harbor.WebhookEventData{
			Resources: []harbor.WebhookResource{
				{Digest: ""}, // empty digest
			},
			Repository: harbor.WebhookRepository{
				Name:      "library/nginx",
				Namespace: "library",
			},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Harbor-Secret", "my-secret")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestHandleWebhook_MissingProject_Returns422(t *testing.T) {
	h, _ := newTestHandler("my-secret", 1048576)
	router := setupRouter(h)

	payload := harbor.WebhookPayload{
		Type: "SCANNING_COMPLETED",
		EventData: harbor.WebhookEventData{
			Resources: []harbor.WebhookResource{
				{Digest: "sha256:abc123"},
			},
			Repository: harbor.WebhookRepository{
				Name:      "library/nginx",
				Namespace: "", // empty namespace/project
			},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Harbor-Secret", "my-secret")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestHandleWebhook_MissingRepository_Returns422(t *testing.T) {
	h, _ := newTestHandler("my-secret", 1048576)
	router := setupRouter(h)

	payload := harbor.WebhookPayload{
		Type: "SCANNING_COMPLETED",
		EventData: harbor.WebhookEventData{
			Resources: []harbor.WebhookResource{
				{Digest: "sha256:abc123"},
			},
			Repository: harbor.WebhookRepository{
				Name:      "", // empty repo name
				Namespace: "library",
			},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Harbor-Secret", "my-secret")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestHandleWebhook_NoResources_Returns422(t *testing.T) {
	h, _ := newTestHandler("my-secret", 1048576)
	router := setupRouter(h)

	payload := harbor.WebhookPayload{
		Type: "SCANNING_COMPLETED",
		EventData: harbor.WebhookEventData{
			Resources:  []harbor.WebhookResource{}, // no resources
			Repository: harbor.WebhookRepository{Name: "library/nginx", Namespace: "library"},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Harbor-Secret", "my-secret")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestHandleWebhook_ValidPayload_Returns200AndEnqueues(t *testing.T) {
	h, mock := newTestHandler("my-secret", 1048576)
	router := setupRouter(h)

	body := validPayloadJSON()
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Harbor-Secret", "my-secret")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if len(mock.submitted) != 1 {
		t.Fatalf("expected 1 job submitted, got %d", len(mock.submitted))
	}
	job := mock.submitted[0]
	if job.artifact.Project != "library" {
		t.Errorf("expected project 'library', got %q", job.artifact.Project)
	}
	if job.artifact.Repo != "library/nginx" {
		t.Errorf("expected repo 'library/nginx', got %q", job.artifact.Repo)
	}
	if job.artifact.Digest != "sha256:abc123def456" {
		t.Errorf("expected digest 'sha256:abc123def456', got %q", job.artifact.Digest)
	}
	if job.source != "webhook" {
		t.Errorf("expected source 'webhook', got %q", job.source)
	}
}

func TestValidateSecret(t *testing.T) {
	tests := []struct {
		name     string
		provided string
		expected string
		want     bool
	}{
		{"matching", "secret123", "secret123", true},
		{"not matching", "wrong", "secret123", false},
		{"empty provided", "", "secret123", false},
		{"empty expected", "secret123", "", false},
		{"both empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateSecret(tt.provided, tt.expected)
			if got != tt.want {
				t.Errorf("validateSecret(%q, %q) = %v, want %v", tt.provided, tt.expected, got, tt.want)
			}
		})
	}
}
