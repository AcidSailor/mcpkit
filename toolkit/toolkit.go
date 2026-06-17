package toolkit

import (
	"context"
	"fmt"

	"github.com/acidsailor/mcpkit/elicit"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Re-exported so callers can match elicit sentinels without importing elicit.
var (
	ErrUserDeclined           = elicit.ErrUserDeclined
	ErrUserCanceled           = elicit.ErrUserCanceled
	ErrNoElicitation          = elicit.ErrNoElicitation
	ErrUnexpectedElicitAction = elicit.ErrUnexpectedElicitAction
	ErrElicitationFailed      = elicit.ErrElicitationFailed
)

type (
	// CallFunc is the function a tool invokes.
	CallFunc[In, Out any] func(ctx context.Context, in In) (Out, error)
	// ValidateFunc validates decoded input before the call.
	ValidateFunc[In any] func(ctx context.Context, in In) error
	// ElicitParamsFunc builds the elicitation prompt for a write tool.
	ElicitParamsFunc[In any] = elicit.ParamsFunc[In]
)

// Tool is a fluent registration builder, distinct from the SDK's mcp.Tool.
type Tool[In, Out any] struct {
	server           *mcp.Server
	name             string
	description      string
	callFunc         CallFunc[In, Out]
	inputSchema      *jsonschema.Schema
	outputSchema     *jsonschema.Schema
	validateFunc     ValidateFunc[In]
	elicitParamsFunc ElicitParamsFunc[In]
}

// New starts a tool registration, inferring In/Out from call.
func New[In, Out any](
	server *mcp.Server,
	name, description string,
	inputSchema *jsonschema.Schema,
	call CallFunc[In, Out],
) Tool[In, Out] {
	return Tool[In, Out]{
		server:      server,
		name:        name,
		description: description,
		callFunc:    call,
		inputSchema: inputSchema,
	}
}

// WithOutputSchema sets the optional output schema the SDK validates against.
func (t Tool[In, Out]) WithOutputSchema(
	schema *jsonschema.Schema,
) Tool[In, Out] {
	t.outputSchema = schema
	return t
}

// WithValidateFunc sets a validator run on decoded input before the call.
func (t Tool[In, Out]) WithValidateFunc(f ValidateFunc[In]) Tool[In, Out] {
	t.validateFunc = f
	return t
}

// WithElicitParamsFunc sets the write tool's elicitation prompt builder.
func (t Tool[In, Out]) WithElicitParamsFunc(
	f ElicitParamsFunc[In],
) Tool[In, Out] {
	t.elicitParamsFunc = f
	return t
}

// mcpTool builds the SDK tool descriptor; OutputSchema set only when present.
func (t Tool[In, Out]) mcpTool(
	annotations *mcp.ToolAnnotations,
) *mcp.Tool {
	tool := &mcp.Tool{
		Name:        t.name,
		Description: t.description,
		Annotations: annotations,
		InputSchema: t.inputSchema,
	}
	if t.outputSchema != nil {
		tool.OutputSchema = t.outputSchema
	}
	return tool
}

// runValidated runs validator, optional gate, then call; wraps errs with name.
func (t Tool[In, Out]) runValidated(
	ctx context.Context,
	in In,
	gate func() error,
) (Out, error) {
	out, err := func() (Out, error) {
		var out Out
		if t.validateFunc != nil {
			if err := t.validateFunc(ctx, in); err != nil {
				return out, err
			}
		}
		if gate != nil {
			if err := gate(); err != nil {
				return out, err
			}
		}
		return t.callFunc(ctx, in)
	}()
	if err != nil {
		return out, fmt.Errorf("%s: %w", t.name, err)
	}
	return out, nil
}

// ptr returns a pointer to v, used for the SDK's *bool annotation hints. On
// Go 1.26+ this is the built-in new(v).
func ptr[T any](v T) *T {
	return &v
}
