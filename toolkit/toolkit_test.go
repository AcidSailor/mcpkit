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

	_, err := tl.runValidated(context.Background(), echoIn{}, nil)
	require.ErrorIs(t, err, validate.ErrEmpty)
}

func TestMCPToolOutputSchema(t *testing.T) {
	tl := New(nil, "n", "d", objectSchema(),
		func(_ context.Context, in echoIn) (echoOut, error) {
			return echoOut{Echo: in.Msg}, nil
		})

	// Without WithOutputSchema, OutputSchema must be an untyped nil interface.
	tool := tl.mcpTool(&mcp.ToolAnnotations{})
	require.True(
		t,
		tool.OutputSchema == nil,
		"unset output schema must be an untyped nil interface",
	)

	tool = tl.WithOutputSchema(objectSchema()).mcpTool(&mcp.ToolAnnotations{})
	require.NotNil(t, tool.OutputSchema)
}
