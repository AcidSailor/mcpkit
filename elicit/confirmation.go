package elicit

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MaxLines is the most message lines Claude Code (2.1.312) shows. A longer
// message shows 3 lines plus "… (+N more lines)" and cannot be expanded.
// Lines are also cut at the terminal width, so put the action, the object
// and any irreversibility first.
const MaxLines = 4

// ParamsFunc builds an elicitation prompt from a decoded request of type In.
type ParamsFunc[In any] func(
	ctx context.Context, in In,
) (*mcp.ElicitParams, error)

// DescribeFunc renders a confirmation message from a decoded In request.
type DescribeFunc[In any] func(ctx context.Context, in In) (string, error)

// emptyObject includes the properties map required by clients.
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

// BoundedConfirmation is DynamicConfirmation that fails with
// ErrPromptOverflow when the message has more lines than limit. The gate
// then aborts before the write runs. It panics with ErrInvalidLineLimit
// when limit is less than 1, because no message could pass.
func BoundedConfirmation[In any](
	limit int,
	describe DescribeFunc[In],
) ParamsFunc[In] {
	if limit < 1 {
		panic(fmt.Errorf("%w: %d", ErrInvalidLineLimit, limit))
	}
	return confirmation(
		func(ctx context.Context, in In) (string, error) {
			message, err := describe(ctx, in)
			if err != nil {
				return "", err
			}
			if err := CheckLines(message, limit); err != nil {
				return "", err
			}
			return message, nil
		},
	)
}

// CheckLines returns ErrPromptOverflow when message has more lines than
// limit.
func CheckLines(message string, limit int) error {
	if n := strings.Count(message, "\n") + 1; n > limit {
		return fmt.Errorf(
			"%w: %d lines, limit %d", ErrPromptOverflow, n, limit,
		)
	}
	return nil
}
