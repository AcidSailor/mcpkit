package telemetry_test

import (
	"context"
	"log/slog"
	"maps"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/acidsailor/mcpkit/telemetry"
)

// Sending tests need control of both endpoints, which mcptest doesn't expose.
func sendingPair(
	t *testing.T,
	s *mcp.Server,
	c *mcp.Client,
) *mcp.ClientSession {
	t.Helper()
	ct, st := mcp.NewInMemoryTransports()
	ss, err := s.Connect(t.Context(), st, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := c.Connect(t.Context(), ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func TestTraceSendingTool(t *testing.T) {
	s, received := tracedServer(t)
	tp, sent := recorder(t)
	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	h := &recordHandler{}
	c.AddSendingMiddleware(
		func(next mcp.MethodHandler) mcp.MethodHandler {
			return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				if method != "tools/call" {
					return next(ctx, method, req)
				}
				before := maps.Clone(req.GetParams().GetMeta())
				res, err := next(ctx, method, req)
				require.Equal(t, before, req.GetParams().GetMeta())
				return res, err
			}
		},
		telemetry.TraceSending(tp),
		func(next mcp.MethodHandler) mcp.MethodHandler {
			return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				if method == "tools/call" {
					// SDK client sends use this concrete generic request.
					r, ok := req.(*mcp.ClientRequest[mcp.Params])
					if !ok {
						t.Error("request type was changed")
					} else if _, ok := r.Params.(*mcp.CallToolParams); !ok {
						t.Error("params type was changed")
					}
				}
				return next(ctx, method, req)
			}
		},
		telemetry.Log(slog.New(h)),
	)
	cs := sendingPair(t, s, c)
	ctx, parent := tp.Tracer("test").Start(t.Context(), "work")
	defer parent.End()
	md := mcp.Meta{
		"traceparent": traceparent,
		"tracestate":  "old=value",
		"custom":      "kept",
	}
	p := &mcp.CallToolParams{Name: "echo", Arguments: echoIn{}, Meta: md}
	_, err := cs.CallTool(ctx, p)
	require.NoError(t, err)
	out := span(t, sent, "tools/call echo")
	in := span(t, received, "tools/call echo")
	require.Equal(t, trace.SpanKindClient, out.SpanKind())
	require.Equal(t, parent.SpanContext(), out.Parent())
	require.Equal(t, out.SpanContext().SpanID(), in.Parent().SpanID())
	require.Equal(t, out.SpanContext().TraceID(), in.SpanContext().TraceID())
	require.True(t, in.Parent().IsRemote())
	require.Empty(t, in.Parent().TraceState().String())
	require.Equal(t, "echo", attrs(out)["gen_ai.tool.name"])
	require.Equal(t, traceparent, p.Meta["traceparent"])
	require.Equal(t, "old=value", p.Meta["tracestate"])
	require.Equal(t, "kept", p.Meta["custom"])
	require.Equal(t, out.SpanContext(), h.only(t).span)
}

func TestTraceSendingRootAndError(t *testing.T) {
	s, received := tracedServer(t)
	tp, sent := recorder(t)
	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	c.AddSendingMiddleware(telemetry.TraceSending(tp))
	cs := sendingPair(t, s, c)
	require.NoError(t, cs.Ping(t.Context(), nil))
	out := span(t, sent, "ping")
	require.False(t, out.Parent().IsValid())
	require.Equal(t, out.SpanContext().SpanID(),
		span(t, received, "ping").Parent().SpanID())
	_, err := cs.ReadResource(t.Context(),
		&mcp.ReadResourceParams{URI: "test://missing"})
	require.Error(t, err)
	require.Equal(t, codes.Error, span(t, sent, "resources/read").Status().Code)
	callEcho(t, cs, true)
	require.Equal(
		t,
		"tool_error",
		attrs(span(t, sent, "tools/call echo"))["error.type"],
	)
}

func TestTraceSendingNotification(t *testing.T) {
	tp, sr := recorder(t)
	remoteTP, remote := recorder(t)
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	s.AddReceivingMiddleware(telemetry.TraceReceiving(tp))
	s.AddSendingMiddleware(telemetry.TraceSending(tp))
	original := &mcp.ProgressNotificationParams{
		ProgressToken: "p", Message: "step",
		Meta: mcp.Meta{"custom": "kept"},
	}
	mcp.AddTool(s, &mcp.Tool{Name: "progress"}, func(
		ctx context.Context, req *mcp.CallToolRequest, _ echoIn,
	) (*mcp.CallToolResult, echoOut, error) {
		return nil, echoOut{}, req.Session.NotifyProgress(ctx, original)
	})
	got := make(chan mcp.Meta, 1)
	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"},
		&mcp.ClientOptions{
			ProgressNotificationHandler: func(_ context.Context, req *mcp.ProgressNotificationClientRequest) {
				got <- req.Params.Meta
			},
		})
	c.AddReceivingMiddleware(telemetry.TraceReceiving(remoteTP))
	cs := sendingPair(t, s, c)
	_, err := cs.CallTool(
		t.Context(),
		&mcp.CallToolParams{Name: "progress", Arguments: echoIn{}},
	)
	require.NoError(t, err)
	select {
	case md := <-got:
		require.Equal(t, "kept", md["custom"])
		require.Contains(t, md, "traceparent")
	case <-time.After(5 * time.Second):
		t.Fatal("notification not delivered")
	}
	require.Eventually(t, func() bool {
		return len(remote.Ended()) > 0
	}, time.Second, time.Millisecond)
	out := span(t, sr, "notifications/progress")
	in := span(t, remote, "notifications/progress")
	require.Equal(
		t,
		span(t, sr, "tools/call progress").SpanContext(),
		out.Parent(),
	)
	require.Equal(t, out.SpanContext().SpanID(), in.Parent().SpanID())
	require.Equal(t, out.SpanContext().TraceID(), in.SpanContext().TraceID())
	require.Equal(t, mcp.Meta{"custom": "kept"}, original.Meta)
}

// SDK sends lack typed-nil params fields and RequestExtra.
func TestTraceSendingTypedNil(t *testing.T) {
	tp, _ := recorder(t)
	req := &mcp.ServerRequest[*mcp.PingParams]{
		Session: &mcp.ServerSession{},
		Extra:   &mcp.RequestExtra{},
	}
	_, err := telemetry.TraceSending(tp)(func(
		ctx context.Context, _ string, out mcp.Request,
	) (mcp.Result, error) {
		copy, ok := out.(*mcp.ServerRequest[*mcp.PingParams])
		require.True(t, ok)
		require.NotSame(t, req, copy)
		require.Same(t, req.Extra, copy.Extra)
		require.Same(t, req.Session, copy.Session)
		require.NotNil(t, copy.Params)
		carrier := propagation.MapCarrier{}
		for k, v := range copy.Params.GetMeta() {
			carrier[k] = v.(string)
		}
		extracted := propagation.TraceContext{}.Extract(
			context.Background(),
			carrier,
		)
		require.Equal(t, trace.SpanContextFromContext(ctx).SpanID(),
			trace.SpanContextFromContext(extracted).SpanID())
		return nil, nil
	})(t.Context(), "ping", req)
	require.NoError(t, err)
	require.Nil(t, req.Params)
}

func TestTraceSendingNilDependencies(t *testing.T) {
	require.PanicsWithValue(t, telemetry.ErrNilTracerProvider, func() {
		telemetry.TraceSending(nil)
	})
	tp, _ := recorder(t)
	require.PanicsWithValue(t, telemetry.ErrNilPropagator, func() {
		telemetry.TraceSending(tp, telemetry.WithPropagator(nil))
	})
}

// A composite propagator also carries caller-selected metadata such as baggage.
func TestTraceSendingCustomPropagator(t *testing.T) {
	tp, _ := recorder(t)
	prop := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	)
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	got := make(chan string, 1)
	s.AddReceivingMiddleware(
		telemetry.TraceReceiving(tp, telemetry.WithPropagator(prop)),
		func(next mcp.MethodHandler) mcp.MethodHandler {
			return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				if method == "ping" {
					got <- baggage.FromContext(ctx).Member("tenant").Value()
				}
				return next(ctx, method, req)
			}
		},
	)
	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	c.AddSendingMiddleware(
		telemetry.TraceSending(tp, telemetry.WithPropagator(prop)),
	)
	cs := sendingPair(t, s, c)
	member, err := baggage.NewMember("tenant", "demo")
	require.NoError(t, err)
	bag, err := baggage.New(member)
	require.NoError(t, err)
	require.NoError(
		t,
		cs.Ping(baggage.ContextWithBaggage(t.Context(), bag), nil),
	)
	require.Equal(t, "demo", <-got)
}
