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
	// describe builds the prompt without requested fields.
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

	// A non-nil map encodes the required empty properties object.
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

func TestCheck(t *testing.T) {
	tests := []struct {
		name    string
		message string
		wantErr bool
	}{
		{name: "single line", message: "Delete cal?"},
		{name: "at limit", message: "a\nb\nc\nd"},
		{name: "over limit", message: "a\nb\nc\nd\ne", wantErr: true},
		{name: "trailing newline", message: "a\nb\nc\nd\n", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := elicit.Check(tt.message, elicit.MaxLines)
			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, elicit.ErrPromptOverflow)
			require.ErrorContains(t, err, "5 lines, limit 4")
		})
	}
}

func TestBoundedConfirmationBuildsMessageWithinLimit(t *testing.T) {
	pf := elicit.BoundedConfirmation(2,
		func(_ context.Context, in confirmInput) (string, error) {
			return "Delete " + in.Name + "?\nThis cannot be undone.", nil
		},
	)

	params, err := pf(context.Background(), confirmInput{Name: "cal"})
	require.NoError(t, err)
	require.Equal(t, "Delete cal?\nThis cannot be undone.", params.Message)
}

func TestBoundedConfirmationRejectsOverflow(t *testing.T) {
	pf := elicit.BoundedConfirmation(1,
		func(_ context.Context, _ confirmInput) (string, error) {
			return "Delete cal?\nThis cannot be undone.", nil
		},
	)

	params, err := pf(context.Background(), confirmInput{})
	require.Nil(t, params)
	require.ErrorIs(t, err, elicit.ErrPromptOverflow)
}

func TestBoundedConfirmationPropagatesDescribeError(t *testing.T) {
	sentinel := errors.New("boom")
	pf := elicit.BoundedConfirmation(elicit.MaxLines,
		func(_ context.Context, _ confirmInput) (string, error) {
			return "", sentinel
		},
	)

	params, err := pf(context.Background(), confirmInput{})
	require.Nil(t, params)
	require.ErrorIs(t, err, sentinel)
}
