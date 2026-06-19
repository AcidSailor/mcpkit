package toolkit

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AddRead registers a read-only MCP tool (ReadOnlyHint=true,
// IdempotentHint=true, DestructiveHint=false). Panics if an elicitation prompt
// was set via WithElicitParamsFunc, which is meaningless for a read tool.
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

// AddReadFunc registers a read-only MCP tool (ReadOnlyHint=true,
// IdempotentHint=true, DestructiveHint=false) with a caller-supplied handler.
// Unlike AddRead, t.runValidated is not applied: callFunc runs exactly as
// given, so the caller owns any validation and result wrapping. Panics if an
// elicitation prompt was set via WithElicitParamsFunc, which is meaningless for
// a read tool. AddRead is the common case; reach for AddReadFunc only when you
// need full control of the handler.
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
			DestructiveHint: new(false),
		},
	)

	mcp.AddTool(
		t.server,
		tool,
		callFunc,
	)
}
