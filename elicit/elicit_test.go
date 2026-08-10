package elicit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/acidsailor/mcpkit/elicit"
	"github.com/acidsailor/mcpkit/mcptest"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// gateTool registers a two-pass gated tool.
func gateTool(s *mcp.Server, gateID string) {
	s.AddTool(
		&mcp.Tool{
			Name:        "gate",
			Description: "test gate",
			InputSchema: &jsonschema.Schema{Type: "object"},
		},
		func(
			_ context.Context,
			req *mcp.CallToolRequest,
		) (*mcp.CallToolResult, error) {
			resp, ok := req.Params.InputResponses[gateID]
			if !ok {
				return elicit.Ask(
					gateID,
					&mcp.ElicitParams{Message: "ok?"},
				), nil
			}
			if err := elicit.Decide(resp); err != nil {
				return toolError(err), nil
			}
			return &mcp.CallToolResult{}, nil
		},
	)
}

func toolError(err error) *mcp.CallToolResult {
	var r mcp.CallToolResult
	r.SetError(err)
	return &r
}

// gateServer creates a session that returns action.
func gateServer(t *testing.T, action, gateID string) *mcp.ClientSession {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	gateTool(s, gateID)
	return mcptest.NewSessionWithElicitation(
		t,
		s,
		func(
			_ context.Context,
			_ *mcp.ElicitRequest,
		) (*mcp.ElicitResult, error) {
			return &mcp.ElicitResult{Action: action}, nil
		},
	)
}

func callGate(
	t *testing.T,
	cs *mcp.ClientSession,
) (*mcp.CallToolResult, error) {
	t.Helper()
	return cs.CallTool(
		context.Background(),
		&mcp.CallToolParams{Name: "gate"},
	)
}

// The client action determines the gated call result.
func TestGateActions(t *testing.T) {
	tests := []struct {
		name    string
		action  string
		wantErr error
		echo    string
	}{
		{"accept", "accept", nil, ""},
		{"decline", "decline", elicit.ErrUserDeclined, ""},
		{"cancel", "cancel", elicit.ErrUserCanceled, ""},
		{
			"unexpected",
			"maybe",
			elicit.ErrUnexpectedElicitAction,
			`"maybe"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := callGate(t, gateServer(t, tt.action, elicit.GateID))
			require.NoError(t, err)

			if tt.wantErr == nil {
				require.False(t, res.IsError, "accept must not error")
				return
			}
			require.True(t, res.IsError, "%s must error", tt.action)

			tc, ok := res.Content[0].(*mcp.TextContent)
			require.True(t, ok)
			require.Contains(t, tc.Text, tt.wantErr.Error())
			if tt.echo != "" {
				require.Contains(t, tc.Text, tt.echo, "must echo the action")
			}
		})
	}
}

// A custom gate ID completes both passes.
func TestGateCustomID(t *testing.T) {
	cs := gateServer(t, "accept", "acme/confirm")
	res, err := callGate(t, cs)
	require.NoError(t, err)
	require.False(t, res.IsError, "a custom gate id must round-trip")
}

// The SDK, not Ask, refuses a client that cannot answer the request.
func TestAskNoElicitationCapability(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	gateTool(s, elicit.GateID)

	// NewSession does not advertise elicitation.
	cs := mcptest.NewSession(t, s)

	_, err := callGate(t, cs)
	require.Error(t, err, "an unanswerable gate must fail the call")
	require.Contains(t, err.Error(), "client does not support elicitation")
}

// A client handler error stops the retry.
func TestGateClientHandlerFails(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	gateTool(s, elicit.GateID)

	cs := mcptest.NewSessionWithElicitation(
		t,
		s,
		func(
			_ context.Context,
			_ *mcp.ElicitRequest,
		) (*mcp.ElicitResult, error) {
			return nil, errors.New("transport boom")
		},
	)

	_, err := callGate(t, cs)
	require.Error(t, err, "fulfilment failure must fail the call")
	require.Contains(t, err.Error(), "transport boom")
}

func TestDecideRejectsNonElicitResponse(t *testing.T) {
	err := elicit.Decide(nil)
	require.ErrorIs(t, err, elicit.ErrElicitationFailed)
}

// Decide rejects a typed nil result without panicking.
func TestDecideRejectsTypedNilResult(t *testing.T) {
	require.NotPanics(t, func() {
		err := elicit.Decide((*mcp.ElicitResult)(nil))
		require.ErrorIs(t, err, elicit.ErrElicitationFailed)
	})
}

// Nil params still produce a request carrying an object schema.
func TestAskNilParams(t *testing.T) {
	res := elicit.Ask(elicit.GateID, nil)
	params, ok := res.InputRequests[elicit.GateID].(*mcp.ElicitParams)
	require.True(t, ok)
	require.NotNil(t, params.RequestedSchema)
}
