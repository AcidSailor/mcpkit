package telemetry

import (
	"context"
	"errors"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/acidsailor/mcpkit/elicit"
)

// Outcome describes how an MCP request completed.
type Outcome string

// Request outcomes logged under the outcome key.
const (
	OutcomeOK            Outcome = "ok"
	OutcomeInputRequired Outcome = "input_required"
	OutcomeDeclined      Outcome = "declined"
	OutcomeCanceled      Outcome = "canceled"
	OutcomeToolError     Outcome = "tool_error"
	OutcomeError         Outcome = "error"
)

// DefaultLevel maps call errors to Error, tool errors to Warn, others to Info.
func (o Outcome) DefaultLevel() slog.Level {
	switch o {
	case OutcomeToolError:
		return slog.LevelWarn
	case OutcomeError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// LevelFunc selects severity and whether to emit the record.
type LevelFunc func(
	method string, outcome Outcome, err error,
) (level slog.Level, emit bool)

// PayloadFunc selects request and result attributes to log.
type PayloadFunc func(method string, p mcp.Params, res mcp.Result) []slog.Attr

type logConfig struct {
	payload PayloadFunc
	level   LevelFunc
}

// LogOption configures Log.
type LogOption func(*logConfig)

// WithLevelFunc overrides severity and emission; nil panics.
func WithLevelFunc(f LevelFunc) LogOption {
	if f == nil {
		panic(ErrNilLevelFunc)
	}
	return func(c *logConfig) { c.level = f }
}

// WithPayload adds caller-selected attributes; nil panics.
func WithPayload(f PayloadFunc) LogOption {
	if f == nil {
		panic(ErrNilPayloadFunc)
	}
	return func(c *logConfig) { c.payload = f }
}

// Log logs requests and notifications in either middleware direction.
func Log(l *slog.Logger, opts ...LogOption) mcp.Middleware {
	if l == nil {
		panic(ErrNilLogger)
	}
	var c logConfig
	for _, o := range opts {
		o(&c)
	}

	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(
			ctx context.Context,
			method string,
			req mcp.Request,
		) (mcp.Result, error) {
			res, err := next(ctx, method, req)

			outcome, cause := classify(res, err)

			level, emit := outcome.DefaultLevel(), true
			if c.level != nil {
				level, emit = c.level(method, outcome, cause)
			}
			if !emit || !l.Enabled(ctx, level) {
				return res, err
			}

			// Leave request attributes, duration, and error on the span.
			p := params(req)
			attrs := []slog.Attr{
				slog.String("outcome", string(outcome)),
			}
			if c.payload != nil {
				payload := res
				if err != nil {
					payload = nil
				}
				attrs = append(
					attrs,
					c.payload(method, p, payload)...,
				)
			}
			l.LogAttrs(
				ctx, level,
				spanName(method, p), attrs...,
			)
			return res, err
		}
	}
}

// classify returns the outcome and cause without changing the wire response.
func classify(res mcp.Result, err error) (Outcome, error) {
	if err != nil {
		return OutcomeError, err
	}
	switch r := res.(type) {
	case *mcp.GetPromptResult:
		if r != nil && r.NeedsInput() {
			return OutcomeInputRequired, nil
		}
	case *mcp.ReadResourceResult:
		if r != nil && r.NeedsInput() {
			return OutcomeInputRequired, nil
		}
	case *mcp.CallToolResult:
		if r != nil {
			if r.NeedsInput() {
				return OutcomeInputRequired, nil
			}
			if r.IsError {
				return classifyToolError(r.GetError())
			}
		}
	}
	return OutcomeOK, nil
}

func classifyToolError(cause error) (Outcome, error) {
	switch {
	case errors.Is(cause, elicit.ErrUserDeclined):
		return OutcomeDeclined, cause
	case errors.Is(cause, elicit.ErrUserCanceled):
		return OutcomeCanceled, cause
	default:
		return OutcomeToolError, cause
	}
}
