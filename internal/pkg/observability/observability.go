// Package observability provides OpenTelemetry tracing initialization for the signer-service.
// It configures an OTLP/HTTP exporter targeting the platform collector. If the collector
// is unreachable, trace exports are dropped without blocking the signing pipeline.
package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
)

// InitTraceProvider initializes the OpenTelemetry trace provider with an OTLP/HTTP exporter.
// It returns a shutdown function that should be called on application exit.
// If the collector is unreachable, exports are dropped without blocking.
func InitTraceProvider(ctx context.Context, serviceName, serviceVersion, endpoint string) (func(context.Context) error, error) {
	// Create OTLP/HTTP exporter targeting the provided endpoint.
	// Using WithInsecure since the in-cluster collector uses HTTP (not HTTPS).
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	// Build a resource describing this service.
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return nil, err
	}

	// Configure a batch span processor. The batch processor is non-blocking:
	// if the collector is down, spans are dropped after the export timeout
	// without blocking the application.
	bsp := sdktrace.NewBatchSpanProcessor(exporter)

	// Set up the TracerProvider with the exporter, resource, and sampler.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(bsp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Register as the global trace provider.
	otel.SetTracerProvider(tp)

	// Return a shutdown function that flushes pending spans and shuts down the provider.
	return tp.Shutdown, nil
}
