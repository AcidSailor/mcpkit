package toolkit

import (
	"context"

	"github.com/acidsailor/mcpkit/elicit"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AddWrite registers a state-mutating MCP tool guarded by elicitation
// (ReadOnlyHint=false, IdempotentHint=false, DestructiveHint=true). Clients
// lacking elicitation capability get ErrNoElicitation. The prompt is set via
// WithElicitParamsFunc; when unset the elicitation message is empty.
func AddWrite[In, Out any](t Tool[In, Out]) {
	AddWriteFunc(
		t,
		func(
			ctx context.Context,
			req *mcp.CallToolRequest,
			in In,
		) (*mcp.CallToolResult, Out, error) {
			gate := func() error {
				var params *mcp.ElicitParams
				if t.elicitParamsFunc != nil {
					p, err := t.elicitParamsFunc(ctx, in)
					if err != nil {
						return err
					}
					params = p
				}
				return elicit.Gate(ctx, req.Session, params)
			}
			out, err := t.runValidated(ctx, in, gate)
			return nil, out, err
		},
	)
}

// AddWriteFunc registers a state-mutating MCP tool (ReadOnlyHint=false,
// IdempotentHint=false, DestructiveHint=true) with a caller-supplied handler.
// Unlike AddWrite, the handler is NOT elicitation-gated and t.runValidated is
// not applied: callFunc runs exactly as given, so the caller owns any
// confirmation, validation, and result wrapping. AddWrite is the common case;
// reach for AddWriteFunc only when you need full control of the handler.
func AddWriteFunc[In, Out any](
	t Tool[In, Out],
	callFunc mcp.ToolHandlerFor[In, Out],
) {
	tool := t.mcpTool(
		&mcp.ToolAnnotations{
			ReadOnlyHint:    false,
			IdempotentHint:  false,
			DestructiveHint: new(true),
		},
	)

	mcp.AddTool(
		t.server,
		tool,
		callFunc,
	)
}
