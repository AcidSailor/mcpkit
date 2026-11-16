package telemetry

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"slices"
	"strconv"

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

// MCP keys live outside the Go semconv package.
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

// Option configures TraceReceiving and TraceSending.
type Option func(*config)

// WithPropagator selects propagation through params._meta.
func WithPropagator(p propagation.TextMapPropagator) Option {
	return func(c *config) { c.propagator = p }
}

// TraceReceiving traces incoming messages using the parent in params._meta.
func TraceReceiving(tp trace.TracerProvider, opts ...Option) mcp.Middleware {
	tracer, propagator := tracing(tp, opts)
	kind := trace.WithSpanKind(trace.SpanKindServer)

	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(
			ctx context.Context,
			method string,
			req mcp.Request,
		) (mcp.Result, error) {
			p := params(req)
			name, attrs := describe(method, p, req.GetSession())

			start := make([]trace.SpanStartOption, 0, 3)
			start = append(start, kind, trace.WithAttributes(attrs...))

			ambient := trace.SpanContextFromContext(ctx)
			var md metaCarrier
			if p != nil {
				md = p.GetMeta()
			}
			ctx = propagator.Extract(ctx, md)

			// Keep the displaced ambient parent as a link.
			if ambient.IsValid() &&
				!ambient.Equal(
					trace.SpanContextFromContext(ctx),
				) {
				start = append(
					start,
					trace.WithLinks(
						trace.Link{SpanContext: ambient},
					),
				)
			}
			ctx, span := tracer.Start(ctx, name, start...)
			defer span.End()

			res, err := next(ctx, method, req)
			record(span, res, err)
			return res, err
		}
	}
}

// tracing validates the caller-owned dependencies shared by both directions.
func tracing(
	tp trace.TracerProvider,
	opts []Option,
) (trace.Tracer, propagation.TextMapPropagator) {
	if tp == nil {
		panic(ErrNilTracerProvider)
	}
	c := config{propagator: propagation.TraceContext{}}
	for _, o := range opts {
		o(&c)
	}
	if c.propagator == nil {
		panic(ErrNilPropagator)
	}
	return tp.Tracer(scope), c.propagator
}

// describe returns the span name and request attributes.
func describe(
	method string,
	p mcp.Params,
	s mcp.Session,
) (string, []attribute.KeyValue) {
	// Method, tool, operation, and session are the most set at once.
	attrs := make([]attribute.KeyValue, 0, 4)
	attrs = append(attrs, keyMethodName.String(method))

	switch p := p.(type) {
	case *mcp.CallToolParamsRaw:
		attrs = append(
			attrs,
			keyToolName.String(p.Name),
			keyOperationName.String("execute_tool"),
		)
	case *mcp.CallToolParams:
		attrs = append(attrs, keyToolName.String(p.Name),
			keyOperationName.String("execute_tool"))
	case *mcp.GetPromptParams:
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
	return spanName(method, p), attrs
}

// spanName appends a non-empty tool or prompt name to the method.
func spanName(method string, p mcp.Params) string {
	var name string
	switch p := p.(type) {
	case *mcp.CallToolParamsRaw:
		name = p.Name
	case *mcp.CallToolParams:
		name = p.Name
	case *mcp.GetPromptParams:
		name = p.Name
	}
	if name == "" {
		return method
	}
	return method + " " + name
}

// record marks failures and records available error details.
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
		if err := r.GetError(); err != nil {
			span.RecordError(err)
		}
		span.SetStatus(codes.Error, toolError)
		span.SetAttributes(semconv.ErrorTypeKey.String(toolError))
	}
}

// params normalizes typed-nil SDK params to a nil interface.
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

// metaCarrier adapts incoming _meta; Get reads only string values.
type metaCarrier map[string]any

func (m metaCarrier) Get(key string) string {
	s, _ := m[key].(string)
	return s
}

func (metaCarrier) Set(string, string) {}

func (m metaCarrier) Keys() []string {
	return slices.Collect(maps.Keys(m))
}

// mutableMetaCarrier writes injected values into the metadata map.
type mutableMetaCarrier struct{ metaCarrier }

func (m mutableMetaCarrier) Set(
	key, value string,
) {
	m.metaCarrier[key] = value
}
