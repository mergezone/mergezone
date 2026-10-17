package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "mergezone"

// Setup configures an OTLP/HTTP trace exporter when OTEL_EXPORTER_OTLP_ENDPOINT
// is set. With no endpoint, tracing uses OpenTelemetry's no-op provider.
func Setup(ctx context.Context, logger *slog.Logger) (func(context.Context) error, error) {
	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if endpoint == "" {
		otel.SetTracerProvider(trace.NewNoopTracerProvider())
		otel.SetTextMapPropagator(propagation.TraceContext{})
		return func(context.Context) error { return nil }, nil
	}

	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("invalid OTEL_EXPORTER_OTLP_ENDPOINT %q", endpoint)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT must use http or https")
	}
	options := []otlptracehttp.Option{otlptracehttp.WithEndpoint(parsed.Host)}
	if parsed.Scheme == "http" {
		options = append(options, otlptracehttp.WithInsecure())
	}
	path := strings.TrimRight(parsed.Path, "/")
	if path != "" {
		if strings.HasSuffix(path, "/v1/traces") {
			path = strings.TrimSuffix(path, "/v1/traces")
		}
		if path != "" {
			options = append(options, otlptracehttp.WithURLPath(path+"/v1/traces"))
		}
	}

	exporter, err := otlptracehttp.New(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}
	resourceAttributes := []attribute.KeyValue{semconv.ServiceName(serviceName)}
	if environment := strings.TrimSpace(os.Getenv("OTEL_ENVIRONMENT")); environment != "" {
		resourceAttributes = append(resourceAttributes, attribute.String("deployment.environment", environment))
	}
	res, err := resource.New(ctx, resource.WithAttributes(resourceAttributes...))
	if err != nil {
		return nil, fmt.Errorf("create OpenTelemetry resource: %w", err)
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	logger.Info("OpenTelemetry tracing enabled", "scheme", parsed.Scheme, "host", parsed.Host)
	return provider.Shutdown, nil
}

func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

func RecordError(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}
