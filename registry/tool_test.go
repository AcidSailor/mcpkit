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
