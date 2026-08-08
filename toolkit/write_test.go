package toolkit

import (
	"context"
	"encoding/json"
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

// A gated call runs the handler twice: ask, then act. The write happens once.
func TestAddWrite_TwoPasses(t *testing.T) {
	var validated, called int
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	AddWrite(New(s, "do", "does", objectSchema(),
		func(_ context.Context, in echoIn) (echoOut, error) {
			called++
			return echoOut{Echo: in.Msg}, nil
		}).
		WithValidateFunc(func(_ context.Context, _ echoIn) error {
			validated++
			return nil
		}).
		WithElicitParamsFunc(elicit.SimpleConfirmation[echoIn]("confirm?")))

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
	assert.Equal(t, 2, validated, "validator runs on both passes")
	assert.Equal(t, 1, called, "the write runs once")
}

// acceptor answers every confirmation with accept.
func acceptor(
	_ context.Context,
	_ *mcp.ElicitRequest,
) (*mcp.ElicitResult, error) {
	return &mcp.ElicitResult{Action: "accept"}, nil
}

// callDo drives the "do" tool with a single string argument.
func callDo(
	t *testing.T,
	cs *mcp.ClientSession,
) (*mcp.CallToolResult, error) {
	t.Helper()
	return cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "do",
		Arguments: map[string]any{"msg": "hi"},
	})
}

// A prompt builder that fails aborts the call before it asks or writes.
func TestAddWrite_ElicitParamsError(t *testing.T) {
	called, elicited := false, false
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	AddWrite(New(s, "do", "does", objectSchema(),
		func(_ context.Context, in echoIn) (echoOut, error) {
			called = true
			return echoOut{Echo: in.Msg}, nil
		}).
		WithElicitParamsFunc(elicit.DynamicConfirmation(
			func(_ context.Context, _ echoIn) (string, error) {
				return "", errors.New("cannot describe")
			},
		)))

	cs := newTestMCPSessionWithElicitation(
		t,
		s,
		func(
			ctx context.Context,
			req *mcp.ElicitRequest,
		) (*mcp.ElicitResult, error) {
			elicited = true
			return acceptor(ctx, req)
		},
	)
	res, err := callDo(t, cs)
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, errorText(t, res), "do: cannot describe")
	assert.False(t, elicited, "a failed prompt must not reach the client")
	assert.False(t, called, "the write must not run")
}

// The validator guards the act pass too: state may change after the prompt.
func TestAddWrite_ValidateFailsOnSecondPass(t *testing.T) {
	var passes int
	called := false
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	AddWrite(New(s, "do", "does", objectSchema(),
		func(_ context.Context, in echoIn) (echoOut, error) {
			called = true
			return echoOut{Echo: in.Msg}, nil
		}).
		WithValidateFunc(func(_ context.Context, _ echoIn) error {
			passes++
			if passes > 1 {
				return errors.New("state changed since the prompt")
			}
			return nil
		}).
		WithElicitParamsFunc(elicit.SimpleConfirmation[echoIn]("confirm?")))

	cs := newTestMCPSessionWithElicitation(t, s, acceptor)
	res, err := callDo(t, cs)
	require.NoError(t, err)
	assert.True(t, res.IsError, "a stale confirmation must not write")
	assert.Contains(t, errorText(t, res), "state changed since the prompt")
	assert.False(t, called, "the write must not run")
}

// WithGateID keys the confirmation, and the retry round-trips under that key.
func TestAddWrite_CustomGateID(t *testing.T) {
	called := false
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	AddWrite(New(s, "do", "does", objectSchema(),
		func(_ context.Context, in echoIn) (echoOut, error) {
			called = true
			return echoOut{Echo: in.Msg}, nil
		}).
		WithGateID("acme/confirm").
		WithElicitParamsFunc(elicit.SimpleConfirmation[echoIn]("confirm?")))

	cs := newTestMCPSessionWithElicitation(t, s, acceptor)
	res, err := callDo(t, cs)
	require.NoError(t, err)
	require.False(t, res.IsError, errorTextOrEmpty(res))
	assert.True(t, called, "a custom gate id must complete both passes")
}

// A write with no prompt builder still asks, with a default message and the
// non-nil empty properties clients require.
func TestAddWrite_DefaultPrompt(t *testing.T) {
	var got *mcp.ElicitParams
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	AddWrite(New(s, "do", "does", objectSchema(),
		func(_ context.Context, in echoIn) (echoOut, error) {
			return echoOut{Echo: in.Msg}, nil
		}))

	cs := newTestMCPSessionWithElicitation(
		t,
		s,
		func(
			_ context.Context,
			req *mcp.ElicitRequest,
		) (*mcp.ElicitResult, error) {
			got = req.Params
			return &mcp.ElicitResult{Action: "accept"}, nil
		},
	)
	res, err := callDo(t, cs)
	require.NoError(t, err)
	require.False(t, res.IsError, errorTextOrEmpty(res))

	require.NotNil(t, got, "an ungated-looking write must still prompt")
	assert.Equal(t, "Run do?", got.Message)
	require.NotNil(t, got.RequestedSchema)

	schema, err := json.Marshal(got.RequestedSchema)
	require.NoError(t, err)
	assert.Contains(
		t,
		string(schema),
		`"properties"`,
		"clients reject an omitted requestedSchema.properties",
	)
}

// The gate proves an answer was reported, not that one was ever asked for: a
// call already carrying an accept skips the ask. Deliberate — see elicit's doc.
func TestAddWrite_SuppliedAnswerSkipsAsk(t *testing.T) {
	called := false
	s := writeServer(t, &called)

	cs := newTestMCPSession(t, s) // no elicitation capability at all
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "do",
		Arguments: map[string]any{"msg": "hi"},
		InputResponses: mcp.InputResponseMap{
			elicit.GateID: &mcp.ElicitResult{Action: "accept"},
		},
	})
	require.NoError(t, err)
	require.False(t, res.IsError, errorTextOrEmpty(res))
	assert.True(
		t,
		called,
		"a supplied answer is trusted verbatim; the gate is not authorization",
	)
}

// errorTextOrEmpty renders a result's first text block, for failure messages.
func errorTextOrEmpty(res *mcp.CallToolResult) string {
	if len(res.Content) == 0 {
		return ""
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		return ""
	}
	return tc.Text
}
