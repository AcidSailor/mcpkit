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
