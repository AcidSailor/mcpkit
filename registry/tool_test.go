package registry_test

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

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
