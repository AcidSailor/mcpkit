package toolkit

import (
	"context"
	"fmt"

	"github.com/acidsailor/mcpkit/elicit"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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
	annotations      *mcp.ToolAnnotations
	gateID           string
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

// WithAnnotations sets the tool's hints, used verbatim. ReadOnlyHint must
// match the access category (AddRead / AddWrite) or registration panics.
func (t Tool[In, Out]) WithAnnotations(
	a mcp.ToolAnnotations,
) Tool[In, Out] {
	t.annotations = &a
	return t
}

// WithGateID overrides the key naming the write tool's confirmation request.
// Ask and read must agree on it: a handler reading under a different key never
// sees the answer and re-asks until the SDK's retry cap. Panics on a read.
func (t Tool[In, Out]) WithGateID(id string) Tool[In, Out] {
	t.gateID = id
	return t
}

// gate returns the confirmation request key, defaulting to elicit.GateID.
func (t Tool[In, Out]) gate() string {
	if t.gateID == "" {
		return elicit.GateID
	}
	return t.gateID
}

// annotate returns the category's default hints, or the caller's verbatim once
// checked against the category. Hints are the caller's to own whole: a write
// leaving DestructiveHint unset omits it, which the spec already reads as true.
func (t Tool[In, Out]) annotate(readOnly bool) *mcp.ToolAnnotations {
	if t.annotations == nil {
		return &mcp.ToolAnnotations{
			ReadOnlyHint:    readOnly,
			IdempotentHint:  readOnly,
			DestructiveHint: new(!readOnly),
		}
	}
	a := *t.annotations
	if a.ReadOnlyHint != readOnly {
		panic(t.wrap(ErrReadOnlyMismatch))
	}
	if readOnly && a.DestructiveHint != nil && *a.DestructiveHint {
		panic(t.wrap(ErrDestructiveRead))
	}
	return &a
}

// mcpTool builds the SDK tool descriptor; OutputSchema set only when present.
func (t Tool[In, Out]) mcpTool(readOnly bool) *mcp.Tool {
	tool := &mcp.Tool{
		Name:        t.name,
		Description: t.description,
		Annotations: t.annotate(readOnly),
		InputSchema: t.inputSchema,
	}
	if t.outputSchema != nil {
		tool.OutputSchema = t.outputSchema
	}
	return tool
}

// wrap prefixes err with the tool name, preserving sentinels for errors.Is.
func (t Tool[In, Out]) wrap(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", t.name, err)
}

// wrapHandler names the tool in every error the registered handler returns,
// so the wrap happens once, at the boundary, for custom handlers too.
func (t Tool[In, Out]) wrapHandler(
	h mcp.ToolHandlerFor[In, Out],
) mcp.ToolHandlerFor[In, Out] {
	return func(
		ctx context.Context,
		req *mcp.CallToolRequest,
		in In,
	) (*mcp.CallToolResult, Out, error) {
		res, out, err := h(ctx, req, in)
		return res, out, t.wrap(err)
	}
}

// validate runs the optional validator on decoded input.
func (t Tool[In, Out]) validate(ctx context.Context, in In) error {
	if t.validateFunc == nil {
		return nil
	}
	return t.validateFunc(ctx, in)
}
