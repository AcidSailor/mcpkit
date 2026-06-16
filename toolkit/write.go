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
