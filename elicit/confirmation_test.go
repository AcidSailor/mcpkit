package elicit_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/acidsailor/mcpkit/elicit"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/require"
)

type confirmInput struct {
	Name string
}

func TestDynamicConfirmationBuildsMessageFromInput(t *testing.T) {
	// describe turns input into the prompt message; no requested fields.
	pf := elicit.DynamicConfirmation(
		func(_ context.Context, in confirmInput) (string, error) {
			return "delete " + in.Name + "?", nil
		},
	)

	params, err := pf(context.Background(), confirmInput{Name: "cal"})
	require.NoError(t, err)
	require.Equal(t, "delete cal?", params.Message)
	schema, ok := params.RequestedSchema.(*jsonschema.Schema)
	require.True(t, ok)
	require.Equal(t, "object", schema.Type)
	require.Empty(t, schema.Properties, "confirmation requests no input fields")

	// Non-nil map serialises "properties":{}; a nil map omits it.
	require.NotNil(t, schema.Properties,
		"properties must be a non-nil empty map, not omitted")
	raw, err := json.Marshal(schema)
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"object","properties":{}}`, string(raw),
		"requestedSchema must serialise properties as an empty object")
}

func TestDynamicConfirmationPropagatesDescribeError(t *testing.T) {
	sentinel := errors.New("boom")
	pf := elicit.DynamicConfirmation(
		func(_ context.Context, _ confirmInput) (string, error) {
			return "", sentinel
		},
	)

	params, err := pf(context.Background(), confirmInput{})
	require.Nil(t, params)
	require.ErrorIs(t, err, sentinel)
}
