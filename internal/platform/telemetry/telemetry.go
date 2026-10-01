// Package telemetry adds transport metadata without changing financial payloads.
package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"

	"github.com/alexandre/wagering/internal/infra/config"
)

var (
	enabled    atomic.Bool
	propagator = propagation.TraceContext{} // Deliberately does not propagate baggage.
)

func Enabled() bool { return enabled.Load() }

// Install must precede dependency hooks, so export drains after application shutdown.
func Install(lc fx.Lifecycle, cfg config.Config, log *slog.Logger) {
	var provider *sdktrace.TracerProvider
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if cfg.OTelDisabled {
				return nil
			}
			exporter, err := otlptracehttp.New(ctx,
				otlptracehttp.WithEndpointURL(cfg.OTelTracesEndpoint),
				otlptracehttp.WithTimeout(2*time.Second),
				otlptracehttp.WithRetry(otlptracehttp.RetryConfig{Enabled: false}),
			)
			if err != nil {
				return err
			}
			provider = sdktrace.NewTracerProvider(
				sdktrace.WithResource(resource.NewSchemaless(
					attribute.String("service.name", cfg.OTelServiceName),
					attribute.String("service.instance.id", uuid.NewString()),
				)),
				sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.OTelSampleRatio))),
				sdktrace.WithBatcher(exporter, sdktrace.WithMaxQueueSize(2048),
					sdktrace.WithMaxExportBatchSize(256), sdktrace.WithBatchTimeout(time.Second),
					sdktrace.WithExportTimeout(2*time.Second)),
			)
			// Do not log exporter errors: they may contain endpoint credentials or response bodies.
			otel.SetErrorHandler(otel.ErrorHandlerFunc(func(error) { log.Warn("trace export failed; telemetry may be dropped") }))
			otel.SetTracerProvider(provider)
			otel.SetTextMapPropagator(propagator)
			enabled.Store(true)
			log.Info("tracing enabled", "service", cfg.OTelServiceName, "sampleRatio", cfg.OTelSampleRatio)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if provider == nil {
				return nil
			}
			defer enabled.Store(false)
			ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			if err := provider.Shutdown(ctx); err != nil {
				log.Warn("trace shutdown incomplete; telemetry may be dropped")
			}
			return nil // Observability is not a financial dependency.
		},
	})
}

// Start hides SDK types from application ports. Error details are deliberately redacted.
func Start(ctx context.Context, name string) (context.Context, func(error)) {
	return start(ctx, name, trace.SpanKindInternal)
}

func Producer(ctx context.Context, name string) (context.Context, func(error)) {
	return start(ctx, name, trace.SpanKindProducer)
}

func Consumer(ctx context.Context, name string) (context.Context, func(error)) {
	return start(ctx, name, trace.SpanKindConsumer)
}

func start(ctx context.Context, name string, kind trace.SpanKind) (context.Context, func(error)) {
	if !Enabled() {
		return ctx, func(error) {}
	}
	ctx, span := otel.Tracer("wagering").Start(ctx, name, trace.WithSpanKind(kind))
	return ctx, func(err error) {
		if err != nil {
			category := "operation_failed"
			if errors.Is(err, context.Canceled) {
				category = "canceled"
			}
			if errors.Is(err, context.DeadlineExceeded) {
				category = "timeout"
			}
			span.SetStatus(codes.Error, category)
			span.SetAttributes(attribute.String("error.type", category))
		}
		span.End()
	}
}

func Attribute(ctx context.Context, key, value string) {
	trace.SpanFromContext(ctx).SetAttributes(attribute.String(key, value))
}

func Capture(ctx context.Context) map[string]string {
	carrier := propagation.MapCarrier{}
	if Enabled() {
		propagator.Inject(ctx, carrier)
	}
	return carrier
}

func Extract(ctx context.Context, carrier map[string]string) context.Context {
	if !Enabled() {
		return ctx
	}
	return propagator.Extract(ctx, propagation.MapCarrier(carrier))
}

// Resume uses the persisted origin as parent and links the current polling span.
// This avoids making a long-lived operation depend on an in-memory goroutine.
func Resume(ctx context.Context, name string, carrier map[string]string) (context.Context, func(error)) {
	if !Enabled() {
		return ctx, func(error) {}
	}
	caller := trace.SpanContextFromContext(ctx)
	parent := Extract(context.Background(), carrier)
	origin := trace.SpanContextFromContext(parent)
	opts := []trace.SpanStartOption{}
	if origin.IsValid() {
		ctx = trace.ContextWithRemoteSpanContext(ctx, origin)
		if caller.IsValid() {
			opts = append(opts, trace.WithLinks(trace.Link{SpanContext: caller}))
		}
	}
	ctx, span := otel.Tracer("wagering").Start(ctx, name, opts...)
	return ctx, func(err error) {
		if err != nil {
			span.SetStatus(codes.Error, "operation_failed")
		}
		span.End()
	}
}

// LogHandler adds identifiers only to context-aware log records, preserving JSON fields.
type LogHandler struct{ slog.Handler }

func (h LogHandler) Handle(ctx context.Context, r slog.Record) error {
	sc := trace.SpanContextFromContext(ctx)
	if Enabled() && sc.IsValid() {
		r.AddAttrs(slog.String("trace_id", sc.TraceID().String()), slog.String("span_id", sc.SpanID().String()))
	}
	return h.Handler.Handle(ctx, r)
}

func (h LogHandler) WithAttrs(a []slog.Attr) slog.Handler { return LogHandler{h.Handler.WithAttrs(a)} }

func (h LogHandler) WithGroup(g string) slog.Handler { return LogHandler{h.Handler.WithGroup(g)} }
