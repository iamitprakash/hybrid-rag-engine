package observability

import (
	"context"
	"io"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func InitTracing(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	exporterName := os.Getenv("OTEL_TRACES_EXPORTER")
	if exporterName == "" || exporterName == "none" {
		return func(context.Context) error { return nil }, nil
	}

	var exporter sdktrace.SpanExporter
	switch exporterName {
	case "stdout":
		var writer io.Writer = os.Stdout
		stdoutExporter, err := stdouttrace.New(stdouttrace.WithWriter(writer), stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, err
		}
		exporter = stdoutExporter
	case "otlp":
		otlpExporter, err := otlptracehttp.New(ctx)
		if err != nil {
			return nil, err
		}
		exporter = otlpExporter
	default:
		return func(context.Context) error { return nil }, nil
	}
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return provider.Shutdown, nil
}
