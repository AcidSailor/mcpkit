package toolkit

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AddRead registers a read-only tool; panics if an elicit prompt was set.
func AddRead[In, Out any](t Tool[In, Out]) {
	AddReadFunc(t, t.Call)
}

// AddReadFunc registers a read-only tool running callFunc as-is, unvalidated.
func AddReadFunc[In, Out any](
	t Tool[In, Out],
	callFunc mcp.ToolHandlerFor[In, Out],
) {
	if t.elicitParamsFunc != nil {
		panic(t.wrap(ErrElicitOnRead))
	}

	mcp.AddTool(
		t.server,
		t.mcpTool(true),
		callFunc,
	)
}

// Call runs the validator then the call func, shaped as a tool handler so a
// custom handler passed to AddReadFunc / AddWriteFunc can reuse it.
func (t Tool[In, Out]) Call(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in In,
) (*mcp.CallToolResult, Out, error) {
	out, err := t.callValidated(ctx, in)
	return nil, out, err
}
