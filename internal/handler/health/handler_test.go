package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// mockChecker is a test double for DependencyChecker.
type mockChecker struct {
	name     string
	err      error
	critical bool
}

func (m *mockChecker) Name() string                  { return m.name }
func (m *mockChecker) Check(_ context.Context) error { return m.err }
func (m *mockChecker) Critical() bool                { return m.critical }

func setupRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("")
	h.RegisterRoutes(rg)
	return r
}

func TestHandleHealth_Returns200(t *testing.T) {
	h := New(nil)
	router := setupRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestHandleReady_AllHealthy(t *testing.T) {
	checkers := []DependencyChecker{
		&mockChecker{name: "harbor", err: nil, critical: true},
		&mockChecker{name: "fulcio", err: nil, critical: true},
		&mockChecker{name: "rekor", err: nil, critical: true},
	}
	h := New(checkers)
	router := setupRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/ready", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestHandleReady_CriticalDependencyDown(t *testing.T) {
	checkers := []DependencyChecker{
		&mockChecker{name: "harbor", err: nil, critical: true},
		&mockChecker{name: "fulcio", err: errors.New("connection refused"), critical: true},
		&mockChecker{name: "rekor", err: nil, critical: true},
	}
	h := New(checkers)
	router := setupRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/ready", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", w.Code)
	}
}

func TestHandleReady_NonCriticalDependencyDown(t *testing.T) {
	checkers := []DependencyChecker{
		&mockChecker{name: "harbor", err: nil, critical: true},
		&mockChecker{name: "optional-svc", err: errors.New("timeout"), critical: false},
	}
	h := New(checkers)
	router := setupRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/ready", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 (non-critical failure), got %d", w.Code)
	}
}

func TestHandleReady_ShutdownFlag(t *testing.T) {
	checkers := []DependencyChecker{
		&mockChecker{name: "harbor", err: nil, critical: true},
	}
	h := New(checkers)
	h.SetShutdown()
	router := setupRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/ready", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503 after shutdown, got %d", w.Code)
	}
}

func TestSetShutdown_SetsFlag(t *testing.T) {
	h := New(nil)
	if h.IsShuttingDown() {
		t.Fatal("expected shutdown flag to be false initially")
	}
	h.SetShutdown()
	if !h.IsShuttingDown() {
		t.Fatal("expected shutdown flag to be true after SetShutdown")
	}
}

func TestHTTPChecker_Name(t *testing.T) {
	c := NewHTTPChecker("harbor", "http://localhost", true)
	if c.Name() != "harbor" {
		t.Fatalf("expected name 'harbor', got %q", c.Name())
	}
}

func TestHTTPChecker_Critical(t *testing.T) {
	c := NewHTTPChecker("rekor", "http://localhost", true)
	if !c.Critical() {
		t.Fatal("expected critical to be true")
	}

	c2 := NewHTTPChecker("optional", "http://localhost", false)
	if c2.Critical() {
		t.Fatal("expected critical to be false")
	}
}

func TestHTTPChecker_CheckSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewHTTPChecker("test-svc", srv.URL, true)
	err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestHTTPChecker_Check5xxReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewHTTPChecker("test-svc", srv.URL, true)
	err := c.Check(context.Background())
	if err == nil {
		t.Fatal("expected error for 5xx response")
	}
}

func TestHTTPChecker_CheckUnreachable(t *testing.T) {
	c := NewHTTPChecker("unreachable", "http://127.0.0.1:1", true)
	err := c.Check(context.Background())
	if err == nil {
		t.Fatal("expected error for unreachable host")
	}
}
