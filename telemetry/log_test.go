package telemetry_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"

	"github.com/acidsailor/mcpkit/mcptest"
	"github.com/acidsailor/mcpkit/telemetry"
	"github.com/acidsailor/mcpkit/toolkit"
)

// logRecord captures a record and its context span.
type logRecord struct {
	level slog.Level
	msg   string
	attrs map[string]slog.Value
	span  trace.SpanContext
}

// strings returns the record's attributes as strings.
func (r logRecord) strings() map[string]string {
	m := map[string]string{}
	for k, v := range r.attrs {
		m[k] = v.String()
	}
	return m
}

// recordHandler records each record at or above level.
type recordHandler struct {
	level   slog.Level
	mu      sync.Mutex
	records []logRecord
}

func (h *recordHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level
}

func (h *recordHandler) Handle(ctx context.Context, r slog.Record) error {
	rec := logRecord{
		level: r.Level,
		msg:   r.Message,
		attrs: map[string]slog.Value{},
		span:  trace.SpanContextFromContext(ctx),
	}
	r.Attrs(func(a slog.Attr) bool {
		rec.attrs[a.Key] = a.Value
		return true
	})
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, rec)
	return nil
}

func (h *recordHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *recordHandler) WithGroup(string) slog.Handler { return h }

// all excludes session setup records (server/discover).
func (h *recordHandler) all() []logRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	var recs []logRecord
	for _, r := range h.records {
		if r.msg != "server/discover" {
			recs = append(recs, r)
		}
	}
	return recs
}

// only returns the one record.
func (h *recordHandler) only(t *testing.T) logRecord {
	t.Helper()
	recs := h.all()
	require.Len(t, recs, 1)
	return recs[0]
}

// loggedServer registers an echo tool, a gated write, and a resource.
func loggedServer(
	t *testing.T,
	level slog.Level,
	opts ...telemetry.LogOption,
) (*mcp.Server, *recordHandler) {
	t.Helper()
	h := &recordHandler{level: level}
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	s.AddReceivingMiddleware(telemetry.Log(slog.New(h), opts...))
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
	toolkit.AddWrite(toolkit.New(s, "do", "does", nil,
		func(context.Context, echoIn) (echoOut, error) {
			return echoOut{OK: true}, nil
		}))
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
	return s, h
}

func callEcho(t *testing.T, cs *mcp.ClientSession, fail bool) {
	t.Helper()
	_, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "echo",
		Arguments: echoIn{Fail: fail},
	})
	require.NoError(t, err)
}

func TestLogToolCall(t *testing.T) {
	s, h := loggedServer(t, slog.LevelDebug)
	callEcho(t, mcptest.NewSession(t, s), false)

	r := h.only(t)
	require.Equal(t, slog.LevelInfo, r.level)
	require.Equal(t, "tools/call echo", r.msg)
	// The span holds the request attributes, so the record repeats none.
	require.Equal(t, map[string]string{"outcome": "ok"}, r.strings())
}

func TestLogToolError(t *testing.T) {
	s, h := loggedServer(t, slog.LevelDebug)
	callEcho(t, mcptest.NewSession(t, s), true)

	r := h.only(t)
	require.Equal(t, slog.LevelWarn, r.level)
	require.Equal(t, map[string]string{"outcome": "tool_error"}, r.strings())
}

func TestLogCallError(t *testing.T) {
	s, h := loggedServer(t, slog.LevelDebug, telemetry.WithPayload(
		func(_ string, _ mcp.Params, res mcp.Result) []slog.Attr {
			return []slog.Attr{slog.Bool("nil_result", res == nil)}
		},
	))
	_, err := mcptest.NewSession(t, s).ReadResource(
		t.Context(),
		&mcp.ReadResourceParams{URI: "test://missing"},
	)
	require.Error(t, err)

	r := h.only(t)
	require.Equal(t, slog.LevelError, r.level)
	require.Equal(t, "resources/read", r.msg)
	require.Equal(t, map[string]string{
		"outcome": "error", "nil_result": "true",
	}, r.strings())
}

func TestLogLevelFunc(t *testing.T) {
	s, h := loggedServer(t, slog.LevelWarn, telemetry.WithLevelFunc(
		func(_ string, o telemetry.Outcome, err error) (slog.Level, bool) {
			switch o {
			case telemetry.OutcomeError:
				if e, ok := errors.AsType[*jsonrpc.Error](err); ok &&
					e.Code == jsonrpc.CodeInvalidParams {
					return slog.LevelInfo, true
				}
			case telemetry.OutcomeToolError:
				if err != nil && err.Error() == "boom" {
					return slog.LevelError, true
				}
			}
			return o.DefaultLevel(), true
		},
	))
	cs := mcptest.NewSession(t, s)
	_, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{
		URI: "test://missing",
	})
	var wireErr *jsonrpc.Error
	require.ErrorAs(t, err, &wireErr)
	require.EqualValues(t, jsonrpc.CodeInvalidParams, wireErr.Code)
	require.Empty(t, h.all())

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "echo", Arguments: echoIn{Fail: true},
	})
	require.NoError(t, err)
	require.True(t, res.IsError)
	r := h.only(t)
	require.Equal(t, slog.LevelError, r.level)
	require.Equal(t, "tool_error", r.attrs["outcome"].String())
}

func TestNilLevelFuncPanics(t *testing.T) {
	require.PanicsWithValue(t, telemetry.ErrNilLevelFunc, func() {
		telemetry.WithLevelFunc(nil)
	})
}

// Current clients resend gated writes after answering the prompt.
func TestLogGatedWrite(t *testing.T) {
	for action, want := range map[string]struct {
		level   slog.Level
		outcome string
	}{
		"accept":  {slog.LevelInfo, "ok"},
		"decline": {slog.LevelInfo, "declined"},
		"cancel":  {slog.LevelInfo, "canceled"},
	} {
		t.Run(action, func(t *testing.T) {
			s, h := loggedServer(t, slog.LevelDebug)
			cs := mcptest.NewSessionWithElicitation(t, s, func(
				context.Context, *mcp.ElicitRequest,
			) (*mcp.ElicitResult, error) {
				return &mcp.ElicitResult{Action: action}, nil
			})
			_, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
				Name:      "do",
				Arguments: echoIn{},
			})
			require.NoError(t, err)

			recs := h.all()
			require.Len(t, recs, 2)
			require.Equal(t, slog.LevelInfo, recs[0].level)
			require.Equal(
				t,
				"input_required",
				recs[0].attrs["outcome"].String(),
			)
			require.Equal(t, want.level, recs[1].level)
			require.Equal(t, want.outcome, recs[1].attrs["outcome"].String())
		})
	}
}

func TestLogPendingPromptAndResource(t *testing.T) {
	s, h := loggedServer(t, slog.LevelDebug)
	input := mcp.InputRequestMap{"confirm": &mcp.ElicitParams{Message: "OK?"}}
	s.AddPrompt(&mcp.Prompt{Name: "gated"}, func(
		_ context.Context, req *mcp.GetPromptRequest,
	) (*mcp.GetPromptResult, error) {
		if len(req.Params.InputResponses) == 0 {
			return &mcp.GetPromptResult{InputRequests: input}, nil
		}
		return &mcp.GetPromptResult{Messages: []*mcp.PromptMessage{}}, nil
	})
	const uri = "test://gated"
	s.AddResource(&mcp.Resource{Name: "gated", URI: uri}, func(
		_ context.Context, req *mcp.ReadResourceRequest,
	) (*mcp.ReadResourceResult, error) {
		if len(req.Params.InputResponses) == 0 {
			return &mcp.ReadResourceResult{InputRequests: input}, nil
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{
			{URI: uri, Text: "ok"},
		}}, nil
	})
	cs := mcptest.NewSessionWithElicitation(t, s, func(
		context.Context, *mcp.ElicitRequest,
	) (*mcp.ElicitResult, error) {
		return &mcp.ElicitResult{Action: "accept"}, nil
	})
	_, err := cs.GetPrompt(t.Context(), &mcp.GetPromptParams{Name: "gated"})
	require.NoError(t, err)
	_, err = cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: uri})
	require.NoError(t, err)
	var outcomes []string
	for _, rec := range h.all() {
		outcomes = append(outcomes, rec.attrs["outcome"].String())
	}
	require.Equal(
		t,
		[]string{"input_required", "ok", "input_required", "ok"},
		outcomes,
	)
}

func TestLogPayload(t *testing.T) {
	var got mcp.Result
	s, h := loggedServer(t, slog.LevelDebug, telemetry.WithPayload(
		func(method string, p mcp.Params, res mcp.Result) []slog.Attr {
			if method != "tools/call" {
				return nil
			}
			got = res
			return []slog.Attr{slog.String(
				"args",
				string(p.(*mcp.CallToolParamsRaw).Arguments),
			)}
		},
	))
	callEcho(t, mcptest.NewSession(t, s), false)

	require.Equal(t, `{"fail":false}`, h.only(t).attrs["args"].String())
	require.IsType(t, &mcp.CallToolResult{}, got)
}

// Ping supplies typed-nil params.
func TestLogPingPayload(t *testing.T) {
	var got mcp.Params = &mcp.CallToolParamsRaw{}
	s, h := loggedServer(t, slog.LevelDebug, telemetry.WithPayload(
		func(_ string, p mcp.Params, _ mcp.Result) []slog.Attr {
			got = p
			return nil
		},
	))
	require.NoError(t, mcptest.NewSession(t, s).Ping(t.Context(), nil))

	require.Equal(t, "ping", h.only(t).msg)
	require.Nil(t, got)
}

func TestLogDisabledLevelSkipsPayload(t *testing.T) {
	called := false
	s, h := loggedServer(t, slog.LevelError, telemetry.WithPayload(
		func(string, mcp.Params, mcp.Result) []slog.Attr {
			called = true
			return nil
		},
	))
	callEcho(t, mcptest.NewSession(t, s), false)

	require.Empty(t, h.all())
	require.False(t, called)
}

func TestLogNotifications(t *testing.T) {
	for name, discard := range map[string]bool{"default": false, "discard": true} {
		t.Run(name, func(t *testing.T) {
			h := &recordHandler{}
			received := make(chan string, 1)
			s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"},
				&mcp.ServerOptions{
					ProgressNotificationHandler: func(
						_ context.Context, req *mcp.ProgressNotificationServerRequest,
					) {
						received <- req.Params.Message
					},
				})
			payloadCalled := false
			opts := []telemetry.LogOption{telemetry.WithPayload(
				func(method string, _ mcp.Params, _ mcp.Result) []slog.Attr {
					if method == "notifications/progress" {
						payloadCalled = true
					}
					return nil
				},
			)}
			if discard {
				opts = append(opts, telemetry.WithLevelFunc(
					func(
						method string, o telemetry.Outcome, _ error,
					) (slog.Level, bool) {
						return o.DefaultLevel(), method != "notifications/progress"
					},
				))
			}
			done := make(chan struct{})
			s.AddReceivingMiddleware(
				func(next mcp.MethodHandler) mcp.MethodHandler {
					return func(
						ctx context.Context, method string, req mcp.Request,
					) (mcp.Result, error) {
						if method == "notifications/progress" {
							defer close(done)
						}
						return next(ctx, method, req)
					}
				},
				telemetry.Log(slog.New(h), opts...),
			)
			cs := mcptest.NewSession(t, s)
			require.NoError(t, cs.NotifyProgress(
				t.Context(),
				&mcp.ProgressNotificationParams{
					ProgressToken: "p",
					Message:       "step",
				},
			))
			select {
			case msg := <-received:
				require.Equal(t, "step", msg)
			case <-time.After(5 * time.Second):
				t.Fatal("progress notification was not forwarded")
			}
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("progress middleware did not return")
			}
			require.Equal(t, !discard, payloadCalled)
			if discard {
				require.Empty(t, h.all())
			} else {
				r := h.only(t)
				require.Equal(t, "notifications/progress", r.msg)
				require.Equal(t, slog.LevelInfo, r.level)
			}
		})
	}
}

// Tracing must wrap logging to pass its span through context.
func TestLogInsideSpan(t *testing.T) {
	tp, sr := recorder(t)
	h := &recordHandler{}
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	s.AddReceivingMiddleware(
		telemetry.TraceReceiving(tp),
		telemetry.Log(slog.New(h)),
	)
	require.NoError(t, mcptest.NewSession(t, s).Ping(t.Context(), nil))

	r := h.only(t)
	require.True(t, r.span.IsValid())
	require.Equal(t, span(t, sr, "ping").SpanContext(), r.span)
}

func TestNilLoggerPanics(t *testing.T) {
	require.PanicsWithValue(t, telemetry.ErrNilLogger, func() {
		telemetry.Log(nil)
	})
}

func TestNilPayloadFuncPanics(t *testing.T) {
	require.PanicsWithValue(t, telemetry.ErrNilPayloadFunc, func() {
		telemetry.WithPayload(nil)
	})
}
