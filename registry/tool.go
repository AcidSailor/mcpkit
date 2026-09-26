package registry

import (
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/acidsailor/mcpkit/toolkit"
)

// options holds the optional toolkit config captured by Read/Write.
type options[In any] struct {
	validate    toolkit.ValidateFunc[In]
	elicit      toolkit.ElicitParamsFunc[In]
	annotations *mcp.ToolAnnotations
	gateID      string
}

// Option configures a Read/Write registration; In is usually inferred.
type Option[In any] func(*options[In])

// WithValidateFunc sets a validator run on decoded input before the call.
func WithValidateFunc[In any](f toolkit.ValidateFunc[In]) Option[In] {
	return func(o *options[In]) { o.validate = f }
}

// WithElicitFunc sets a write tool's elicit-prompt builder; on Read panics.
func WithElicitFunc[In any](f toolkit.ElicitParamsFunc[In]) Option[In] {
	return func(o *options[In]) { o.elicit = f }
}

// WithToolAnnotations replaces tool hints and requires matching access.
func WithToolAnnotations[In any](a mcp.ToolAnnotations) Option[In] {
	return func(o *options[In]) { o.annotations = &a }
}

// WithGateID overrides a write tool's confirmation key; on Read panics at Bind.
func WithGateID[In any](id string) Option[In] {
	return func(o *options[In]) { o.gateID = id }
}

// Read describes a read-only tool. In/Out are inferred from call.
// Nil in/out schemas are reflected from In/Out by the SDK.
func Read[In, Out any](
	name, description string,
	in, out *jsonschema.Schema,
	call toolkit.CallFunc[In, Out],
	opts ...Option[In],
) Registration {
	return Registration{
		Name:   name,
		Access: AccessRead,
		bind: func(s *mcp.Server) {
			toolkit.AddRead(build(s, name, description, in, out, call, opts))
		},
	}
}

// Write describes a state-mutating tool gated by elicitation; In/Out inferred.
// Nil in/out schemas are reflected from In/Out by the SDK.
func Write[In, Out any](
	name, description string,
	in, out *jsonschema.Schema,
	call toolkit.CallFunc[In, Out],
	opts ...Option[In],
) Registration {
	return Registration{
		Name:   name,
		Access: AccessWrite,
		bind: func(s *mcp.Server) {
			toolkit.AddWrite(build(s, name, description, in, out, call, opts))
		},
	}
}

// build applies opts onto a fresh toolkit.Tool via the fluent chain.
func build[In, Out any](
	s *mcp.Server,
	name, description string,
	in, out *jsonschema.Schema,
	call toolkit.CallFunc[In, Out],
	opts []Option[In],
) toolkit.Tool[In, Out] {
	var o options[In]
	for _, opt := range opts {
		opt(&o)
	}
	// Only annotations need a guard: the rest are zero-valued when unset.
	t := toolkit.New(s, name, description, in, call).
		WithOutputSchema(out).
		WithValidateFunc(o.validate).
		WithElicitParamsFunc(o.elicit).
		WithGateID(o.gateID)
	if o.annotations != nil {
		t = t.WithAnnotations(*o.annotations)
	}
	return t
}
