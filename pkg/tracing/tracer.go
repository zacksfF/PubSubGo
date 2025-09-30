package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

const (
	// Service name
	ServiceName = "pubsubgo"

	// Operation names
	OpPublishMessage = "publish_message"
	OpConsumeMessage = "consume_message"
	OpCreateTopic    = "create_topic"
	OpDeleteTopic    = "delete_topic"
	OpSubscribe      = "subscribe"
	OpUnsubscribe    = "unsubscribe"
	OpAcknowledge    = "acknowledge_message"
	OpRedisOperation = "redis_operation"
	OpHTTPRequest    = "http_request"
	OpWebSocketConn  = "websocket_connection"
)

// TracerConfig holds the configuration for tracing
type TracerConfig struct {
	Enabled      bool
	ServiceName  string
	OTLPEndpoint string
	SamplingRate float64
}

// Tracer wraps OpenTelemetry functionality
type Tracer struct {
	tracer   trace.Tracer
	provider *sdktrace.TracerProvider
	enabled  bool
}

// New creates a new tracer instance
func New(config TracerConfig) (*Tracer, error) {
	if !config.Enabled {
		return &Tracer{
			tracer:  noop.NewTracerProvider().Tracer(config.ServiceName),
			enabled: false,
		}, nil
	}

	// Create OTLP HTTP exporter
	exp, err := otlptracehttp.New(context.Background(),
		otlptracehttp.WithEndpoint(config.OTLPEndpoint),
		otlptracehttp.WithInsecure(), // Use insecure for local development
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(config.SamplingRate)),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Tracer{
		tracer:   tp.Tracer(config.ServiceName),
		provider: tp,
		enabled:  true,
	}, nil
}

// StartSpan starts a new span with the given operation name
func (t *Tracer) StartSpan(ctx context.Context, operationName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if !t.enabled {
		return ctx, trace.SpanFromContext(ctx)
	}
	return t.tracer.Start(ctx, operationName, opts...)
}

// StartMessageSpan starts a span for message operations
func (t *Tracer) StartMessageSpan(ctx context.Context, operationName, topic, messageID string) (context.Context, trace.Span) {
	ctx, span := t.StartSpan(ctx, operationName)
	if t.enabled {
		span.SetAttributes(
			attribute.String("messaging.system", "pubsubgo"),
			attribute.String("messaging.destination", topic),
			attribute.String("messaging.message_id", messageID),
			attribute.String("messaging.operation", operationName),
		)
	}
	return ctx, span
}

// StartHTTPSpan starts a span for HTTP operations
func (t *Tracer) StartHTTPSpan(ctx context.Context, method, path string) (context.Context, trace.Span) {
	ctx, span := t.StartSpan(ctx, OpHTTPRequest)
	if t.enabled {
		span.SetAttributes(
			attribute.String("http.method", method),
			attribute.String("http.route", path),
			attribute.String("component", "http"),
		)
	}
	return ctx, span
}

// StartRedisSpan starts a span for Redis operations
func (t *Tracer) StartRedisSpan(ctx context.Context, operation, key string) (context.Context, trace.Span) {
	ctx, span := t.StartSpan(ctx, OpRedisOperation)
	if t.enabled {
		span.SetAttributes(
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", operation),
			attribute.String("db.redis.key", key),
			attribute.String("component", "redis"),
		)
	}
	return ctx, span
}

// AddEventToSpan adds an event to the current span
func (t *Tracer) AddEventToSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	if !t.enabled {
		return
	}
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

// SetSpanError marks the span as error and records the error
func (t *Tracer) SetSpanError(ctx context.Context, err error) {
	if !t.enabled || err == nil {
		return
	}
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("error", true))
	}
}

// SetSpanAttributes sets attributes on the current span
func (t *Tracer) SetSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	if !t.enabled {
		return
	}
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(attrs...)
	}
}

// Shutdown gracefully shuts down the tracer
func (t *Tracer) Shutdown(ctx context.Context) error {
	if !t.enabled || t.provider == nil {
		return nil
	}
	return t.provider.Shutdown(ctx)
}

// ExtractTraceContext extracts trace context from headers
func (t *Tracer) ExtractTraceContext(ctx context.Context, headers map[string]string) context.Context {
	if !t.enabled {
		return ctx
	}

	carrier := propagation.MapCarrier(headers)
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

// InjectTraceContext injects trace context into headers
func (t *Tracer) InjectTraceContext(ctx context.Context, headers map[string]string) {
	if !t.enabled {
		return
	}

	carrier := propagation.MapCarrier(headers)
	otel.GetTextMapPropagator().Inject(ctx, carrier)
}

// GetTraceID returns the trace ID from the current span
func (t *Tracer) GetTraceID(ctx context.Context) string {
	if !t.enabled {
		return ""
	}

	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasTraceID() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// GetSpanID returns the span ID from the current span
func (t *Tracer) GetSpanID(ctx context.Context) string {
	if !t.enabled {
		return ""
	}

	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasSpanID() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}
