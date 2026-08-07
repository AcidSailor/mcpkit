package toolkit

import (
	"context"

	"github.com/acidsailor/mcpkit/elicit"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AddWrite registers an elicitation-gated write tool; it runs in two passes.
func AddWrite[In, Out any](t Tool[In, Out]) {
	AddWriteFunc(
		t,
		func(
			ctx context.Context,
			req *mcp.CallToolRequest,
			in In,
		) (*mcp.CallToolResult, Out, error) {
			var zero Out
			resp, ok := elicit.Response(req.Params)
			if !ok {
				res, err := t.ask(ctx, req.Session, in)
				return res, zero, err
			}
			if err := elicit.Decide(resp); err != nil {
				return nil, zero, t.wrap(err)
			}
			out, err := t.runValidated(ctx, in)
			return nil, out, err
		},
	)
}

// ask validates in, then builds the confirmation the client must fulfill.
func (t Tool[In, Out]) ask(
	ctx context.Context,
	session *mcp.ServerSession,
	in In,
) (*mcp.CallToolResult, error) {
	if err := t.validate(ctx, in); err != nil {
		return nil, t.wrap(err)
	}
	var params *mcp.ElicitParams
	if t.elicitParamsFunc != nil {
		p, err := t.elicitParamsFunc(ctx, in)
		if err != nil {
			return nil, t.wrap(err)
		}
		params = p
	}
	res, err := elicit.Ask(session, params)
	if err != nil {
		return nil, t.wrap(err)
	}
	return res, nil
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
