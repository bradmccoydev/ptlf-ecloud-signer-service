package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration for the signer-service.
type Config struct {
	// Server configuration
	ServiceName    string
	ServiceVersion string
	Environment    string
	ServerPort     int
	MetricsPort    int
	LogLevel       string

	// Harbor
	HarborURL      string
	HarborRobot    string
	HarborPassword string

	// Registry
	RegistryURL string

	// Sigstore
	FulcioURL     string
	RekorURL      string
	TokenPath     string
	TokenAudience string

	// Webhook
	WebhookSecret  string
	MaxPayloadSize int64

	// Worker pool
	WorkerCount int
	QueueSize   int

	// Reconciliation
	ReconcileInterval time.Duration
	ReconcileTimeout  time.Duration
	HarborProjects    []string

	// Retry
	MaxRetries     int
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration

	// Severity threshold
	SeverityThreshold string

	// OTel
	OTelEndpoint string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		ServiceName:    getEnv("SERVICE_NAME", "signer-service"),
		ServiceVersion: getEnv("SERVICE_VERSION", "1.0.0"),
		Environment:    getEnv("ENVIRONMENT", "production"),
		ServerPort:     getEnvAsInt("SERVER_PORT", 8080),
		MetricsPort:    getEnvAsInt("METRICS_PORT", 9090),
		LogLevel:       getEnv("LOG_LEVEL", "info"),

		// Harbor
		HarborURL:      getEnv("HARBOR_URL", ""),
		HarborRobot:    getEnv("HARBOR_ROBOT_USER", ""),
		HarborPassword: getEnv("HARBOR_ROBOT_PASSWORD", ""),

		// Registry
		RegistryURL: getEnv("REGISTRY_URL", "registry.platform.cuscal.io"),

		// Sigstore
		FulcioURL:     getEnv("FULCIO_URL", ""),
		RekorURL:      getEnv("REKOR_URL", ""),
		TokenPath:     getEnv("TOKEN_PATH", "/var/run/secrets/tokens/fulcio-token"),
		TokenAudience: getEnv("TOKEN_AUDIENCE", "sigstore"),

		// Webhook
		WebhookSecret:  getEnv("WEBHOOK_SECRET", ""),
		MaxPayloadSize: getEnvAsInt64("MAX_PAYLOAD_SIZE", 1048576), // 1MB

		// Worker pool
		WorkerCount: getEnvAsInt("WORKER_COUNT", 5),
		QueueSize:   getEnvAsInt("QUEUE_SIZE", 100),

		// Reconciliation
		ReconcileInterval: getEnvAsDuration("RECONCILE_INTERVAL", 15*time.Minute),
		ReconcileTimeout:  getEnvAsDuration("RECONCILE_TIMEOUT", 10*time.Minute),
		HarborProjects:    getEnvAsStringSlice("HARBOR_PROJECTS", []string{"chainguard", "charts", "platform", "applications", "library"}),

		// Retry
		MaxRetries:     getEnvAsInt("MAX_RETRIES", 3),
		RetryBaseDelay: getEnvAsDuration("RETRY_BASE_DELAY", 1*time.Second),
		RetryMaxDelay:  getEnvAsDuration("RETRY_MAX_DELAY", 10*time.Second),

		// Severity threshold
		SeverityThreshold: getEnv("SEVERITY_THRESHOLD", "High"),

		// OTel
		OTelEndpoint: getEnv("OTEL_EXPORTER_ENDPOINT", "http://opentelemetry-collector.observability.svc.cluster.local:4318"),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks that all required configuration is present and valid.
func (c *Config) Validate() error {
	var errors []string

	// Required fields — these come from ExternalSecrets or operator-provided config
	if c.HarborURL == "" {
		errors = append(errors, "HARBOR_URL is required")
	}
	if c.HarborRobot == "" {
		errors = append(errors, "HARBOR_ROBOT_USER is required")
	}
	if c.HarborPassword == "" {
		errors = append(errors, "HARBOR_ROBOT_PASSWORD is required")
	}
	if c.FulcioURL == "" {
		errors = append(errors, "FULCIO_URL is required")
	}
	if c.RekorURL == "" {
		errors = append(errors, "REKOR_URL is required")
	}
	// WEBHOOK_SECRET sourced from ExternalSecret harbor-signer-webhook
	if c.WebhookSecret == "" {
		errors = append(errors, "WEBHOOK_SECRET is required (ExternalSecret harbor-signer-webhook must be available)")
	}

	// Validate severity threshold
	validSeverities := map[string]bool{
		"None": true, "Low": true, "Medium": true, "High": true, "Critical": true,
	}
	if !validSeverities[c.SeverityThreshold] {
		errors = append(errors, fmt.Sprintf("SEVERITY_THRESHOLD must be one of: None, Low, Medium, High, Critical; got %q", c.SeverityThreshold))
	}

	// Validate numeric ranges
	if c.ServerPort < 1 || c.ServerPort > 65535 {
		errors = append(errors, "SERVER_PORT must be between 1 and 65535")
	}
	if c.MetricsPort < 1 || c.MetricsPort > 65535 {
		errors = append(errors, "METRICS_PORT must be between 1 and 65535")
	}
	if c.WorkerCount < 1 {
		errors = append(errors, "WORKER_COUNT must be at least 1")
	}
	if c.QueueSize < 1 {
		errors = append(errors, "QUEUE_SIZE must be at least 1")
	}
	if c.MaxRetries < 0 {
		errors = append(errors, "MAX_RETRIES must be non-negative")
	}
	if c.MaxPayloadSize < 1 {
		errors = append(errors, "MAX_PAYLOAD_SIZE must be at least 1")
	}

	// Validate durations
	if c.ReconcileInterval <= 0 {
		errors = append(errors, "RECONCILE_INTERVAL must be positive")
	}
	if c.ReconcileTimeout <= 0 {
		errors = append(errors, "RECONCILE_TIMEOUT must be positive")
	}
	if c.RetryBaseDelay < 0 {
		errors = append(errors, "RETRY_BASE_DELAY must be non-negative")
	}
	if c.RetryMaxDelay < 0 {
		errors = append(errors, "RETRY_MAX_DELAY must be non-negative")
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLogLevels[c.LogLevel] {
		errors = append(errors, fmt.Sprintf("LOG_LEVEL must be one of: debug, info, warn, error; got %q", c.LogLevel))
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
}

// Helper functions to read environment variables

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsInt64(key string, defaultValue int64) int64 {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseInt(valueStr, 10, 64)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := time.ParseDuration(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsStringSlice(key string, defaultValue []string) []string {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	parts := strings.Split(valueStr, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return defaultValue
	}
	return result
}
