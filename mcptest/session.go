// Package mcptest drives a server over the SDK's in-memory transport for tests.
package mcptest

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewSession connects an in-memory client to s, advertising no elicitation.
func NewSession(tb testing.TB, s *mcp.Server) *mcp.ClientSession {
	tb.Helper()
	return connect(tb, s, nil)
}

// NewSessionWithElicitation is like NewSession but wires an elicit handler.
func NewSessionWithElicitation(
	tb testing.TB,
	s *mcp.Server,
	handler func(
		context.Context, *mcp.ElicitRequest,
	) (*mcp.ElicitResult, error),
) *mcp.ClientSession {
	tb.Helper()
	return connect(tb, s, &mcp.ClientOptions{ElicitationHandler: handler})
}

func connect(
	tb testing.TB,
	s *mcp.Server,
	opts *mcp.ClientOptions,
) *mcp.ClientSession {
	tb.Helper()
	ct, st := mcp.NewInMemoryTransports()

	ctx := context.Background()
	go func() {
		if _, err := s.Connect(ctx, st, nil); err != nil {
			tb.Errorf("server-side MCP Connect failed: %v", err)
		}
	}()

	client := mcp.NewClient(
		&mcp.Implementation{Name: "test", Version: "0"},
		opts,
	)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		tb.Fatalf("client-side MCP Connect failed: %v", err)
	}
	tb.Cleanup(func() { _ = cs.Close() })
	return cs
}
