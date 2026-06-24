package toolkit

import (
	"context"

	"github.com/acidsailor/mcpkit/elicit"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AddWrite registers a state-mutating tool guarded by elicitation.
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

// AddWriteFunc registers a state-mutating tool running callFunc as-is, ungated.
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
