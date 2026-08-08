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

// DescribeFunc renders a confirmation message from a decoded In request.
type DescribeFunc[In any] func(ctx context.Context, in In) (string, error)

// emptyObject is the schema a fieldless confirmation must carry: clients
// reject an omitted requestedSchema.properties (e.g. Claude Code).
func emptyObject() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:       "object",
		Properties: map[string]*jsonschema.Schema{},
	}
}

func confirmation[In any](describe DescribeFunc[In]) ParamsFunc[In] {
	return func(ctx context.Context, in In) (*mcp.ElicitParams, error) {
		message, err := describe(ctx, in)
		if err != nil {
			return nil, err
		}
		return &mcp.ElicitParams{
			Message:         message,
			RequestedSchema: emptyObject(),
		}, nil
	}
}

// SimpleConfirmation returns a ParamsFunc that prompts with message, no fields.
func SimpleConfirmation[In any](message string) ParamsFunc[In] {
	return confirmation(
		func(ctx context.Context, in In) (string, error) {
			return message, nil
		},
	)
}

// DynamicConfirmation returns a ParamsFunc building the prompt via describe.
func DynamicConfirmation[In any](describe DescribeFunc[In]) ParamsFunc[In] {
	return confirmation(describe)
}
