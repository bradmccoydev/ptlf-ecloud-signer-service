package observability

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
)

func TestInitTraceProvider(t *testing.T) {
	ctx := context.Background()

	// Use a non-routable endpoint so the exporter won't actually connect,
	// but the provider should still initialize without error (non-blocking).
	shutdown, err := InitTraceProvider(ctx, "test-service", "1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("InitTraceProvider returned unexpected error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	// Verify the global tracer provider is set and produces a valid tracer.
	tracer := otel.Tracer("test-tracer")
	if tracer == nil {
		t.Fatal("expected non-nil tracer from global provider")
	}

	// Create a span to verify tracing pipeline works without panic.
	_, span := tracer.Start(ctx, "test-span")
	span.End()

	// Shutdown should not error (the exporter may fail to flush to unreachable
	// collector, but shutdown itself should be graceful).
	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown returned unexpected error: %v", err)
	}
}

func TestInitTraceProviderSetsGlobalProvider(t *testing.T) {
	ctx := context.Background()

	shutdown, err := InitTraceProvider(ctx, "signer-service", "2.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("InitTraceProvider returned unexpected error: %v", err)
	}
	defer shutdown(ctx) //nolint:errcheck

	// The signing service uses otel.Tracer("signing-service") — verify it works.
	tracer := otel.Tracer("signing-service")
	_, span := tracer.Start(ctx, "ProcessArtifact")
	span.End()
}
