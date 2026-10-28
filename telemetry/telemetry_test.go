package telemetry_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/acidsailor/mcpkit/mcptest"
	"github.com/acidsailor/mcpkit/telemetry"
)

type echoIn struct {
	Fail bool `json:"fail"`
}

type echoOut struct {
	OK bool `json:"ok"`
}

const (
	traceID     = "0af7651916cd43dd8448eb211c80319c"
	spanID      = "b7ad6b7169203331"
	traceparent = "00-" + traceID + "-" + spanID + "-01"
)

// nop is a next handler that returns a nil result and no error.
func nop(context.Context, string, mcp.Request) (mcp.Result, error) {
	return nil, nil
}

// recorder returns a sampling provider whose ended spans sr records.
func recorder(
	t *testing.T,
) (*sdktrace.TracerProvider, *tracetest.SpanRecorder) {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return tp, sr
}

// tracedServer returns a traced server with a tool, a resource, and two
// prompts, one of which fails.
func tracedServer(
	t *testing.T,
	opts ...telemetry.Option,
) (*mcp.Server, *tracetest.SpanRecorder) {
	t.Helper()
	tp, sr := recorder(t)
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	s.AddReceivingMiddleware(telemetry.Middleware(tp, opts...))
	mcp.AddTool(
		s,
		&mcp.Tool{Name: "echo", Description: "echo"},
		func(
			_ context.Context,
			_ *mcp.CallToolRequest,
			in echoIn,
		) (*mcp.CallToolResult, echoOut, error) {
			if in.Fail {
				return nil, echoOut{}, errors.New("boom")
			}
			return nil, echoOut{OK: true}, nil
		},
	)
	s.AddResource(
		&mcp.Resource{URI: "test://known", Name: "known"},
		func(
			_ context.Context,
			req *mcp.ReadResourceRequest,
		) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{URI: req.Params.URI, Text: "hi"},
				},
			}, nil
		},
	)
	s.AddPrompt(
		&mcp.Prompt{Name: "greet"},
		func(
			_ context.Context,
			_ *mcp.GetPromptRequest,
		) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{}, nil
		},
	)
	s.AddPrompt(
		&mcp.Prompt{Name: "fail"},
		func(
			context.Context,
			*mcp.GetPromptRequest,
		) (*mcp.GetPromptResult, error) {
			return nil, errors.New("boom")
		},
	)
	return s, sr
}

// tracedSession connects a client to a traced server.
func tracedSession(
	t *testing.T,
	opts ...telemetry.Option,
) (*mcp.ClientSession, *tracetest.SpanRecorder) {
	t.Helper()
	s, sr := tracedServer(t, opts...)
	return mcptest.NewSession(t, s), sr
}

// span returns the only ended span named name.
func span(
	t *testing.T,
	sr *tracetest.SpanRecorder,
	name string,
) sdktrace.ReadOnlySpan {
	t.Helper()
	var found []sdktrace.ReadOnlySpan
	for _, s := range sr.Ended() {
		if s.Name() == name {
			found = append(found, s)
		}
	}
	require.Len(t, found, 1, "spans named %q", name)
	return found[0]
}

func attrs(s sdktrace.ReadOnlySpan) map[attribute.Key]string {
	m := map[attribute.Key]string{}
	for _, kv := range s.Attributes() {
		m[kv.Key] = kv.Value.AsString()
	}
	return m
}

func TestToolCallSpan(t *testing.T) {
	cs, sr := tracedSession(t)

	_, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "echo",
		Arguments: echoIn{},
	})
	require.NoError(t, err)

	s := span(t, sr, "tools/call echo")
	require.Equal(t, trace.SpanKindServer, s.SpanKind())
	require.Equal(t, codes.Unset, s.Status().Code)
	require.False(t, s.Parent().IsValid())
	require.Equal(t, map[attribute.Key]string{
		"mcp.method.name":       "tools/call",
		"gen_ai.tool.name":      "echo",
		"gen_ai.operation.name": "execute_tool",
	}, attrs(s))
}

func TestToolErrorSpan(t *testing.T) {
	cs, sr := tracedSession(t)

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "echo",
		Arguments: echoIn{Fail: true},
	})
	require.NoError(t, err)
	require.True(t, res.IsError)

	s := span(t, sr, "tools/call echo")
	require.Equal(t, codes.Error, s.Status().Code)
	require.Equal(t, "tool_error", attrs(s)["error.type"])
}

func TestCallErrorSpan(t *testing.T) {
	cs, sr := tracedSession(t)

	_, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{
		URI: "test://missing",
	})
	var rpcErr *jsonrpc.Error
	require.ErrorAs(t, err, &rpcErr)
	code := strconv.FormatInt(rpcErr.Code, 10)

	s := span(t, sr, "resources/read")
	require.Equal(t, codes.Error, s.Status().Code)
	a := attrs(s)
	require.Equal(t, "test://missing", a["mcp.resource.uri"])
	require.Equal(t, code, a["error.type"])
	require.Equal(t, code, a["rpc.response.status_code"])
	require.Len(t, s.Events(), 1)
}

func TestPlainErrorSpan(t *testing.T) {
	cs, sr := tracedSession(t)

	_, err := cs.GetPrompt(t.Context(), &mcp.GetPromptParams{Name: "fail"})
	require.Error(t, err)

	s := span(t, sr, "prompts/get fail")
	require.Equal(t, codes.Error, s.Status().Code)
	a := attrs(s)
	require.Equal(t, "*errors.errorString", a["error.type"])
	require.NotContains(t, a, attribute.Key("rpc.response.status_code"))
	require.Len(t, s.Events(), 1)
}

func TestPromptSpan(t *testing.T) {
	cs, sr := tracedSession(t)

	_, err := cs.GetPrompt(t.Context(), &mcp.GetPromptParams{Name: "greet"})
	require.NoError(t, err)

	s := span(t, sr, "prompts/get greet")
	require.Equal(t, "greet", attrs(s)["gen_ai.prompt.name"])
}

func TestParentFromMeta(t *testing.T) {
	cs, sr := tracedSession(t)

	_, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Meta:      mcp.Meta{"traceparent": traceparent},
		Name:      "echo",
		Arguments: echoIn{},
	})
	require.NoError(t, err)

	p := span(t, sr, "tools/call echo").Parent()
	require.True(t, p.IsRemote())
	require.Equal(t, traceID, p.TraceID().String())
	require.Equal(t, spanID, p.SpanID().String())
}

func TestWithPropagatorReplacesDefault(t *testing.T) {
	cs, sr := tracedSession(t, telemetry.WithPropagator(propagation.Baggage{}))

	_, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Meta:      mcp.Meta{"traceparent": traceparent},
		Name:      "echo",
		Arguments: echoIn{},
	})
	require.NoError(t, err)

	require.False(t, span(t, sr, "tools/call echo").Parent().IsValid())
}

func TestNonStringMetaIgnored(t *testing.T) {
	cs, sr := tracedSession(t)

	_, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Meta:      mcp.Meta{"traceparent": 123},
		Name:      "echo",
		Arguments: echoIn{},
	})
	require.NoError(t, err)

	require.False(t, span(t, sr, "tools/call echo").Parent().IsValid())
}

// TestPingSpan covers the SDK's typed nil params for a request without
// params.
func TestPingSpan(t *testing.T) {
	cs, sr := tracedSession(t)
	require.NoError(t, cs.Ping(t.Context(), nil))

	span(t, sr, "ping")
}

// direct calls the middleware for what mcptest cannot produce and returns
// the one span it ends.
func direct(
	t *testing.T,
	ctx context.Context,
	method string,
	req mcp.Request,
) sdktrace.ReadOnlySpan {
	t.Helper()
	tp, sr := recorder(t)
	_, err := telemetry.Middleware(tp)(nop)(ctx, method, req)
	require.NoError(t, err)
	require.Len(t, sr.Ended(), 1)
	return sr.Ended()[0]
}

// TestSubscribeResourceURI hand-builds the request: protocol 2026-07-28
// clients, including mcptest's, use subscriptions/listen instead.
func TestSubscribeResourceURI(t *testing.T) {
	for method, req := range map[string]mcp.Request{
		"resources/subscribe": &mcp.ServerRequest[*mcp.SubscribeParams]{
			Session: &mcp.ServerSession{},
			Params:  &mcp.SubscribeParams{URI: "test://known"},
		},
		"resources/unsubscribe": &mcp.ServerRequest[*mcp.UnsubscribeParams]{
			Session: &mcp.ServerSession{},
			Params:  &mcp.UnsubscribeParams{URI: "test://known"},
		},
	} {
		t.Run(method, func(t *testing.T) {
			s := direct(t, t.Context(), method, req)
			require.Equal(t, method, s.Name())
			require.Equal(t, "test://known", attrs(s)["mcp.resource.uri"])
		})
	}
}

func TestEmptyToolNameSpan(t *testing.T) {
	cs, sr := tracedSession(t)
	_, _ = cs.CallTool(t.Context(), &mcp.CallToolParams{})

	span(t, sr, "tools/call")
}

func TestNotificationsNotTraced(t *testing.T) {
	s, sr := tracedServer(t)
	seen := make(chan struct{}, 1)
	s.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(
			ctx context.Context,
			method string,
			req mcp.Request,
		) (mcp.Result, error) {
			res, err := next(ctx, method, req)
			if method == "notifications/progress" {
				seen <- struct{}{}
			}
			return res, err
		}
	})
	cs := mcptest.NewSession(t, s)

	require.NoError(t, cs.NotifyProgress(
		t.Context(),
		&mcp.ProgressNotificationParams{ProgressToken: "p", Progress: 1},
	))
	select {
	case <-seen:
	case <-time.After(5 * time.Second):
		t.Fatal("progress notification did not reach the server")
	}
	for _, s := range sr.Ended() {
		require.NotContains(t, s.Name(), "notifications/")
	}
}

// httpSpan stands in for an HTTP server span in the handler context.
// mcptest connects with a background context, so it cannot supply one.
var httpSpan = trace.NewSpanContext(trace.SpanContextConfig{
	TraceID:    trace.TraceID{1},
	SpanID:     trace.SpanID{1},
	TraceFlags: trace.FlagsSampled,
})

func TestAmbientLinkedUnderMetaParent(t *testing.T) {
	ctx := trace.ContextWithSpanContext(t.Context(), httpSpan)
	s := direct(
		t,
		ctx,
		"tools/call",
		&mcp.ServerRequest[*mcp.CallToolParamsRaw]{
			Session: &mcp.ServerSession{},
			Params: &mcp.CallToolParamsRaw{
				Meta: mcp.Meta{"traceparent": traceparent},
				Name: "echo",
			},
		},
	)

	require.Equal(t, traceID, s.Parent().TraceID().String())
	require.Len(t, s.Links(), 1)
	require.Equal(t, httpSpan, s.Links()[0].SpanContext)
}

func TestAmbientParentWithoutMeta(t *testing.T) {
	ctx := trace.ContextWithSpanContext(t.Context(), httpSpan)
	s := direct(t, ctx, "tools/list", &mcp.ServerRequest[*mcp.ListToolsParams]{
		Session: &mcp.ServerSession{},
	})

	require.Equal(t, httpSpan.SpanID(), s.Parent().SpanID())
	require.Empty(t, s.Links())
}

func TestNilTracerProviderPanics(t *testing.T) {
	require.PanicsWithValue(t, telemetry.ErrNilTracerProvider, func() {
		telemetry.Middleware(nil)
	})
}

func TestNilPropagatorPanics(t *testing.T) {
	tp, _ := recorder(t)
	require.PanicsWithValue(t, telemetry.ErrNilPropagator, func() {
		telemetry.Middleware(tp, telemetry.WithPropagator(nil))
	})
}
