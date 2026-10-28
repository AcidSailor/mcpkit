package telemetry_test

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/acidsailor/mcpkit/telemetry"
)

func BenchmarkMiddleware(b *testing.B) {
	tp := sdktrace.NewTracerProvider()
	next := func(
		context.Context,
		string,
		mcp.Request,
	) (mcp.Result, error) {
		return nil, nil
	}
	h := telemetry.Middleware(tp)(next)
	req := &mcp.ServerRequest[*mcp.CallToolParamsRaw]{
		Session: &mcp.ServerSession{},
		Params: &mcp.CallToolParamsRaw{
			Meta: mcp.Meta{
				"traceparent": "00-0af7651916cd43dd8448eb211c80319c-" +
					"b7ad6b7169203331-01",
			},
			Name: "get_orders",
		},
	}
	ctx := context.Background()
	for b.Loop() {
		_, _ = h(ctx, "tools/call", req)
	}
}
