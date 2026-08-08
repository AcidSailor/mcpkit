package registry_test

import (
	"context"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/acidsailor/mcpkit/elicit"
	"github.com/acidsailor/mcpkit/mcptest"
	"github.com/acidsailor/mcpkit/registry"
	"github.com/acidsailor/mcpkit/toolkit"
)

type echoIn struct {
	Msg string `json:"msg"`
}

type echoOut struct {
	Msg string `json:"msg"`
}

func echo(_ context.Context, in echoIn) (echoOut, error) {
	return echoOut(in), nil
}

func newServer(t *testing.T) *mcp.Server {
	t.Helper()
	return mcp.NewServer(
		&mcp.Implementation{Name: "test", Version: "0"},
		nil,
	)
}

func toolNames(t *testing.T, srv *mcp.Server) []string {
	t.Helper()
	cs := mcptest.NewSession(t, srv)
	res, err := cs.ListTools(
		context.Background(),
		&mcp.ListToolsParams{},
	)
	require.NoError(t, err)
	names := make([]string, 0, len(res.Tools))
	for _, tl := range res.Tools {
		names = append(names, tl.Name)
	}
	return names
}

func TestReadRegistersAndExposesAccess(t *testing.T) {
	r := registry.Read(
		"echo",
		"echoes msg",
		toolkit.InputSchema[echoIn](),
		echo,
	)
	require.Equal(t, "echo", r.Name)
	require.Equal(t, registry.AccessRead, r.Access)

	srv := newServer(t)
	registry.New([]registry.Registration{r}).Bind(srv, registry.Enable{})

	require.Equal(t, []string{"echo"}, toolNames(t, srv))
}

func TestBindSkipsWriteWhenDisabled(t *testing.T) {
	read := registry.Read(
		"r", "", toolkit.InputSchema[echoIn](), echo,
	)
	write := registry.Write(
		"w", "", toolkit.InputSchema[echoIn](), echo,
	)
	require.Equal(t, registry.AccessWrite, write.Access)

	srv := newServer(t)
	registry.New([]registry.Registration{read, write}).
		Bind(srv, registry.Enable{Write: false})

	require.Equal(t, []string{"r"}, toolNames(t, srv))
}

func TestBindIncludesWriteWhenEnabled(t *testing.T) {
	read := registry.Read(
		"r", "", toolkit.InputSchema[echoIn](), echo,
	)
	write := registry.Write(
		"w", "", toolkit.InputSchema[echoIn](), echo,
	)

	srv := newServer(t)
	registry.New([]registry.Registration{read, write}).
		Bind(srv, registry.Enable{Write: true})

	require.ElementsMatch(t, []string{"r", "w"}, toolNames(t, srv))
}

func TestReadCallsHandler(t *testing.T) {
	r := registry.Read("echo", "echoes msg",
		toolkit.InputSchema[echoIn](), echo)
	srv := newServer(t)
	registry.New([]registry.Registration{r}).
		Bind(srv, registry.Enable{})

	cs := mcptest.NewSession(t, srv)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "echo",
		Arguments: map[string]any{"msg": "hi"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
}

// Caller hints reach the wire; the access category still owns ReadOnlyHint.
func TestWithToolAnnotationsReachTheWire(t *testing.T) {
	w := registry.Write(
		"w", "", toolkit.InputSchema[echoIn](), echo,
		registry.WithToolAnnotations[echoIn](mcp.ToolAnnotations{
			Title:           "Create",
			IdempotentHint:  true,
			DestructiveHint: new(false),
		}),
		registry.WithGateID[echoIn]("acme/confirm"),
	)

	srv := newServer(t)
	registry.New([]registry.Registration{w}).
		Bind(srv, registry.Enable{Write: true})

	cs := mcptest.NewSession(t, srv)
	res, err := cs.ListTools(context.Background(), &mcp.ListToolsParams{})
	require.NoError(t, err)
	require.Len(t, res.Tools, 1)

	a := res.Tools[0].Annotations
	require.NotNil(t, a)
	require.False(t, a.ReadOnlyHint, "a write is never read-only")
	require.Equal(t, "Create", a.Title)
	require.True(t, a.IdempotentHint)
	require.False(t, *a.DestructiveHint)
}

func TestReadOnlyHintOnWritePanicsAtBind(t *testing.T) {
	w := registry.Write(
		"w", "", toolkit.InputSchema[echoIn](), echo,
		registry.WithToolAnnotations[echoIn](mcp.ToolAnnotations{
			ReadOnlyHint: true,
		}),
	)

	srv := newServer(t)
	require.PanicsWithError(
		t,
		"w: "+toolkit.ErrReadOnlyMismatch.Error(),
		func() {
			registry.New([]registry.Registration{w}).
				Bind(srv, registry.Enable{Write: true})
		},
	)
}

// WithOutputSchema and WithValidateFunc must survive Bind: the schema reaches
// the wire, and the validator can fail the call.
func TestWithOutputSchemaAndValidateFuncReachTheTool(t *testing.T) {
	r := registry.Read(
		"echo", "", toolkit.InputSchema[echoIn](), echo,
		registry.WithOutputSchema[echoIn](toolkit.InputSchema[echoOut]()),
		registry.WithValidateFunc(
			func(_ context.Context, in echoIn) error {
				if in.Msg == "" {
					return errors.New("msg is required")
				}
				return nil
			},
		),
	)

	srv := newServer(t)
	registry.New([]registry.Registration{r}).Bind(srv, registry.Enable{})
	cs := mcptest.NewSession(t, srv)

	list, err := cs.ListTools(context.Background(), &mcp.ListToolsParams{})
	require.NoError(t, err)
	require.Len(t, list.Tools, 1)
	require.NotNil(t, list.Tools[0].OutputSchema, "output schema must bind")

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "echo",
		Arguments: map[string]any{"msg": ""},
	})
	require.NoError(t, err)
	require.True(t, res.IsError, "the validator must fail the call")
}

// The gate id must survive Bind and key the real confirmation round trip.
func TestWithGateIDDrivesTheConfirmation(t *testing.T) {
	called := false
	w := registry.Write(
		"w", "", toolkit.InputSchema[echoIn](),
		func(_ context.Context, in echoIn) (echoOut, error) {
			called = true
			return echoOut(in), nil
		},
		registry.WithGateID[echoIn]("acme/confirm"),
		registry.WithElicitFunc(
			elicit.SimpleConfirmation[echoIn]("confirm?"),
		),
	)

	srv := newServer(t)
	registry.New([]registry.Registration{w}).
		Bind(srv, registry.Enable{Write: true})

	cs := mcptest.NewSessionWithElicitation(
		t,
		srv,
		func(
			_ context.Context,
			_ *mcp.ElicitRequest,
		) (*mcp.ElicitResult, error) {
			return &mcp.ElicitResult{Action: "accept"}, nil
		},
	)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "w",
		Arguments: map[string]any{"msg": "hi"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.True(t, called, "a custom gate id must complete both passes")
}

func TestWithGateIDOnReadPanicsAtBind(t *testing.T) {
	r := registry.Read(
		"bad", "", toolkit.InputSchema[echoIn](), echo,
		registry.WithGateID[echoIn]("acme/confirm"),
	)

	srv := newServer(t)
	require.PanicsWithError(
		t,
		"bad: "+toolkit.ErrGateIDOnRead.Error(),
		func() {
			registry.New([]registry.Registration{r}).
				Bind(srv, registry.Enable{})
		},
	)
}

func TestWithElicitOnReadPanicsAtBind(t *testing.T) {
	r := registry.Read(
		"bad",
		"",
		toolkit.InputSchema[echoIn](),
		echo,
		registry.WithElicitFunc(
			func(
				_ context.Context,
				_ echoIn,
			) (*mcp.ElicitParams, error) {
				return nil, nil
			},
		),
	)

	srv := newServer(t)
	require.Panics(t, func() {
		registry.New([]registry.Registration{r}).
			Bind(srv, registry.Enable{})
	})
}
