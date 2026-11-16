package telemetry

import (
	"context"
	"maps"
	"reflect"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"
)

// TraceSending traces outgoing messages and injects their span context.
func TraceSending(tp trace.TracerProvider, opts ...Option) mcp.Middleware {
	tracer, propagator := tracing(tp, opts)
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(
			ctx context.Context, method string, req mcp.Request,
		) (mcp.Result, error) {
			name, attrs := describe(method, params(req), req.GetSession())
			ctx, span := tracer.Start(ctx, name,
				trace.WithSpanKind(trace.SpanKindClient),
				trace.WithAttributes(attrs...))
			defer span.End()

			out, p := copyForSending(req)
			md := maps.Clone(p.GetMeta())
			if md == nil {
				md = make(map[string]any)
			}
			// Remove stale fields that injection might omit.
			for _, key := range propagator.Fields() {
				delete(md, key)
			}
			propagator.Inject(
				ctx,
				mutableMetaCarrier{metaCarrier: metaCarrier(md)},
			)
			p.SetMeta(md)
			res, err := next(ctx, method, out)
			record(span, res, err)
			return res, err
		}
	}
}

// copyForSending shallow-copies the concrete SDK request and params.
func copyForSending(req mcp.Request) (mcp.Request, mcp.Params) {
	r := copyStructPointer(req)
	p := req.GetParams()
	if p == nil {
		p = &mcp.ParamsBase{}
	} else {
		p = copyStructPointer(p).Interface().(mcp.Params)
	}
	r.Elem().FieldByName("Params").Set(reflect.ValueOf(p))
	return r.Interface().(mcp.Request), p
}

// copyStructPointer copies an SDK struct pointer, allocating typed nils.
func copyStructPointer(v any) reflect.Value {
	src := reflect.ValueOf(v)
	dst := reflect.New(src.Type().Elem())
	if !src.IsNil() {
		dst.Elem().Set(src.Elem())
	}
	return dst
}
