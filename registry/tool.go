package registry

import (
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/acidsailor/mcpkit/toolkit"
)

// options holds the optional toolkit config captured by Read/Write.
type options[In any] struct {
	output   *jsonschema.Schema
	validate toolkit.ValidateFunc[In]
	elicit   toolkit.ElicitParamsFunc[In]
}

// Option configures a Read/Write registration; In is usually inferred.
type Option[In any] func(*options[In])

// WithOutputSchema sets the tool's optional output schema (pin In if alone).
func WithOutputSchema[In any](s *jsonschema.Schema) Option[In] {
	return func(o *options[In]) { o.output = s }
}

// WithValidateFunc sets a validator run on decoded input before the call.
func WithValidateFunc[In any](f toolkit.ValidateFunc[In]) Option[In] {
	return func(o *options[In]) { o.validate = f }
}

// WithElicitFunc sets a write tool's elicit-prompt builder; on Read panics.
func WithElicitFunc[In any](f toolkit.ElicitParamsFunc[In]) Option[In] {
	return func(o *options[In]) { o.elicit = f }
}

// Read describes a read-only tool. In/Out are inferred from call.
func Read[In, Out any](
	name, description string,
	in *jsonschema.Schema,
	call toolkit.CallFunc[In, Out],
	opts ...Option[In],
) Registration {
	return Registration{
		Name:   name,
		Access: AccessRead,
		bind: func(s *mcp.Server) {
			toolkit.AddRead(build(s, name, description, in, call, opts))
		},
	}
}

// Write describes a state-mutating tool gated by elicitation; In/Out inferred.
func Write[In, Out any](
	name, description string,
	in *jsonschema.Schema,
	call toolkit.CallFunc[In, Out],
	opts ...Option[In],
) Registration {
	return Registration{
		Name:   name,
		Access: AccessWrite,
		bind: func(s *mcp.Server) {
			toolkit.AddWrite(build(s, name, description, in, call, opts))
		},
	}
}

// build applies opts onto a fresh toolkit.Tool via the fluent chain.
func build[In, Out any](
	s *mcp.Server,
	name, description string,
	in *jsonschema.Schema,
	call toolkit.CallFunc[In, Out],
	opts []Option[In],
) toolkit.Tool[In, Out] {
	var o options[In]
	for _, opt := range opts {
		opt(&o)
	}
	t := toolkit.New(s, name, description, in, call)
	if o.output != nil {
		t = t.WithOutputSchema(o.output)
	}
	if o.validate != nil {
		t = t.WithValidateFunc(o.validate)
	}
	if o.elicit != nil {
		t = t.WithElicitParamsFunc(o.elicit)
	}
	return t
}
