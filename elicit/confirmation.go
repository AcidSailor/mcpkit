package elicit

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Limits bound a confirmation message to what the user can see.
type Limits struct {
	// Lines is the most message lines the client shows.
	Lines int
	// Width caps line length in runes, not terminal columns.
	Width int
}

// DefaultLimits returns the default prompt line and width limits.
func DefaultLimits() Limits {
	return Limits{Lines: 4, Width: 100}
}

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

// BoundedConfirmation rejects prompts exceeding the supplied limits.
func BoundedConfirmation[In any](
	limits Limits,
	describe DescribeFunc[In],
) ParamsFunc[In] {
	if err := limits.validate(); err != nil {
		panic(err)
	}
	return confirmation(
		func(ctx context.Context, in In) (string, error) {
			message, err := describe(ctx, in)
			if err != nil {
				return "", err
			}
			if err := CheckPrompt(message, limits); err != nil {
				return "", err
			}
			return message, nil
		},
	)
}

// CheckPrompt validates message size and limits.
func CheckPrompt(message string, limits Limits) error {
	if err := limits.validate(); err != nil {
		return err
	}
	if n := strings.Count(message, "\n") + 1; n > limits.Lines {
		return fmt.Errorf(
			"%w: %d lines, limit %d", ErrPromptOverflow, n, limits.Lines,
		)
	}
	i := 0
	for line := range strings.SplitSeq(message, "\n") {
		i++
		if n := utf8.RuneCountInString(line); n > limits.Width {
			return fmt.Errorf(
				"%w: line %d is %d runes, limit %d",
				ErrPromptOverflow, i, n, limits.Width,
			)
		}
	}
	return nil
}

func (l Limits) validate() error {
	if l.Lines < 1 || l.Width < 1 {
		return fmt.Errorf("%w: %+v", ErrInvalidLimits, l)
	}
	return nil
}
