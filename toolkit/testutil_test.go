package toolkit

import (
	"context"
	"testing"

	"github.com/acidsailor/mcpkit/mcptest"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newTestMCPSession(t *testing.T, s *mcp.Server) *mcp.ClientSession {
	t.Helper()
	return mcptest.NewSession(t, s)
}

func newTestMCPSessionWithElicitation(
	t *testing.T,
	s *mcp.Server,
	handler func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error),
) *mcp.ClientSession {
	t.Helper()
	return mcptest.NewSessionWithElicitation(t, s, handler)
}
