package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func newMCP() *mcp.Server {
	return mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
}

// httpHandler builds an SDK streamable handler for a non-nil Handler in tests.
func httpHandler(m *mcp.Server) http.Handler {
	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return m },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true},
	)
}

func TestNew_Defaults(t *testing.T) {
	s := New(newMCP())
	require.Equal(t, Stdio, s.transport)
	require.Equal(t, 30*time.Second, s.shutdownTimeout)
	require.Nil(t, s.httpServer)
}

func TestNew_Options(t *testing.T) {
	hs := &http.Server{Addr: "127.0.0.1:9000"}
	s := New(newMCP(),
		WithTransport(HTTP),
		WithShutdownTimeout(2*time.Second),
		WithHTTPServer(hs),
	)
	require.Equal(t, HTTP, s.transport)
	require.Equal(t, 2*time.Second, s.shutdownTimeout)
	require.Same(t, hs, s.httpServer)
}

func TestListenAndServe_InvalidTransport(t *testing.T) {
	s := New(newMCP(), WithTransport(Transport("ftp")))
	err := s.ListenAndServe(context.Background())
	require.ErrorIs(t, err, ErrInvalidTransport)
}

func TestListenAndServe_HTTPBadAddr(t *testing.T) {
	m := newMCP()
	s := New(
		m,
		WithTransport(HTTP),
		WithHTTPServer(
			&http.Server{Addr: "not-an-addr", Handler: httpHandler(m)},
		),
	)
	err := s.ListenAndServe(context.Background())
	require.ErrorIs(t, err, ErrInvalidAddr)
}

func TestListenAndServe_NoHTTPServer(t *testing.T) {
	s := New(newMCP(), WithTransport(HTTP))
	err := s.ListenAndServe(context.Background())
	require.ErrorIs(t, err, ErrNoHTTPServer)
}

func TestListenAndServe_BothNoHTTPServer(t *testing.T) {
	// Both shares the HTTP validation contract: it needs WithHTTPServer too.
	s := New(newMCP(), WithTransport(Both))
	err := s.ListenAndServe(context.Background())
	require.ErrorIs(t, err, ErrNoHTTPServer)
}

func TestListenAndServe_NilServer(t *testing.T) {
	s := New(nil)
	err := s.ListenAndServe(context.Background())
	require.ErrorIs(t, err, ErrNilServer)
}

func TestRunWithRecover_ForwardsResult(t *testing.T) {
	ch := make(chan error, 1)
	runWithRecover(ch, "stdio", func() error { return nil })
	require.NoError(t, <-ch)

	sentinel := errors.New("boom")
	runWithRecover(ch, "stdio", func() error { return sentinel })
	require.ErrorIs(t, <-ch, sentinel)
}

func TestRunWithRecover_RecoversPanic(t *testing.T) {
	ch := make(chan error, 1)
	runWithRecover(ch, "http", func() error { panic("kaboom") })
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

	m := newMCP()
	base := &http.Server{Addr: addr, Handler: httpHandler(m)}
	s := New(m, WithTransport(HTTP), WithHTTPServer(base))
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

func TestWithHTTPServer_ServesAsIs(t *testing.T) {
	m := newMCP()
	base := &http.Server{Addr: "127.0.0.1:9999", Handler: httpHandler(m)}
	s := New(m, WithTransport(HTTP), WithHTTPServer(base))
	require.NoError(t, s.validate())
	require.Same(t, base, s.httpServer) // served unchanged
}

func TestWithHTTPServer_NilHandler(t *testing.T) {
	base := &http.Server{Addr: "127.0.0.1:9999"} // no Handler
	s := New(newMCP(), WithTransport(HTTP), WithHTTPServer(base))
	err := s.ListenAndServe(context.Background())
	require.ErrorIs(t, err, ErrNilHandler)
}

func selfSignedTLSConfig(t *testing.T) *tls.Config {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(
		rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	return &tls.Config{Certificates: []tls.Certificate{cert}}
}

func TestListenAndServe_HTTPSGracefulShutdown(t *testing.T) {
	// Grab a free port, then release it for the server.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())

	m := newMCP()
	base := &http.Server{
		Addr:      addr,
		Handler:   httpHandler(m),
		TLSConfig: selfSignedTLSConfig(t),
	}
	s := New(m, WithTransport(HTTP), WithHTTPServer(base))
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() { errCh <- s.ListenAndServe(ctx) }()

	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
	require.Eventually(t, func() bool {
		resp, derr := client.Get("https://" + addr + "/")
		if derr != nil {
			return false
		}
		_ = resp.Body.Close()
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
