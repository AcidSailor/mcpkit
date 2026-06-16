package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTransport_Valid(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Transport
	}{
		{"http", HTTP},
		{"stdio", Stdio},
		{"both", Both},
	} {
		got, err := ParseTransport(tc.in)
		require.NoError(t, err)
		assert.Equal(t, tc.want, got)
	}
}

func TestParseTransport_Invalid(t *testing.T) {
	_, err := ParseTransport("ftp")
	require.Error(t, err)
}

func TestTransportUnmarshalText_Valid(t *testing.T) {
	var m Transport
	require.NoError(t, m.UnmarshalText([]byte("both")))
	assert.Equal(t, Both, m)
}

func TestTransportUnmarshalText_Invalid(t *testing.T) {
	var m Transport
	require.Error(t, m.UnmarshalText([]byte("ftp")))
}
