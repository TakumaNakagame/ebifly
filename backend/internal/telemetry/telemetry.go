// Package telemetry wires OpenTelemetry tracing for ebifly.
//
// Tracing is opt-in: a TracerProvider is only installed when an OTLP endpoint
// is configured through the standard environment variables
// (OTEL_EXPORTER_OTLP_ENDPOINT or OTEL_EXPORTER_OTLP_TRACES_ENDPOINT). Without
// them the global no-op provider stays in place and the instrumentation in the
// rest of the code costs next to nothing.
//
// Everything else follows the OpenTelemetry environment conventions:
// OTEL_SERVICE_NAME overrides the service name, OTEL_RESOURCE_ATTRIBUTES adds
// resource attributes, OTEL_TRACES_SAMPLER / OTEL_TRACES_SAMPLER_ARG pick the
// sampler (default parentbased_always_on), and OTEL_SDK_DISABLED=true turns it
// off even when an endpoint is set. The exporter is OTLP/HTTP (protobuf).
package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// ScopeName is the instrumentation scope used for spans created by ebifly
// itself (as opposed to otelhttp / otelsql).
const ScopeName = "github.com/takuma/planning-poker/backend"

// Tracer returns ebifly's own tracer from the global provider.
func Tracer() trace.Tracer { return otel.Tracer(ScopeName) }

// Enabled reports whether an OTLP endpoint is configured in the environment.
func Enabled() bool {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("OTEL_SDK_DISABLED")), "true") {
		return false
	}
	return os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" ||
		os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") != ""
}

// Setup installs the global TracerProvider and propagator when tracing is
// enabled. The returned shutdown function flushes buffered spans; it is always
// non-nil and safe to call when tracing is disabled.
func Setup(ctx context.Context, defaultServiceName, version string) (func(context.Context) error, error) {
	// Propagator is installed unconditionally: it is cheap and keeps incoming
	// traceparent headers flowing even if this process does not export.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	noop := func(context.Context) error { return nil }
	if !Enabled() {
		return noop, nil
	}

	exp, err := otlptracehttp.New(ctx) // endpoint, headers, timeout etc. come from OTEL_EXPORTER_OTLP_*
	if err != nil {
		return noop, err
	}

	attrs := []attribute.KeyValue{attribute.String("service.name", defaultServiceName)}
	if version != "" {
		attrs = append(attrs, attribute.String("service.version", version))
	}
	// Later sources win: OTEL_SERVICE_NAME / OTEL_RESOURCE_ATTRIBUTES (WithFromEnv)
	// override the default service name above.
	res, err := resource.New(ctx,
		resource.WithAttributes(attrs...),
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithProcessRuntimeName(),
		resource.WithProcessRuntimeVersion(),
	)
	if err != nil && !errors.Is(err, resource.ErrPartialResource) {
		return noop, err
	}

	// No WithSampler: the SDK reads OTEL_TRACES_SAMPLER itself and defaults
	// to parentbased_always_on.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		slog.Warn("otel", "err", err)
	}))
	slog.Info("tracing enabled", "service", serviceName(res, defaultServiceName))
	return tp.Shutdown, nil
}

func serviceName(res *resource.Resource, def string) string {
	if res != nil {
		if v, ok := res.Set().Value("service.name"); ok {
			return v.AsString()
		}
	}
	return def
}

// HTTPHandler wraps h with otelhttp. Only requests for which keep returns true
// get a span; everything else (static files, health checks, long-lived
// WebSocket upgrades) passes through untouched. Span names start as the HTTP
// method and are replaced with "METHOD /route/{pattern}" by ChiRouteName.
func HTTPHandler(h http.Handler, keep func(*http.Request) bool) http.Handler {
	return otelhttp.NewHandler(h, "http",
		otelhttp.WithFilter(keep),
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method
		}),
	)
}

// ChiRouteName is a chi middleware that, once the request has been routed,
// renames the current server span to "METHOD <route pattern>" and sets
// http.route. Using the pattern (e.g. /api/rooms/{code}) instead of the raw
// path keeps span names low-cardinality and free of room codes.
func ChiRouteName(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		span := trace.SpanFromContext(r.Context())
		if !span.IsRecording() {
			return
		}
		rctx := chi.RouteContext(r.Context())
		if rctx == nil {
			return
		}
		if pattern := rctx.RoutePattern(); pattern != "" {
			span.SetName(r.Method + " " + pattern)
			span.SetAttributes(attribute.String("http.route", pattern))
		}
	})
}

// TraceID returns the trace id of the span in ctx, or "" when there is none.
// Handy for correlating log lines with traces.
func TraceID(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return ""
	}
	return sc.TraceID().String()
}
