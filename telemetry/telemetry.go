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
// conventions to the GenAI semantic conventions repository. Later Go semconv
// packages omit them, and v1.41.0 is a frozen snapshot, so they are declared
// here.
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
	kind := trace.WithSpanKind(trace.SpanKindServer)

	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(
			ctx context.Context,
			method string,
			req mcp.Request,
		) (mcp.Result, error) {
			if strings.HasPrefix(method, "notifications/") {
				return next(ctx, method, req)
			}

			p := params(req)
			name, attrs := describe(method, p, req.GetSession())
			start := make([]trace.SpanStartOption, 0, 3)
			start = append(start, kind, trace.WithAttributes(attrs...))
			ambient := trace.SpanContextFromContext(ctx)
			var md metaCarrier
			if p != nil {
				md = p.GetMeta()
			}
			ctx = c.propagator.Extract(ctx, md)
			// The _meta parent replaces the ambient one, such as an HTTP
			// server span; keep the ambient span as a link.
			if ambient.IsValid() &&
				!ambient.Equal(trace.SpanContextFromContext(ctx)) {
				start = append(start, trace.WithLinks(
					trace.Link{SpanContext: ambient},
				))
			}
			ctx, span := tracer.Start(ctx, name, start...)
			defer span.End()

			res, err := next(ctx, method, req)
			record(span, res, err)
			return res, err
		}
	}
}

// describe returns the span name and attributes for a request. Nil params
// match no case.
func describe(
	method string,
	p mcp.Params,
	s mcp.Session,
) (string, []attribute.KeyValue) {
	name := method
	// Method, tool, operation, and session are the most set at once.
	attrs := make([]attribute.KeyValue, 0, 4)
	attrs = append(attrs, keyMethodName.String(method))

	switch p := p.(type) {
	case *mcp.CallToolParamsRaw:
		name = target(name, p.Name)
		attrs = append(
			attrs,
			keyToolName.String(p.Name),
			keyOperationName.String("execute_tool"),
		)
	case *mcp.GetPromptParams:
		name = target(name, p.Name)
		attrs = append(attrs, keyPromptName.String(p.Name))
	case *mcp.ReadResourceParams:
		attrs = append(attrs, keyResourceURI.String(p.URI))
	case *mcp.SubscribeParams:
		attrs = append(attrs, keyResourceURI.String(p.URI))
	case *mcp.UnsubscribeParams:
		attrs = append(attrs, keyResourceURI.String(p.URI))
	}

	if id := s.ID(); id != "" {
		attrs = append(attrs, keySessionID.String(id))
	}
	return name, attrs
}

// target appends a non-empty tool or prompt name to the span name.
func target(method, name string) string {
	if name == "" {
		return method
	}
	return method + " " + name
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

// params returns the request's params, or nil when it has none. The SDK
// passes typed nil params for a request without params, such as ping, and
// their methods panic. go-sdk after v1.8.0 exports this check as
// mcp.HasParams; switch to it when upgrading.
func params(req mcp.Request) mcp.Params {
	p := req.GetParams()
	if p == nil {
		return nil
	}
	if v := reflect.ValueOf(p); v.Kind() == reflect.Pointer && v.IsNil() {
		return nil
	}
	return p
}

// metaCarrier adapts _meta to a read-only propagation.TextMapCarrier. Only
// string values are read.
type metaCarrier map[string]any

func (m metaCarrier) Get(key string) string {
	s, _ := m[key].(string)
	return s
}

// Set does nothing: Extract only reads, and the client's _meta must not
// change.
func (metaCarrier) Set(string, string) {}

func (m metaCarrier) Keys() []string {
	return slices.Collect(maps.Keys(m))
}
