package toolkit

import (
	"context"
	"errors"
	"testing"

	"github.com/acidsailor/mcpkit/elicit"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddRead_Success(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	AddRead(New(s, "echo", "echoes", objectSchema(),
		func(_ context.Context, in echoIn) (echoOut, error) {
			return echoOut{Echo: in.Msg}, nil
		}))

	cs := newTestMCPSession(t, s)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "echo",
		Arguments: map[string]any{"msg": "hi"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Len(t, res.Content, 1)
	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.JSONEq(t, `{"echo":"hi"}`, tc.Text)
}

func TestAddRead_DecodeError(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	AddRead(New(s, "echo", "echoes", objectSchema(),
		func(_ context.Context, in echoIn) (echoOut, error) {
			return echoOut{Echo: in.Msg}, nil
		}))

	cs := newTestMCPSession(t, s)
	// msg is an int but echoIn.Msg is a string: the decode failure must surface
	// as a tool error (IsError), not a protocol-level Go error.
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "echo",
		Arguments: map[string]any{"msg": 42},
	})
	require.NoError(t, err, "decode error is a tool-level error")
	assert.True(t, res.IsError)
}

func TestAddRead_ValidateFail(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	AddRead(New(s, "echo", "echoes", objectSchema(),
		func(_ context.Context, in echoIn) (echoOut, error) {
			return echoOut{Echo: in.Msg}, nil
		}).
		WithValidateFunc(func(_ context.Context, _ echoIn) error {
			return errors.New("bad input")
		}))

	cs := newTestMCPSession(t, s)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "echo",
		Arguments: map[string]any{"msg": "hi"},
	})
	require.NoError(t, err, "validate error is a tool-level error")
	assert.True(t, res.IsError)
}

func TestAddRead_PanicsWhenElicitSet(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	assert.Panics(t, func() {
		AddRead(New(s, "echo", "echoes", objectSchema(),
			func(_ context.Context, in echoIn) (echoOut, error) {
				return echoOut{Echo: in.Msg}, nil
			}).
			WithElicitParamsFunc(elicit.SimpleConfirmation[echoIn]("x")))
	})
}

func TestAddReadFunc_CustomHandler(t *testing.T) {
	called := false
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	// The supplied handler runs as-is; it returns its own structured result
	// rather than echoing the input, proving callFunc is what executes.
	AddReadFunc(
		New(s, "echo", "echoes", objectSchema(),
			func(_ context.Context, in echoIn) (echoOut, error) {
				return echoOut{Echo: in.Msg}, nil
			}),
		func(
			_ context.Context,
			_ *mcp.CallToolRequest,
			_ echoIn,
		) (*mcp.CallToolResult, echoOut, error) {
			called = true
			return nil, echoOut{Echo: "custom"}, nil
		},
	)

	cs := newTestMCPSession(t, s)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "echo",
		Arguments: map[string]any{"msg": "hi"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	assert.True(t, called, "custom handler should run")
	require.Len(t, res.Content, 1)
	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.JSONEq(t, `{"echo":"custom"}`, tc.Text)
}

func TestAddReadFunc_PanicsWhenElicitSet(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	assert.Panics(t, func() {
		AddReadFunc(
			New(s, "echo", "echoes", objectSchema(),
				func(_ context.Context, in echoIn) (echoOut, error) {
					return echoOut{Echo: in.Msg}, nil
				}).
				WithElicitParamsFunc(elicit.SimpleConfirmation[echoIn]("x")),
			func(
				_ context.Context,
				_ *mcp.CallToolRequest,
				in echoIn,
			) (*mcp.CallToolResult, echoOut, error) {
				return nil, echoOut{Echo: in.Msg}, nil
			},
		)
	})
}
