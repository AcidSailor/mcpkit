package elicit_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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

func TestCheckPrompt(t *testing.T) {
	long := strings.Repeat("я", 101)
	tests := []struct {
		name    string
		message string
		wantErr string
	}{
		{name: "single line", message: "Delete cal?"},
		{name: "at line limit", message: "a\nb\nc\nd"},
		{
			name:    "over line limit",
			message: "a\nb\nc\nd\ne",
			wantErr: "5 lines, limit 4",
		},
		{
			name:    "trailing newline",
			message: "a\nb\nc\nd\n",
			wantErr: "5 lines, limit 4",
		},
		{name: "at width limit", message: strings.Repeat("я", 100)},
		{
			name:    "over width limit",
			message: "Delete cal?\n" + long,
			wantErr: "line 2 is 101 runes, limit 100",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := elicit.CheckPrompt(tt.message, elicit.DefaultLimits)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, elicit.ErrPromptOverflow)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestBoundedConfirmationBuildsMessageWithinLimits(t *testing.T) {
	pf := elicit.BoundedConfirmation(elicit.Limits{Lines: 2, Width: 22},
		func(_ context.Context, in confirmInput) (string, error) {
			return "Delete " + in.Name + "?\nThis cannot be undone.", nil
		},
	)

	params, err := pf(context.Background(), confirmInput{Name: "cal"})
	require.NoError(t, err)
	require.Equal(t, "Delete cal?\nThis cannot be undone.", params.Message)
}

func TestBoundedConfirmationRejectsOverflow(t *testing.T) {
	describe := func(_ context.Context, _ confirmInput) (string, error) {
		return "Delete cal?\nThis cannot be undone.", nil
	}
	for _, limits := range []elicit.Limits{
		{Lines: 1, Width: 100},
		{Lines: 4, Width: 21},
	} {
		params, err := elicit.BoundedConfirmation(limits, describe)(
			context.Background(), confirmInput{},
		)
		require.Nil(t, params)
		require.ErrorIs(t, err, elicit.ErrPromptOverflow)
	}
}

func TestBoundedConfirmationPropagatesDescribeError(t *testing.T) {
	sentinel := errors.New("boom")
	pf := elicit.BoundedConfirmation(elicit.DefaultLimits,
		func(_ context.Context, _ confirmInput) (string, error) {
			return "", sentinel
		},
	)

	params, err := pf(context.Background(), confirmInput{})
	require.Nil(t, params)
	require.ErrorIs(t, err, sentinel)
}

func TestBoundedConfirmationPanicsOnInvalidLimits(t *testing.T) {
	describe := func(_ context.Context, _ confirmInput) (string, error) {
		return "Delete cal?", nil
	}
	for _, limits := range []elicit.Limits{
		{Lines: 0, Width: 100},
		{Lines: 4, Width: 0},
		{Lines: -1, Width: -1},
	} {
		require.PanicsWithError(t,
			fmt.Sprintf("prompt limits must be at least 1: %+v", limits),
			func() { elicit.BoundedConfirmation(limits, describe) },
		)
	}
}
