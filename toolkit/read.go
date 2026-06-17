package toolkit

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AddRead registers a read-only tool; panics if an elicit prompt was set.
func AddRead[In, Out any](t Tool[In, Out]) {
	AddReadFunc(
		t,
		func(
			ctx context.Context,
			_ *mcp.CallToolRequest,
			in In,
		) (*mcp.CallToolResult, Out, error) {
			out, err := t.runValidated(ctx, in, nil)
			return nil, out, err
		},
	)
}

// AddReadFunc registers a read-only tool running callFunc as-is, unvalidated.
func AddReadFunc[In, Out any](
	t Tool[In, Out],
	callFunc mcp.ToolHandlerFor[In, Out],
) {
	if t.elicitParamsFunc != nil {
		panic(
			fmt.Errorf(
				"%s: elicitation set on a read-only tool",
				t.name,
			),
		)
	}
	tool := t.mcpTool(
		&mcp.ToolAnnotations{
			ReadOnlyHint:    true,
			IdempotentHint:  true,
			DestructiveHint: ptr(false),
		},
	)

	mcp.AddTool(
		t.server,
		tool,
		callFunc,
	)
}
