package toolkit

import (
	"context"

	"github.com/acidsailor/mcpkit/elicit"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AddWrite registers an elicitation-gated write tool; it runs in two passes.
func AddWrite[In, Out any](t Tool[In, Out]) {
	AddWriteFunc(t, t.Gate)
}

// AddWriteFunc registers a state-mutating tool running callFunc as-is, ungated.
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

// Gate asks for confirmation on the first pass and calls on the retry; its
// shape is mcp.ToolHandlerFor, so a custom handler can wrap it. A call whose
// arguments already carry an answer under the gate id skips the ask entirely
// — the gate is not an authorization boundary (see package elicit).
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

// ask validates in, then builds the confirmation the client must fulfill.
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
