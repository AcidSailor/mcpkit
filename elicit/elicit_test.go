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

// gateTool registers the two-pass gate: ask on the first call, act on retry.
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
				res, err := elicit.Ask(
					gateID,
					req.Session,
					&mcp.ElicitParams{Message: "ok?"},
				)
				if err != nil {
					return toolError(err), nil
				}
				return res, nil
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

// gateServer wires a fresh server and session answering with action.
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

// The client's answer drives the outcome across a full multi-round-trip call.
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

// A gate keyed on a custom id completes the same two passes.
func TestGateCustomID(t *testing.T) {
	cs := gateServer(t, "accept", "acme/confirm")
	res, err := callGate(t, cs)
	require.NoError(t, err)
	require.False(t, res.IsError, "a custom gate id must round-trip")
}

func TestAskNoElicitationCapability(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	gateTool(s, elicit.GateID)

	cs := mcptest.NewSession(t, s) // no handler → no elicitation capability

	res, err := callGate(t, cs)
	require.NoError(t, err)
	require.True(t, res.IsError, "must error without the capability")

	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	require.Contains(t, tc.Text, elicit.ErrNoElicitation.Error())
}

// A failing client handler aborts the call before the server is asked again.
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

// A typed-nil result must not dereference: Decide is exported, and a panic in
// a tool handler is a process kill (the SDK does not recover).
func TestDecideRejectsTypedNilResult(t *testing.T) {
	require.NotPanics(t, func() {
		err := elicit.Decide((*mcp.ElicitResult)(nil))
		require.ErrorIs(t, err, elicit.ErrElicitationFailed)
	})
}

// Ask is exported, so a nil session is its own error, not an SDK panic.
func TestAskNilSession(t *testing.T) {
	require.NotPanics(t, func() {
		_, err := elicit.Ask(elicit.GateID, nil, nil)
		require.ErrorIs(t, err, elicit.ErrNoElicitation)
	})
}
