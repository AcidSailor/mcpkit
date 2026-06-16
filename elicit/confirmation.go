package elicit

import (
	"context"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ParamsFunc builds an elicitation prompt from a decoded request of type In.
type ParamsFunc[In any] func(
	ctx context.Context, in In,
) (*mcp.ElicitParams, error)

// DescribeFunc renders a confirmation message from a decoded request of type
// In. A returned error aborts before any prompt is shown.
type DescribeFunc[In any] func(ctx context.Context, in In) (string, error)

func confirmation[In any](describe DescribeFunc[In]) ParamsFunc[In] {
	return func(ctx context.Context, in In) (*mcp.ElicitParams, error) {
		message, err := describe(ctx, in)
		if err != nil {
			return nil, err
		}
		return &mcp.ElicitParams{
			Message: message,
			// No requested fields: a confirmation is a pure
			// accept/decline/cancel decision, which Gate reads from the
			// action. An empty object schema asks the client for no input
			// beyond that choice.
			RequestedSchema: &jsonschema.Schema{Type: "object"},
		}, nil
	}
}

// SimpleConfirmation returns a ParamsFunc that prompts with message and
// requests no input fields, for write tools needing a plain yes/no
// confirmation. Gate gates on the accept/decline/cancel action.
func SimpleConfirmation[In any](message string) ParamsFunc[In] {
	return confirmation(
		func(ctx context.Context, in In) (string, error) {
			return message, nil
		},
	)
}

// DynamicConfirmation returns a ParamsFunc that builds the prompt message from
// the decoded input via describe, using SimpleConfirmation's requested schema.
// Use it for write tools whose confirmation text depends on the request (e.g.
// naming the affected resource). A describe error aborts the prompt and is
// returned as-is.
func DynamicConfirmation[In any](describe DescribeFunc[In]) ParamsFunc[In] {
	return confirmation(describe)
}
