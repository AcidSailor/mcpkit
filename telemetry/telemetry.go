package telemetry

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

// scope is the instrumentation scope name of the tracer.
const scope = "github.com/acidsailor/mcpkit/telemetry"

// toolError is the error.type of a tool result with IsError set.
const toolError = "tool_error"

// MCP semantic convention keys. Semconv v1.42.0 moved the gen_ai and mcp
// conventions to the GenAI semantic conventions repository, which has no Go
// package, so they are declared here.
const (
	keyMethodName    = attribute.Key("mcp.method.name")
	keySessionID     = attribute.Key("mcp.session.id")
	keyResourceURI   = attribute.Key("mcp.resource.uri")
	keyToolName      = attribute.Key("gen_ai.tool.name")
	keyPromptName    = attribute.Key("gen_ai.prompt.name")
	keyOperationName = attribute.Key("gen_ai.operation.name")
)

type config struct {
	propagator propagation.TextMapPropagator
}

// Option configures Middleware.
type Option func(*config)

// WithPropagator replaces the W3C TraceContext propagator that reads the
// parent span from params._meta.
func WithPropagator(p propagation.TextMapPropagator) Option {
	return func(c *config) { c.propagator = p }
}

// Middleware returns receiving middleware that traces each MCP request with
// tracers from tp. It panics with ErrNilTracerProvider when tp is nil.
func Middleware(tp trace.TracerProvider, opts ...Option) mcp.Middleware {
	if tp == nil {
		panic(ErrNilTracerProvider)
	}
	c := config{propagator: propagation.TraceContext{}}
	for _, o := range opts {
		o(&c)
	}
	tracer := tp.Tracer(scope)

	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(
			ctx context.Context,
			method string,
			req mcp.Request,
		) (mcp.Result, error) {
			if strings.HasPrefix(method, "notifications/") {
				return next(ctx, method, req)
			}

			name, attrs := describe(method, req)
			opts := []trace.SpanStartOption{
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(attrs...),
			}
			ambient := trace.SpanContextFromContext(ctx)
			ctx = c.propagator.Extract(ctx, metaCarrier(meta(req)))
			// The _meta parent replaces the ambient one, such as an HTTP
			// server span; keep the ambient span as a link.
			if ambient.IsValid() &&
				!ambient.Equal(trace.SpanContextFromContext(ctx)) {
				opts = append(opts, trace.WithLinks(
					trace.Link{SpanContext: ambient},
				))
			}
			ctx, span := tracer.Start(ctx, name, opts...)
			defer span.End()

			res, err := next(ctx, method, req)
			record(span, res, err)
			return res, err
		}
	}
}

// describe returns the span name and attributes for a request.
func describe(
	method string,
	req mcp.Request,
) (string, []attribute.KeyValue) {
	name := method
	attrs := []attribute.KeyValue{keyMethodName.String(method)}

	switch p := req.GetParams().(type) {
	case *mcp.CallToolParamsRaw:
		if p != nil {
			name += " " + p.Name
			attrs = append(
				attrs,
				keyToolName.String(p.Name),
				keyOperationName.String("execute_tool"),
			)
		}
	case *mcp.GetPromptParams:
		if p != nil {
			name += " " + p.Name
			attrs = append(attrs, keyPromptName.String(p.Name))
		}
	case *mcp.ReadResourceParams:
		if p != nil {
			attrs = append(attrs, keyResourceURI.String(p.URI))
		}
	}

	if s := req.GetSession(); !isNil(s) && s.ID() != "" {
		attrs = append(attrs, keySessionID.String(s.ID()))
	}
	return name, attrs
}

// record marks the span failed for a call error or a tool error result. A
// JSON-RPC error is classified by its code.
func record(span trace.Span, res mcp.Result, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if rpcErr, ok := errors.AsType[*jsonrpc.Error](err); ok {
			code := strconv.FormatInt(rpcErr.Code, 10)
			span.SetAttributes(
				semconv.ErrorTypeKey.String(code),
				semconv.RPCResponseStatusCode(code),
			)
			return
		}
		span.SetAttributes(semconv.ErrorType(err))
		return
	}
	if r, ok := res.(*mcp.CallToolResult); ok && r != nil && r.IsError {
		span.SetStatus(codes.Error, toolError)
		span.SetAttributes(semconv.ErrorTypeKey.String(toolError))
	}
}

// meta returns the request's _meta, or nil when it has no params.
func meta(req mcp.Request) map[string]any {
	p := req.GetParams()
	if isNil(p) {
		return nil
	}
	return p.GetMeta()
}

// isNil reports a nil interface or a typed nil pointer. The SDK can pass
// typed nil params and sessions, whose methods would panic.
func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Pointer && rv.IsNil()
}

// metaCarrier adapts _meta to a propagation.TextMapCarrier. Only string
// values are read.
type metaCarrier map[string]any

func (m metaCarrier) Get(key string) string {
	s, _ := m[key].(string)
	return s
}

func (m metaCarrier) Set(key, value string) {
	if m != nil {
		m[key] = value
	}
}

func (m metaCarrier) Keys() []string {
	return slices.Collect(maps.Keys(m))
}
