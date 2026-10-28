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
	h := telemetry.Middleware(tp)(nop)
	req := &mcp.ServerRequest[*mcp.CallToolParamsRaw]{
		Session: &mcp.ServerSession{},
		Params: &mcp.CallToolParamsRaw{
			Meta: mcp.Meta{"traceparent": traceparent},
			Name: "get_orders",
		},
	}
	ctx := context.Background()
	for b.Loop() {
		_, _ = h(ctx, "tools/call", req)
	}
}
