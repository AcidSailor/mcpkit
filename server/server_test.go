package server

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func newMCP() *mcp.Server {
	return mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
}

func TestNew_Defaults(t *testing.T) {
	s := New(newMCP())
	require.Equal(t, ":8080", s.addr)
	require.Equal(t, Stdio, s.transport)
	require.Equal(t, 30*time.Second, s.readHeaderTimeout)
	require.Equal(t, 30*time.Second, s.shutdownTimeout)
}

func TestNew_Options(t *testing.T) {
	s := New(newMCP(),
		WithAddr("127.0.0.1:9000"),
		WithTransport(HTTP),
		WithReadHeaderTimeout(5*time.Second),
		WithShutdownTimeout(2*time.Second),
	)
	require.Equal(t, "127.0.0.1:9000", s.addr)
	require.Equal(t, HTTP, s.transport)
	require.Equal(t, 5*time.Second, s.readHeaderTimeout)
	require.Equal(t, 2*time.Second, s.shutdownTimeout)
}

func TestListenAndServe_InvalidTransport(t *testing.T) {
	s := New(newMCP(), WithTransport(Transport("ftp")))
	err := s.ListenAndServe(context.Background())
	require.ErrorIs(t, err, ErrInvalidTransport)
}

func TestListenAndServe_HTTPBadAddr(t *testing.T) {
	s := New(newMCP(), WithTransport(HTTP), WithAddr("not-an-addr"))
	err := s.ListenAndServe(context.Background())
	require.ErrorIs(t, err, ErrInvalidAddr)
}

func TestListenAndServe_NilServer(t *testing.T) {
	s := New(nil)
	err := s.ListenAndServe(context.Background())
	require.ErrorIs(t, err, ErrNilServer)
}

func TestRunTransport_ForwardsResult(t *testing.T) {
	ch := make(chan error, 1)
	runTransport(ch, "stdio", func() error { return nil })
	require.NoError(t, <-ch)

	sentinel := errors.New("boom")
	runTransport(ch, "stdio", func() error { return sentinel })
	require.ErrorIs(t, <-ch, sentinel)
}

func TestRunTransport_RecoversPanic(t *testing.T) {
	ch := make(chan error, 1)
	runTransport(ch, "http", func() error { panic("kaboom") })
	err := <-ch
	require.ErrorIs(t, err, ErrServe)
	require.Contains(t, err.Error(), "http")
	require.Contains(t, err.Error(), "kaboom")
}

func TestListenAndServe_HTTPGracefulShutdown(t *testing.T) {
	// Grab a free port, then release it for the server.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())

	s := New(newMCP(), WithTransport(HTTP), WithAddr(addr))
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() { errCh <- s.ListenAndServe(ctx) }()

	// Wait until the server accepts connections.
	require.Eventually(t, func() bool {
		c, derr := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if derr != nil {
			return false
		}
		_ = c.Close()
		return true
	}, 3*time.Second, 50*time.Millisecond)

	cancel()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("ListenAndServe did not return after ctx cancel")
	}
}
