package toolkit

import (
	"context"
	"testing"

	"github.com/acidsailor/mcpkit/elicit"
	"github.com/acidsailor/mcpkit/validate"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

type echoIn struct {
	Msg string `json:"msg"`
}

type echoOut struct {
	Echo string `json:"echo"`
}

// objectSchema is a minimal valid input/output schema for tests.
func objectSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Type: "object"}
}

func TestBuilderStoresFields(t *testing.T) {
	tl := New(nil, "name", "desc", objectSchema(),
		func(_ context.Context, in echoIn) (echoOut, error) {
			return echoOut{Echo: in.Msg}, nil
		}).
		WithOutputSchema(objectSchema()).
		WithValidateFunc(func(_ context.Context, _ echoIn) error { return nil }).
		WithElicitParamsFunc(elicit.SimpleConfirmation[echoIn]("do it?"))

	require.Equal(t, "name", tl.name)
	require.Equal(t, "desc", tl.description)
	require.NotNil(t, tl.callFunc)
	require.NotNil(t, tl.inputSchema)
	require.NotNil(t, tl.outputSchema)
	require.NotNil(t, tl.validateFunc)
	require.NotNil(t, tl.elicitParamsFunc)
}

// A validator's sentinel must stay matchable through the pipeline (errors.Is).
func TestRunValidatedPreservesValidateSentinel(t *testing.T) {
	tl := New(nil, "n", "d", objectSchema(),
		func(_ context.Context, in echoIn) (echoOut, error) {
			return echoOut{Echo: in.Msg}, nil
		}).
		WithValidateFunc(func(_ context.Context, _ echoIn) error {
			return validate.ErrEmpty
		})

	_, err := tl.callValidated(context.Background(), echoIn{})
	require.ErrorIs(t, err, validate.ErrEmpty)
}

func TestMCPToolOutputSchema(t *testing.T) {
	tl := New(nil, "n", "d", objectSchema(),
		func(_ context.Context, in echoIn) (echoOut, error) {
			return echoOut{Echo: in.Msg}, nil
		})

	// Without WithOutputSchema, OutputSchema must be an untyped nil interface.
	tool := tl.mcpTool(true)
	require.True(
		t,
		tool.OutputSchema == nil,
		"unset output schema must be an untyped nil interface",
	)

	tool = tl.WithOutputSchema(objectSchema()).mcpTool(true)
	require.NotNil(t, tool.OutputSchema)
}

// The access category owns ReadOnlyHint; the caller owns the rest.
func TestAnnotateKeepsCallerHints(t *testing.T) {
	tl := New(nil, "n", "d", objectSchema(),
		func(_ context.Context, in echoIn) (echoOut, error) {
			return echoOut{Echo: in.Msg}, nil
		}).
		WithAnnotations(mcp.ToolAnnotations{
			Title:           "Do it",
			IdempotentHint:  true,
			DestructiveHint: new(false),
			OpenWorldHint:   new(true),
		})

	a := tl.annotate(false)
	require.False(t, a.ReadOnlyHint, "a write is never read-only")
	require.Equal(t, "Do it", a.Title)
	require.True(t, a.IdempotentHint, "an idempotent write stays idempotent")
	require.False(t, *a.DestructiveHint, "an additive write is not destructive")
	require.True(t, *a.OpenWorldHint)
}

// Caller hints are returned whole: an unset DestructiveHint stays unset.
func TestAnnotateReturnsCallerHintsVerbatim(t *testing.T) {
	a := mcp.ToolAnnotations{Title: "Look", ReadOnlyHint: true}
	tl := New(nil, "n", "d", objectSchema(),
		func(_ context.Context, in echoIn) (echoOut, error) {
			return echoOut{Echo: in.Msg}, nil
		}).
		WithAnnotations(a)

	got := tl.annotate(true)
	require.Equal(t, &a, got)
	require.NotSame(t, &a, got, "annotate must hand back its own copy")
}

// The contradiction panics at registration, not only in annotate: AddRead and
// AddWrite must actually route through it.
func TestAddReadPanicsOnContradictingAnnotations(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	require.PanicsWithError(t, "n: "+ErrReadOnlyMismatch.Error(), func() {
		AddRead(New(s, "n", "d", objectSchema(),
			func(_ context.Context, in echoIn) (echoOut, error) {
				return echoOut{Echo: in.Msg}, nil
			}).
			WithAnnotations(mcp.ToolAnnotations{Title: "Look"}))
	})
}

// Hints that contradict the access category are programmer errors.
func TestAnnotatePanicsOnContradiction(t *testing.T) {
	tests := []struct {
		name     string
		a        mcp.ToolAnnotations
		readOnly bool
		wantErr  error
	}{
		{
			"read-only write",
			mcp.ToolAnnotations{ReadOnlyHint: true},
			false,
			ErrReadOnlyMismatch,
		},
		{
			"read leaving ReadOnlyHint unset",
			mcp.ToolAnnotations{Title: "Look"},
			true,
			ErrReadOnlyMismatch,
		},
		{
			"destructive read",
			mcp.ToolAnnotations{
				ReadOnlyHint:    true,
				DestructiveHint: new(true),
			},
			true,
			ErrDestructiveRead,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl := New(nil, "n", "d", objectSchema(),
				func(_ context.Context, in echoIn) (echoOut, error) {
					return echoOut{Echo: in.Msg}, nil
				}).
				WithAnnotations(tt.a)

			require.PanicsWithError(
				t,
				"n: "+tt.wantErr.Error(),
				func() { tl.annotate(tt.readOnly) },
			)
		})
	}
}

// Unset hints keep the defaults each category shipped before WithAnnotations.
func TestAnnotateDefaults(t *testing.T) {
	tl := New(nil, "n", "d", objectSchema(),
		func(_ context.Context, in echoIn) (echoOut, error) {
			return echoOut{Echo: in.Msg}, nil
		})

	read := tl.annotate(true)
	require.True(t, read.ReadOnlyHint)
	require.True(t, read.IdempotentHint)
	require.False(t, *read.DestructiveHint)

	write := tl.annotate(false)
	require.False(t, write.ReadOnlyHint)
	require.False(t, write.IdempotentHint)
	require.True(t, *write.DestructiveHint, "writes default to destructive")
}

// The gate key defaults to elicit.GateID and WithGateID overrides it.
func TestGateID(t *testing.T) {
	tl := New(nil, "n", "d", objectSchema(),
		func(_ context.Context, in echoIn) (echoOut, error) {
			return echoOut{Echo: in.Msg}, nil
		})

	require.Equal(t, elicit.GateID, tl.gate())
	require.Equal(t, "acme/confirm", tl.WithGateID("acme/confirm").gate())
}
