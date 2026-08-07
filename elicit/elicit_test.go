package elicit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/acidsailor/mcpkit/elicit"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// gateTool registers the two-pass gate: ask on the first call, act on the retry.
func gateTool(s *mcp.Server) {
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
			resp, ok := elicit.Response(req.Params.InputResponses)
			if !ok {
				res, err := elicit.Ask(
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

func session(
	t *testing.T,
	s *mcp.Server,
	h func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error),
) *mcp.ClientSession {
	t.Helper()
	ct, st := mcp.NewInMemoryTransports()
	ctx := context.Background()
	go func() {
		if _, err := s.Connect(ctx, st, nil); err != nil {
			t.Errorf("server connect: %v", err)
		}
	}()
	var opts *mcp.ClientOptions
	if h != nil {
		opts = &mcp.ClientOptions{ElicitationHandler: h}
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "t", Version: "0"}, opts).
		Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// gateServer wires a fresh server and session answering with action.
func gateServer(t *testing.T, action string) *mcp.ClientSession {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	gateTool(s)
	return session(
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
			res, err := callGate(t, gateServer(t, tt.action))
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

func TestAskNoElicitationCapability(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	gateTool(s)

	cs := session(t, s, nil) // no handler → no elicitation capability

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
	gateTool(s)

	cs := session(
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

func TestResponseAbsent(t *testing.T) {
	_, ok := elicit.Response(nil)
	require.False(t, ok, "a first-pass call carries no confirmation")

	_, ok = elicit.Response(mcp.InputResponseMap{"other": nil})
	require.False(t, ok, "another request's answer is not the gate's")

	resp, ok := elicit.Response(mcp.InputResponseMap{
		elicit.GateID: &mcp.ElicitResult{Action: "accept"},
	})
	require.True(t, ok)
	require.NoError(t, elicit.Decide(resp))
}
