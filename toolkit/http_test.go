package toolkit

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/acidsailor/mcpkit/elicit"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// statelessSession connects an HTTP client to a stateless handler.
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

// A stateless HTTP handler supports a gated write without session state.
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
	require.False(t, res.IsError, errorText(res))
	assert.True(t, called, "a stateless handler must serve gated writes")
	assert.Equal(t, "confirm?", prompted, "the prompt must reach the client")
}

// A stateless client without elicitation cannot run a gated write.
func TestAddWrite_StatelessHTTPNoElicitation(t *testing.T) {
	called := false
	s := writeServer(t, &called)

	cs := statelessSession(t, s, nil) // No elicitation handler.

	_, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "do",
		Arguments: map[string]any{"msg": "hi"},
	})
	require.Error(t, err, "an unanswerable gate must fail the call")
	assert.Contains(t, err.Error(), "client does not support elicitation")
	assert.False(t, called, "the write must not run")
}

// newProtocolCall posts one tools/call carrying the given capabilities.
func newProtocolCall(
	t *testing.T,
	url string,
	capabilities map[string]any,
) map[string]any {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"_meta": map[string]any{
				"io.modelcontextprotocol/protocolVersion":    "2026-07-28",
				"io.modelcontextprotocol/clientCapabilities": capabilities,
			},
			"name":      "do",
			"arguments": map[string]any{"msg": "hi"},
		},
	})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(
		t.Context(), http.MethodPost, url, bytes.NewReader(body),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Protocol-Version", "2026-07-28")
	req.Header.Set("Mcp-Method", "tools/call")
	req.Header.Set("Mcp-Name", "do")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	return got
}

// A modern client need not declare elicitation to receive the gate.
func TestAddWrite_StatelessHTTPNoDeclaredCapability(t *testing.T) {
	called := false
	s := writeServer(t, &called)

	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return s },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true},
	)
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	got := newProtocolCall(t, httpServer.URL, map[string]any{})

	result, ok := got["result"].(map[string]any)
	require.True(t, ok, "want a result, got %v", got)
	assert.NotEqual(t, true, result["isError"], "must not refuse the client")

	requests, ok := result["inputRequests"].(map[string]any)
	require.True(t, ok, "want inputRequests, got %v", result)
	assert.Contains(t, requests, elicit.GateID)
	assert.False(t, called, "the write waits for the retry")
}

// A stateless client can run a read tool.
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
	require.False(t, res.IsError, errorText(res))
}
