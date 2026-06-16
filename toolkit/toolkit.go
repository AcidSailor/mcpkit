// Package toolkit provides a generic fluent builder for registering JSON MCP
// tools on a server.
package toolkit

import (
	"context"
	"fmt"

	"github.com/acidsailor/mcpkit/elicit"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Re-exported so callers can match elicitation sentinels without importing
// elicit.
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

// Tool is a fluent registration builder. New infers In/Out from call, so the
// type name is rarely written at call sites. Distinct from the SDK's mcp.Tool,
// which it produces internally.
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

// New starts a tool registration, inferring In/Out from call. The input schema
// is required (the SDK panics on nil); the output schema is optional, set via
// WithOutputSchema.
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

// WithOutputSchema sets the optional output schema. When set, the SDK validates
// the structured result against it; when unset, the tool declares none.
func (t Tool[In, Out]) WithOutputSchema(
	schema *jsonschema.Schema,
) Tool[In, Out] {
	t.outputSchema = schema
	return t
}

// WithValidateFunc sets an optional validator run on decoded input before the
// call (and, for writes, before elicitation).
func (t Tool[In, Out]) WithValidateFunc(f ValidateFunc[In]) Tool[In, Out] {
	t.validateFunc = f
	return t
}

// WithElicitParamsFunc sets the function that builds a write tool's elicitation
// prompt. Optional for AddWrite; AddRead panics if it is set.
func (t Tool[In, Out]) WithElicitParamsFunc(
	f ElicitParamsFunc[In],
) Tool[In, Out] {
	t.elicitParamsFunc = f
	return t
}

// mcpTool builds the SDK tool descriptor with the given annotations. The SDK's
// schema fields are typed `any`: assigning a nil *jsonschema.Schema would wrap
// a typed-nil into a non-nil interface, which the SDK then rejects as a zero
// schema lacking type "object". So OutputSchema is set only when present.
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

// runValidated is the shared handler pipeline for AddRead/AddWrite: validator
// (if any), then an optional gate (writes pass the elicitation gate, reads pass
// nil), then the call, wrapping any failure with the tool name while preserving
// the underlying sentinel. One place keeps the read/write paths aligned.
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
