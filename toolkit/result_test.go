package toolkit

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrapItems(t *testing.T) {
	got, err := WrapItems([]int{1, 2}, nil)
	require.NoError(t, err)
	b, err := json.Marshal(got)
	require.NoError(t, err)
	assert.JSONEq(t, `{"items":[1,2]}`, string(b))
}

func TestWrapItemsNilNormalizedToEmptyArray(t *testing.T) {
	got, err := WrapItems[int](nil, nil)
	require.NoError(t, err)
	b, err := json.Marshal(got)
	require.NoError(t, err)
	// Nil slices encode as arrays, not null.
	assert.JSONEq(t, `{"items":[]}`, string(b))
}

func TestItemsZeroValueMarshalsEmptyArray(t *testing.T) {
	var it Items[int] // The zero value contains a nil slice.
	b, err := json.Marshal(it)
	require.NoError(t, err)
	// A zero-value envelope also encodes an array.
	assert.JSONEq(t, `{"items":[]}`, string(b))
}

func TestWrapItemsPassesErrorThrough(t *testing.T) {
	sentinel := errors.New("boom")
	_, err := WrapItems[int](nil, sentinel)
	assert.ErrorIs(t, err, sentinel)
}

func TestWrapValue(t *testing.T) {
	got, err := WrapValue(42.5, nil)
	require.NoError(t, err)
	b, err := json.Marshal(got)
	require.NoError(t, err)
	assert.JSONEq(t, `{"value":42.5}`, string(b))
}

func TestWrapValuePassesErrorThrough(t *testing.T) {
	sentinel := errors.New("boom")
	_, err := WrapValue(0, sentinel)
	assert.ErrorIs(t, err, sentinel)
}
