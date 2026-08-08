package toolkit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/acidsailor/mcpkit/elicit"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// statelessSession serves s over a stateless streamable HTTP handler and
// connects a real HTTP client to it, wiring elicit as its handler.
func statelessSession(
	t *testing.T,
	s *mcp.Server,
	elicitHandler func(
		context.Context, *mcp.ElicitRequest,
	) (*mcp.ElicitResult, error),
) *mcp.ClientSession {
	t.Helper()
	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return s },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true},
	)
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	client := mcp.NewClient(
		&mcp.Implementation{Name: "c", Version: "0"},
		&mcp.ClientOptions{ElicitationHandler: elicitHandler},
	)
	cs, err := client.Connect(
		t.Context(),
		&mcp.StreamableClientTransport{Endpoint: httpServer.URL},
		nil,
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// The headline claim of the v1.7.0 migration: a stateless HTTP handler serves
// an elicitation-gated write, because the retry needs no retained session.
func TestAddWrite_StatelessHTTP(t *testing.T) {
	called := false
	prompted := ""
	s := writeServer(t, &called)

	cs := statelessSession(t, s, func(
		_ context.Context,
		req *mcp.ElicitRequest,
	) (*mcp.ElicitResult, error) {
		prompted = req.Params.Message
		return &mcp.ElicitResult{Action: "accept"}, nil
	})

	require.Equal(
		t,
		"2026-07-28",
		cs.InitializeResult().ProtocolVersion,
		"stateless is a precondition for the multi-round-trip protocol",
	)

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "do",
		Arguments: map[string]any{"msg": "hi"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError, errorTextOrEmpty(res))
	assert.True(t, called, "a stateless handler must serve gated writes")
	assert.Equal(t, "confirm?", prompted, "the prompt must reach the client")
}

// Without the capability the gate still refuses, stateless or not.
func TestAddWrite_StatelessHTTPNoElicitation(t *testing.T) {
	called := false
	s := writeServer(t, &called)

	cs := statelessSession(t, s, nil) // no ElicitationHandler

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "do",
		Arguments: map[string]any{"msg": "hi"},
	})
	require.NoError(t, err)
	require.True(t, res.IsError, "no capability must not silently write")
	assert.Contains(t, errorText(t, res), elicit.ErrNoElicitation.Error())
	assert.False(t, called, "the write must not run")
}

// A read tool needs no session either, guarding the same stateless path.
func TestAddRead_StatelessHTTP(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	AddRead(New(s, "echo", "echoes", objectSchema(),
		func(_ context.Context, in echoIn) (echoOut, error) {
			return echoOut{Echo: in.Msg}, nil
		}))

	cs := statelessSession(t, s, nil)
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "echo",
		Arguments: map[string]any{"msg": "hi"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError, errorTextOrEmpty(res))
}
