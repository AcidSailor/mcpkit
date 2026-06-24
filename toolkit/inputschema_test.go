package toolkit

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInputSchema(t *testing.T) {
	// InputSchema derives an object schema from a plain struct input.
	s := InputSchema[echoIn]()
	require.NotNil(t, s)
	require.Equal(t, "object", s.Type)
	require.Contains(t, s.Properties, "msg")
}
