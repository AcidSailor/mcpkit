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

func gateTool(s *mcp.Server) {
	s.AddTool(
		&mcp.Tool{
			Name:        "gate",
			Description: "test gate",
			InputSchema: &jsonschema.Schema{Type: "object"},
		},
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if err := elicit.Gate(
				ctx,
				req.Session,
				&mcp.ElicitParams{Message: "ok?"},
			); err != nil {
				var r mcp.CallToolResult
				r.SetError(err)
				return &r, nil
			}
			return &mcp.CallToolResult{}, nil
		},
	)
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

func TestGateNoElicitationCapability(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	gateTool(s)

	cs := session(
		t,
		s,
		nil,
	) // no handler → no elicitation capability

	res, err := cs.CallTool(
		context.Background(),
		&mcp.CallToolParams{Name: "gate"},
	)
	require.NoError(t, err)
	require.True(
		t,
		res.IsError,
		"expected tool error when client lacks elicitation capability",
	)
	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	require.Contains(t, tc.Text, elicit.ErrNoElicitation.Error())
}

func TestGateAccept(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	gateTool(s)

	cs := session(
		t,
		s,
		func(_ context.Context, _ *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return &mcp.ElicitResult{Action: "accept"}, nil
		},
	)

	res, err := cs.CallTool(
		context.Background(),
		&mcp.CallToolParams{Name: "gate"},
	)
	require.NoError(t, err)
	require.False(t, res.IsError, "accept should not produce a tool error")
}

func TestGateDecline(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	gateTool(s)

	cs := session(
		t,
		s,
		func(_ context.Context, _ *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return &mcp.ElicitResult{Action: "decline"}, nil
		},
	)

	res, err := cs.CallTool(
		context.Background(),
		&mcp.CallToolParams{Name: "gate"},
	)
	require.NoError(t, err)
	require.True(t, res.IsError, "decline should produce a tool error")
	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	require.Contains(t, tc.Text, elicit.ErrUserDeclined.Error())
}

func TestGateElicitationFailed(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	gateTool(s)

	cs := session(
		t,
		s,
		func(_ context.Context, _ *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return nil, errors.New("transport boom")
		},
	)

	res, err := cs.CallTool(
		context.Background(),
		&mcp.CallToolParams{Name: "gate"},
	)
	require.NoError(t, err)
	require.True(t, res.IsError, "handler error should produce a tool error")
	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	require.Contains(t, tc.Text, elicit.ErrElicitationFailed.Error())
}

func TestGateUnexpectedAction(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	gateTool(s)

	cs := session(
		t,
		s,
		func(_ context.Context, _ *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return &mcp.ElicitResult{Action: "maybe"}, nil
		},
	)

	res, err := cs.CallTool(
		context.Background(),
		&mcp.CallToolParams{Name: "gate"},
	)
	require.NoError(t, err)
	require.True(t, res.IsError, "unknown action should produce a tool error")
	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	require.Contains(t, tc.Text, elicit.ErrUnexpectedElicitAction.Error())
	require.Contains(t, tc.Text, `"maybe"`, "error should echo the action")
}

func TestGateCancel(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	gateTool(s)

	cs := session(
		t,
		s,
		func(_ context.Context, _ *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return &mcp.ElicitResult{Action: "cancel"}, nil
		},
	)

	res, err := cs.CallTool(
		context.Background(),
		&mcp.CallToolParams{Name: "gate"},
	)
	require.NoError(t, err)
	require.True(t, res.IsError, "cancel should produce a tool error")
	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	require.Contains(t, tc.Text, elicit.ErrUserCanceled.Error())
}
