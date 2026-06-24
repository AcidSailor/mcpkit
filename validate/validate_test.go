package validate_test

import (
	"testing"

	"github.com/acidsailor/mcpkit/validate"
	"github.com/stretchr/testify/require"
)

func TestRequireNonEmpty(t *testing.T) {
	// Errors when value is blank, naming the field.
	require.NoError(t, validate.RequireNonEmpty("id", "x"))

	err := validate.RequireNonEmpty("id", "")
	require.Error(t, err)
	require.ErrorIs(t, err, validate.ErrEmpty)

	require.Error(t, validate.RequireNonEmpty("id", "   "))

	require.ErrorContains(
		t,
		validate.RequireNonEmpty("account_id", " "),
		"account_id",
	)
}

func TestRequireNonZero(t *testing.T) {
	// Errors when value is the zero value, naming the field.
	require.NoError(t, validate.RequireNonZero("order_id", int64(42)))

	err := validate.RequireNonZero("order_id", int64(0))
	require.Error(t, err)
	require.ErrorIs(t, err, validate.ErrZero)
	require.ErrorContains(t, err, "order_id")

	// Works for any comparable type.
	require.NoError(t, validate.RequireNonZero("n", int32(1)))
	require.Error(t, validate.RequireNonZero("n", int32(0)))
}
