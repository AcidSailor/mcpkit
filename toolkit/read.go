package toolkit

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AddRead registers a validated read-only tool.
func AddRead[In, Out any](t Tool[In, Out]) {
	AddReadFunc(t, t.Call)
}

// AddReadFunc registers an unvalidated read-only tool handler.
func AddReadFunc[In, Out any](
	t Tool[In, Out],
	callFunc mcp.ToolHandlerFor[In, Out],
) {
	if t.elicitParamsFunc != nil {
		panic(t.wrap(ErrElicitOnRead))
	}
	if t.gateID != "" {
		panic(t.wrap(ErrGateIDOnRead))
	}

	mcp.AddTool(
		t.server,
		t.mcpTool(true),
		t.wrapHandler(callFunc),
	)
}

// Call validates the input and invokes the tool function.
func (t Tool[In, Out]) Call(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in In,
) (*mcp.CallToolResult, Out, error) {
	var out Out
	if err := t.validate(ctx, in); err != nil {
		return nil, out, err
	}
	out, err := t.callFunc(ctx, in)
	return nil, out, err
}
