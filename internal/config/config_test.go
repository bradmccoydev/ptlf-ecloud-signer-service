package config

import (
	"os"
	"testing"
	"time"
)

// setRequiredEnv sets all required environment variables to valid values.
func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HARBOR_URL", "https://registry.platform.cuscal.io")
	t.Setenv("HARBOR_ROBOT_USER", "robot$signer")
	t.Setenv("HARBOR_ROBOT_PASSWORD", "secret-password")
	t.Setenv("FULCIO_URL", "https://fulcio.platform.cuscal.io")
	t.Setenv("REKOR_URL", "https://rekor.platform.cuscal.io")
	t.Setenv("WEBHOOK_SECRET", "webhook-shared-secret")
}

func TestLoad_WithAllRequiredVars(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	// Check defaults
	if cfg.ServiceName != "signer-service" {
		t.Errorf("ServiceName = %q, want %q", cfg.ServiceName, "signer-service")
	}
	if cfg.ServerPort != 8080 {
		t.Errorf("ServerPort = %d, want %d", cfg.ServerPort, 8080)
	}
	if cfg.MetricsPort != 9090 {
		t.Errorf("MetricsPort = %d, want %d", cfg.MetricsPort, 9090)
	}
	if cfg.WorkerCount != 5 {
		t.Errorf("WorkerCount = %d, want %d", cfg.WorkerCount, 5)
	}
	if cfg.QueueSize != 100 {
		t.Errorf("QueueSize = %d, want %d", cfg.QueueSize, 100)
	}
	if cfg.ReconcileInterval != 15*time.Minute {
		t.Errorf("ReconcileInterval = %v, want %v", cfg.ReconcileInterval, 15*time.Minute)
	}
	if cfg.ReconcileTimeout != 10*time.Minute {
		t.Errorf("ReconcileTimeout = %v, want %v", cfg.ReconcileTimeout, 10*time.Minute)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want %d", cfg.MaxRetries, 3)
	}
	if cfg.SeverityThreshold != "High" {
		t.Errorf("SeverityThreshold = %q, want %q", cfg.SeverityThreshold, "High")
	}
	if cfg.TokenPath != "/var/run/secrets/tokens/fulcio-token" {
		t.Errorf("TokenPath = %q, want %q", cfg.TokenPath, "/var/run/secrets/tokens/fulcio-token")
	}
	if cfg.MaxPayloadSize != 1048576 {
		t.Errorf("MaxPayloadSize = %d, want %d", cfg.MaxPayloadSize, 1048576)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
}

func TestLoad_OverrideDefaults(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SERVER_PORT", "9000")
	t.Setenv("METRICS_PORT", "9191")
	t.Setenv("WORKER_COUNT", "10")
	t.Setenv("QUEUE_SIZE", "200")
	t.Setenv("RECONCILE_INTERVAL", "30m")
	t.Setenv("RECONCILE_TIMEOUT", "20m")
	t.Setenv("MAX_RETRIES", "5")
	t.Setenv("SEVERITY_THRESHOLD", "Critical")
	t.Setenv("HARBOR_PROJECTS", "proj1,proj2,proj3")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.ServerPort != 9000 {
		t.Errorf("ServerPort = %d, want %d", cfg.ServerPort, 9000)
	}
	if cfg.MetricsPort != 9191 {
		t.Errorf("MetricsPort = %d, want %d", cfg.MetricsPort, 9191)
	}
	if cfg.WorkerCount != 10 {
		t.Errorf("WorkerCount = %d, want %d", cfg.WorkerCount, 10)
	}
	if cfg.QueueSize != 200 {
		t.Errorf("QueueSize = %d, want %d", cfg.QueueSize, 200)
	}
	if cfg.ReconcileInterval != 30*time.Minute {
		t.Errorf("ReconcileInterval = %v, want %v", cfg.ReconcileInterval, 30*time.Minute)
	}
	if cfg.ReconcileTimeout != 20*time.Minute {
		t.Errorf("ReconcileTimeout = %v, want %v", cfg.ReconcileTimeout, 20*time.Minute)
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want %d", cfg.MaxRetries, 5)
	}
	if cfg.SeverityThreshold != "Critical" {
		t.Errorf("SeverityThreshold = %q, want %q", cfg.SeverityThreshold, "Critical")
	}
	if len(cfg.HarborProjects) != 3 || cfg.HarborProjects[0] != "proj1" {
		t.Errorf("HarborProjects = %v, want [proj1 proj2 proj3]", cfg.HarborProjects)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
}

func TestValidate_MissingHarborURL(t *testing.T) {
	setRequiredEnv(t)
	os.Unsetenv("HARBOR_URL")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing HARBOR_URL")
	}
	if !containsString(err.Error(), "HARBOR_URL is required") {
		t.Errorf("error = %q, should mention HARBOR_URL", err.Error())
	}
}

func TestValidate_MissingHarborRobotUser(t *testing.T) {
	setRequiredEnv(t)
	os.Unsetenv("HARBOR_ROBOT_USER")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing HARBOR_ROBOT_USER")
	}
	if !containsString(err.Error(), "HARBOR_ROBOT_USER is required") {
		t.Errorf("error = %q, should mention HARBOR_ROBOT_USER", err.Error())
	}
}

func TestValidate_MissingHarborRobotPassword(t *testing.T) {
	setRequiredEnv(t)
	os.Unsetenv("HARBOR_ROBOT_PASSWORD")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing HARBOR_ROBOT_PASSWORD")
	}
	if !containsString(err.Error(), "HARBOR_ROBOT_PASSWORD is required") {
		t.Errorf("error = %q, should mention HARBOR_ROBOT_PASSWORD", err.Error())
	}
}

func TestValidate_MissingFulcioURL(t *testing.T) {
	setRequiredEnv(t)
	os.Unsetenv("FULCIO_URL")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing FULCIO_URL")
	}
	if !containsString(err.Error(), "FULCIO_URL is required") {
		t.Errorf("error = %q, should mention FULCIO_URL", err.Error())
	}
}

func TestValidate_MissingRekorURL(t *testing.T) {
	setRequiredEnv(t)
	os.Unsetenv("REKOR_URL")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing REKOR_URL")
	}
	if !containsString(err.Error(), "REKOR_URL is required") {
		t.Errorf("error = %q, should mention REKOR_URL", err.Error())
	}
}

func TestValidate_MissingWebhookSecret(t *testing.T) {
	setRequiredEnv(t)
	os.Unsetenv("WEBHOOK_SECRET")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing WEBHOOK_SECRET")
	}
	if !containsString(err.Error(), "WEBHOOK_SECRET is required") {
		t.Errorf("error = %q, should mention WEBHOOK_SECRET", err.Error())
	}
}

func TestValidate_MultipleRequiredVarsMissing(t *testing.T) {
	// No required env vars set at all
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing required vars")
	}
	errStr := err.Error()
	if !containsString(errStr, "HARBOR_URL is required") {
		t.Errorf("error should mention HARBOR_URL")
	}
	if !containsString(errStr, "WEBHOOK_SECRET is required") {
		t.Errorf("error should mention WEBHOOK_SECRET")
	}
}

func TestValidate_InvalidSeverityThreshold(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SEVERITY_THRESHOLD", "Invalid")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid SEVERITY_THRESHOLD")
	}
	if !containsString(err.Error(), "SEVERITY_THRESHOLD") {
		t.Errorf("error = %q, should mention SEVERITY_THRESHOLD", err.Error())
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("LOG_LEVEL", "verbose")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid LOG_LEVEL")
	}
	if !containsString(err.Error(), "LOG_LEVEL") {
		t.Errorf("error = %q, should mention LOG_LEVEL", err.Error())
	}
}

func TestGetEnvAsStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     []string
	}{
		{"comma separated", "a,b,c", []string{"a", "b", "c"}},
		{"with spaces", " a , b , c ", []string{"a", "b", "c"}},
		{"single value", "single", []string{"single"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TEST_SLICE", tt.envValue)
			got := getEnvAsStringSlice("TEST_SLICE", nil)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGetEnvAsDuration(t *testing.T) {
	t.Setenv("TEST_DUR", "5m")
	got := getEnvAsDuration("TEST_DUR", time.Hour)
	if got != 5*time.Minute {
		t.Errorf("got %v, want %v", got, 5*time.Minute)
	}

	// Invalid duration falls back to default
	t.Setenv("TEST_DUR_BAD", "notaduration")
	got = getEnvAsDuration("TEST_DUR_BAD", 30*time.Second)
	if got != 30*time.Second {
		t.Errorf("got %v, want %v", got, 30*time.Second)
	}
}

func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || len(haystack) > 0 && containsSubstr(haystack, needle))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
