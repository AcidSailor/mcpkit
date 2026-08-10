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

// answering replies to each confirmation with action.
func answering(action string) func(
	context.Context, *mcp.ElicitRequest,
) (*mcp.ElicitResult, error) {
	return func(
		_ context.Context,
		_ *mcp.ElicitRequest,
	) (*mcp.ElicitResult, error) {
		return &mcp.ElicitResult{Action: action}, nil
	}
}

// callDo calls the do tool with one string argument.
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

// errorText returns the first text block or an empty string.
func errorText(res *mcp.CallToolResult) string {
	if len(res.Content) == 0 {
		return ""
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		return ""
	}
	return tc.Text
}

// wantNoElicitation is the SDK's refusal text; it exports no sentinel.
const wantNoElicitation = "client does not support elicitation"

func TestAddWrite_NoElicitationHandler(t *testing.T) {
	called := false
	s := writeServer(t, &called)
	cs := newTestMCPSession(t, s) // The client has no elicitation handler.
	_, err := callDo(t, cs)
	require.Error(t, err, "an unanswerable gate must fail the call")
	assert.Contains(t, err.Error(), wantNoElicitation)
	assert.False(t, called, "the write must not run")
}

func TestAddWrite_Accept(t *testing.T) {
	called := false
	s := writeServer(t, &called)
	cs := newTestMCPSessionWithElicitation(t, s, answering("accept"))
	res, err := callDo(t, cs)
	require.NoError(t, err)
	require.False(t, res.IsError)
	assert.True(t, called, "handler should run on accept")
}

func TestAddWrite_Decline(t *testing.T) {
	called := false
	s := writeServer(t, &called)
	cs := newTestMCPSessionWithElicitation(t, s, answering("decline"))
	res, err := callDo(t, cs)
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.False(t, called, "handler must not run on decline")
	assert.Contains(t, errorText(res), ErrUserDeclined.Error())
}

func TestAddWrite_Cancel(t *testing.T) {
	called := false
	s := writeServer(t, &called)
	cs := newTestMCPSessionWithElicitation(t, s, answering("cancel"))
	res, err := callDo(t, cs)
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.False(t, called, "handler must not run on cancel")
	assert.Contains(t, errorText(res), ErrUserCanceled.Error())
}

func TestAddWriteFunc_RunsWithoutElicitation(t *testing.T) {
	called := false
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	// AddWriteFunc runs without elicit.Gate.
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

	cs := newTestMCPSession(t, s) // The client has no elicitation handler.
	res, err := callDo(t, cs)
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
		func(
			ctx context.Context,
			req *mcp.ElicitRequest,
		) (*mcp.ElicitResult, error) {
			elicited = true
			return answering("accept")(ctx, req)
		},
	)
	res, err := callDo(t, cs)
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.False(t, elicited, "validation must run before elicitation")
}

// A gated call asks once and writes once.
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

	cs := newTestMCPSessionWithElicitation(t, s, answering("accept"))
	res, err := callDo(t, cs)
	require.NoError(t, err)
	require.False(t, res.IsError)
	assert.Equal(t, 2, validated, "validator runs on both passes")
	assert.Equal(t, 1, called, "the write runs once")
}

// A prompt error prevents elicitation and mutation.
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
			return answering("accept")(ctx, req)
		},
	)
	res, err := callDo(t, cs)
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, errorText(res), "do: cannot describe")
	assert.False(t, elicited, "a failed prompt must not reach the client")
	assert.False(t, called, "the write must not run")
}

// Validation also runs on the action pass.
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

	cs := newTestMCPSessionWithElicitation(t, s, answering("accept"))
	res, err := callDo(t, cs)
	require.NoError(t, err)
	assert.True(t, res.IsError, "a stale confirmation must not write")
	assert.Contains(t, errorText(res), "state changed since the prompt")
	assert.False(t, called, "the write must not run")
}

// A custom gate ID survives the confirmation round trip.
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

	cs := newTestMCPSessionWithElicitation(t, s, answering("accept"))
	res, err := callDo(t, cs)
	require.NoError(t, err)
	require.False(t, res.IsError, errorText(res))
	assert.True(t, called, "a custom gate id must complete both passes")
}

// A write without a prompt builder uses the default confirmation.
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
			ctx context.Context,
			req *mcp.ElicitRequest,
		) (*mcp.ElicitResult, error) {
			got = req.Params
			return answering("accept")(ctx, req)
		},
	)
	res, err := callDo(t, cs)
	require.NoError(t, err)
	require.False(t, res.IsError, errorText(res))

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

// A supplied acceptance bypasses the confirmation request.
func TestAddWrite_SuppliedAnswerSkipsAsk(t *testing.T) {
	called := false
	s := writeServer(t, &called)

	cs := newTestMCPSession(t, s) // The client has no elicitation handler.
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "do",
		Arguments: map[string]any{"msg": "hi"},
		InputResponses: mcp.InputResponseMap{
			elicit.GateID: &mcp.ElicitResult{Action: "accept"},
		},
	})
	require.NoError(t, err)
	require.False(t, res.IsError, errorText(res))
	assert.True(
		t,
		called,
		"a supplied answer is trusted verbatim; the gate is not authorization",
	)
}
