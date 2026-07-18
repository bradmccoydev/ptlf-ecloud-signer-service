package logger

import (
	"testing"
)

func TestNew_ValidLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error"}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			l, err := New("signer-service", "1.0.0", "production", level)
			if err != nil {
				t.Fatalf("expected no error for level %q, got: %v", level, err)
			}
			if l == nil {
				t.Fatal("expected non-nil logger")
			}
			l.Sync()
		})
	}
}

func TestNew_InvalidLevel(t *testing.T) {
	_, err := New("signer-service", "1.0.0", "production", "invalid")
	if err == nil {
		t.Fatal("expected error for invalid level, got nil")
	}
}

func TestNew_EmptyLevel(t *testing.T) {
	_, err := New("signer-service", "1.0.0", "production", "")
	if err == nil {
		t.Fatal("expected error for empty level, got nil")
	}
}
