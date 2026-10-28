package telemetry_test

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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

// tracedSession serves a traced server and returns its client and spans.
func tracedSession(
	t *testing.T,
) (*mcp.ClientSession, *tracetest.SpanRecorder) {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	s.AddReceivingMiddleware(telemetry.Middleware(tp))
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
		Meta: mcp.Meta{
			"traceparent": "00-0af7651916cd43dd8448eb211c80319c-" +
				"b7ad6b7169203331-01",
		},
		Name:      "echo",
		Arguments: echoIn{},
	})
	require.NoError(t, err)

	p := span(t, sr, "tools/call echo").Parent()
	require.True(t, p.IsRemote())
	require.Equal(t, "0af7651916cd43dd8448eb211c80319c", p.TraceID().String())
	require.Equal(t, "b7ad6b7169203331", p.SpanID().String())
}

// direct calls the middleware with next returning an empty result.
func direct(
	t *testing.T,
	method string,
	req mcp.Request,
) *tracetest.SpanRecorder {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	next := func(
		context.Context,
		string,
		mcp.Request,
	) (mcp.Result, error) {
		return nil, nil
	}
	_, err := telemetry.Middleware(tp)(next)(t.Context(), method, req)
	require.NoError(t, err)
	return sr
}

func TestNotificationsNotTraced(t *testing.T) {
	sr := direct(
		t,
		"notifications/progress",
		&mcp.ServerRequest[*mcp.ProgressNotificationParams]{},
	)
	require.Empty(t, sr.Ended())
}

func TestNilParamsAndSession(t *testing.T) {
	sr := direct(
		t,
		"tools/call",
		&mcp.ServerRequest[*mcp.CallToolParamsRaw]{},
	)
	s := span(t, sr, "tools/call")
	require.Equal(t, map[attribute.Key]string{
		"mcp.method.name": "tools/call",
	}, attrs(s))
}

func TestAmbientLinkedUnderMetaParent(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	ctx, ambient := tp.Tracer("http").Start(t.Context(), "POST /mcp")
	defer ambient.End()

	next := func(
		context.Context,
		string,
		mcp.Request,
	) (mcp.Result, error) {
		return nil, nil
	}
	req := &mcp.ServerRequest[*mcp.CallToolParamsRaw]{
		Params: &mcp.CallToolParamsRaw{
			Meta: mcp.Meta{
				"traceparent": "00-0af7651916cd43dd8448eb211c80319c-" +
					"b7ad6b7169203331-01",
			},
			Name: "echo",
		},
	}
	_, err := telemetry.Middleware(tp)(next)(ctx, "tools/call", req)
	require.NoError(t, err)

	s := span(t, sr, "tools/call echo")
	require.Equal(t, "0af7651916cd43dd8448eb211c80319c",
		s.Parent().TraceID().String())
	require.Len(t, s.Links(), 1)
	require.Equal(t, ambient.SpanContext(), s.Links()[0].SpanContext)
}

func TestAmbientParentWithoutMeta(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	ctx, ambient := tp.Tracer("http").Start(t.Context(), "POST /mcp")
	defer ambient.End()

	next := func(
		context.Context,
		string,
		mcp.Request,
	) (mcp.Result, error) {
		return nil, nil
	}
	req := &mcp.ServerRequest[*mcp.ListToolsParams]{}
	_, err := telemetry.Middleware(tp)(next)(ctx, "tools/list", req)
	require.NoError(t, err)

	s := span(t, sr, "tools/list")
	require.Equal(t, ambient.SpanContext().SpanID(), s.Parent().SpanID())
	require.Empty(t, s.Links())
}

func TestNilTracerProviderPanics(t *testing.T) {
	require.PanicsWithValue(t, telemetry.ErrNilTracerProvider, func() {
		telemetry.Middleware(nil)
	})
}
