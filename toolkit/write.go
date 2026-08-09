package toolkit

import (
	"context"

	"github.com/acidsailor/mcpkit/elicit"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AddWrite registers a validated write tool with an elicitation gate.
func AddWrite[In, Out any](t Tool[In, Out]) {
	AddWriteFunc(t, t.Gate)
}

// AddWriteFunc registers a write handler without validation or elicitation.
func AddWriteFunc[In, Out any](
	t Tool[In, Out],
	callFunc mcp.ToolHandlerFor[In, Out],
) {
	mcp.AddTool(
		t.server,
		t.mcpTool(false),
		t.wrapHandler(callFunc),
	)
}

// Gate requests confirmation, then calls the tool on an accepted retry.
func (t Tool[In, Out]) Gate(
	ctx context.Context,
	req *mcp.CallToolRequest,
	in In,
) (*mcp.CallToolResult, Out, error) {
	var zero Out
	resp, ok := req.Params.InputResponses[t.gate()]
	if !ok {
		res, err := t.ask(ctx, req.Session, in)
		return res, zero, err
	}
	if err := elicit.Decide(resp); err != nil {
		return nil, zero, err
	}
	return t.Call(ctx, req, in)
}

// ask validates input and builds the confirmation request.
func (t Tool[In, Out]) ask(
	ctx context.Context,
	session *mcp.ServerSession,
	in In,
) (*mcp.CallToolResult, error) {
	if err := t.validate(ctx, in); err != nil {
		return nil, err
	}
	build := t.elicitParamsFunc
	if build == nil {
		build = elicit.SimpleConfirmation[In]("Run " + t.name + "?")
	}
	params, err := build(ctx, in)
	if err != nil {
		return nil, err
	}
	return elicit.Ask(t.gate(), session, params)
}
