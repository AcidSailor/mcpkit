package mcptest_test

import (
	"context"
	"testing"

	"github.com/acidsailor/mcpkit/mcptest"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func pingTool(s *mcp.Server) {
	s.AddTool(
		&mcp.Tool{
			Name:        "ping",
			Description: "ping",
			InputSchema: &jsonschema.Schema{Type: "object"},
		},
		func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			r := &mcp.CallToolResult{}
			r.Content = []mcp.Content{&mcp.TextContent{Text: "pong"}}
			return r, nil
		},
	)
}

func TestNewSessionCallsTool(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	pingTool(s)

	cs := mcptest.NewSession(t, s)

	res, err := cs.CallTool(
		context.Background(),
		&mcp.CallToolParams{Name: "ping"},
	)
	require.NoError(t, err)
	require.False(t, res.IsError)
	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	require.Equal(t, "pong", tc.Text)
}

func TestNewSessionWithElicitationRoutesHandler(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)

	called := false
	cs := mcptest.NewSessionWithElicitation(
		t,
		s,
		func(_ context.Context, _ *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			called = true
			return &mcp.ElicitResult{Action: "accept"}, nil
		},
	)
	require.NotNil(t, cs)

	// Handler is wired, but runs only when the server elicits.
	require.False(t, called, "handler runs only when the server elicits")
}
