package registry

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestBindGatesWritesOnEnableWrite(t *testing.T) {
	var bound []string
	reg := Registry{
		{Name: "r", Access: AccessRead, bind: func(*mcp.Server) {
			bound = append(bound, "r")
		}},
		{
			Name: "w", Access: AccessWrite, bind: func(*mcp.Server) {
				bound = append(bound, "w")
			},
		},
	}

	reg.Bind(nil, Enable{Write: false})
	require.Equal(t, []string{"r"}, bound)

	bound = nil
	reg.Bind(nil, Enable{Write: true})
	require.Equal(t, []string{"r", "w"}, bound)
}

func TestBindPanicsOnNilBind(t *testing.T) {
	reg := Registry{{Name: "x"}}
	require.Panics(t, func() { reg.Bind(nil, Enable{}) })
}
