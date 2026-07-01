package resource

import (
	"errors"
	"fmt"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yosida95/uritemplate/v3"
)

// mustVars matches uri against tmpl and wraps the extracted values, failing the
// test if the URI does not match.
func mustVars(t *testing.T, tmpl, uri string) Vars {
	t.Helper()
	parsed, err := uritemplate.New(tmpl)
	require.NoError(t, err)
	vals := parsed.Match(uri)
	require.NotNil(t, vals)
	return Vars{vals: vals}
}

func TestVars_IntWrapsSentinel(t *testing.T) {
	v := mustVars(t, "items://{id}", "items://abc")
	_, err := v.Int("id")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidVars)
}

func TestVars_LookupAbsent(t *testing.T) {
	v := mustVars(t, "items://{id}", "items://7")
	got, ok := v.Lookup("id")
	assert.Equal(t, "7", got)
	assert.True(t, ok)

	_, ok = v.Lookup("missing")
	assert.False(t, ok)
	assert.Equal(t, "", v.Get("missing"))
}

func TestToWireErr_MapsSentinels(t *testing.T) {
	// Both not-found sentinels map to the SDK's typed not-found error, even
	// when wrapped with context via %w (as the handlers do).
	for _, sentinel := range []error{ErrNotFound, ErrTemplateMismatch} {
		wrapped := fmt.Errorf("name: %w", sentinel)
		got := toWireErr("u://x", wrapped)
		assert.Equal(t, mcp.ResourceNotFoundError("u://x"), got)
	}

	// A non-sentinel error passes through unchanged, and nil stays nil.
	other := errors.New("boom")
	assert.Equal(t, other, toWireErr("u://x", other))
	assert.Nil(t, toWireErr("u://x", nil))
}

func TestRaw_EmptyYieldsNoContent(t *testing.T) {
	_, err := Raw{}.contents("u", "")
	assert.ErrorIs(t, err, ErrNoContent)

	_, err = Raw{Contents: nil}.contents("u", "")
	assert.ErrorIs(t, err, ErrNoContent)
}
