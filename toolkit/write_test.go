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

func writeServer(t *testing.T, called *bool) *mcp.Server {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	AddWrite(New(s, "do", "does", objectSchema(),
		func(_ context.Context, in echoIn) (echoOut, error) {
			if called != nil {
				*called = true
			}
			return echoOut{Echo: in.Msg}, nil
		}).
		WithElicitParamsFunc(elicit.SimpleConfirmation[echoIn]("confirm?")))
	return s
}

func TestAddWrite_NoElicitationCapability(t *testing.T) {
	s := writeServer(t, nil)
	cs := newTestMCPSession(t, s) // client has no ElicitationHandler
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "do",
		Arguments: map[string]any{"msg": "hi"},
	})
	require.NoError(t, err)
	assert.True(
		t,
		res.IsError,
		"missing elicitation capability is a tool error",
	)
	assert.Contains(t, errorText(t, res), ErrNoElicitation.Error())
}

func TestAddWrite_Accept(t *testing.T) {
	called := false
	s := writeServer(t, &called)
	cs := newTestMCPSessionWithElicitation(
		t,
		s,
		func(_ context.Context, _ *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return &mcp.ElicitResult{Action: "accept"}, nil
		},
	)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "do",
		Arguments: map[string]any{"msg": "hi"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	assert.True(t, called, "handler should run on accept")
}

func TestAddWrite_Decline(t *testing.T) {
	called := false
	s := writeServer(t, &called)
	cs := newTestMCPSessionWithElicitation(
		t,
		s,
		func(_ context.Context, _ *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return &mcp.ElicitResult{Action: "decline"}, nil
		},
	)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "do",
		Arguments: map[string]any{"msg": "hi"},
	})
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.False(t, called, "handler must not run on decline")
	assert.Contains(t, errorText(t, res), ErrUserDeclined.Error())
}

func TestAddWrite_Cancel(t *testing.T) {
	called := false
	s := writeServer(t, &called)
	cs := newTestMCPSessionWithElicitation(
		t,
		s,
		func(_ context.Context, _ *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return &mcp.ElicitResult{Action: "cancel"}, nil
		},
	)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "do",
		Arguments: map[string]any{"msg": "hi"},
	})
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.False(t, called, "handler must not run on cancel")
	assert.Contains(t, errorText(t, res), ErrUserCanceled.Error())
}

// errorText returns the text of a tool result's first content block, for
// asserting which sentinel surfaced.
func errorText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	require.Len(t, res.Content, 1)
	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	return tc.Text
}

func TestAddWriteFunc_RunsWithoutElicitation(t *testing.T) {
	called := false
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	// AddWriteFunc skips elicit.Gate, so the write runs without elicitation.
	AddWriteFunc(
		New(s, "do", "does", objectSchema(),
			func(_ context.Context, in echoIn) (echoOut, error) {
				return echoOut{Echo: in.Msg}, nil
			}).
			WithElicitParamsFunc(elicit.SimpleConfirmation[echoIn]("confirm?")),
		func(
			_ context.Context,
			_ *mcp.CallToolRequest,
			in echoIn,
		) (*mcp.CallToolResult, echoOut, error) {
			called = true
			return nil, echoOut{Echo: in.Msg}, nil
		},
	)

	cs := newTestMCPSession(t, s) // client has no ElicitationHandler
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "do",
		Arguments: map[string]any{"msg": "hi"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError, "handler must run without elicitation")
	assert.True(t, called, "custom handler should run")
}

func TestAddWrite_ValidateBeforeElicit(t *testing.T) {
	elicited := false
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	AddWrite(New(s, "do", "does", objectSchema(),
		func(_ context.Context, in echoIn) (echoOut, error) {
			return echoOut{Echo: in.Msg}, nil
		}).
		WithValidateFunc(func(_ context.Context, _ echoIn) error {
			return errors.New("bad input")
		}).
		WithElicitParamsFunc(elicit.SimpleConfirmation[echoIn]("x")))

	cs := newTestMCPSessionWithElicitation(
		t,
		s,
		func(_ context.Context, _ *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			elicited = true
			return &mcp.ElicitResult{Action: "accept"}, nil
		},
	)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "do",
		Arguments: map[string]any{"msg": "hi"},
	})
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.False(t, elicited, "validation must run before elicitation")
}
