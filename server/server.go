package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const defaultShutdownTimeout = 30 * time.Second

// Server serves an mcp.Server over the configured transport(s).
type Server struct {
	MCP *mcp.Server // escape hatch to the underlying server

	transport       Transport
	shutdownTimeout time.Duration
	httpServer      *http.Server
}

// Option configures a Server.
type Option func(*Server)

// WithTransport sets the transport (default Stdio).
func WithTransport(t Transport) Option {
	return func(s *Server) { s.transport = t }
}

// WithShutdownTimeout sets the HTTP graceful-shutdown timeout (default 30s).
func WithShutdownTimeout(d time.Duration) Option {
	return func(s *Server) { s.shutdownTimeout = d }
}

// WithHTTPServer sets the *http.Server for the HTTP and Both transports,
// served as-is (Handler, Addr, timeouts, TLSConfig, … unchanged). Set its
// Handler yourself — typically Handler(mcpServer), optionally wrapped or
// mounted in a mux. A non-nil TLSConfig serves HTTPS and must carry its own
// certificates. Required for HTTP and Both (else ErrNoHTTPServer); a nil
// Handler is rejected at serve time with ErrNilHandler.
func WithHTTPServer(srv *http.Server) Option {
	return func(s *Server) { s.httpServer = srv }
}

// New builds a Server wrapping mcpServer. Defaults: Stdio transport, 30s
// graceful-shutdown timeout. The HTTP and Both transports require
// WithHTTPServer.
func New(mcpServer *mcp.Server, opts ...Option) *Server {
	s := &Server{
		MCP:             mcpServer,
		transport:       Stdio,
		shutdownTimeout: defaultShutdownTimeout,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

func (s *Server) validate() error {
	if s.MCP == nil {
		return ErrNilServer
	}
	if _, err := ParseTransport(string(s.transport)); err != nil {
		return err
	}
	if s.transport == Stdio {
		return nil
	}
	// HTTP and Both serve a caller-owned server; it must exist and be wired.
	hs := s.httpServer
	if hs == nil {
		return fmt.Errorf(
			"%w: %s transport requires WithHTTPServer",
			ErrNoHTTPServer, s.transport,
		)
	}
	if hs.Handler == nil {
		return fmt.Errorf(
			"%w: WithHTTPServer server has no Handler", ErrNilHandler,
		)
	}
	if _, _, err := net.SplitHostPort(hs.Addr); err != nil {
		return fmt.Errorf("%w: %q: %w", ErrInvalidAddr, hs.Addr, err)
	}
	return nil
}

// ListenAndServe validates config, serves on the configured transport(s),
// blocks until ctx is cancelled, then runs graceful shutdown.
func (s *Server) ListenAndServe(ctx context.Context) error {
	if err := s.validate(); err != nil {
		return err
	}
	switch s.transport {
	case Stdio:
		return s.serveStdio(ctx)
	case HTTP:
		return s.runHTTP(ctx, s.httpServer)
	case Both:
		return s.serveBoth(ctx)
	default:
		return fmt.Errorf("%w: %q", ErrInvalidTransport, s.transport)
	}
}

func (s *Server) serveStdio(ctx context.Context) error {
	slog.InfoContext(ctx, "server running on stdio")
	err := s.MCP.Run(ctx, &mcp.StdioTransport{})
	// context.Canceled is the graceful-shutdown signal (returns nil); a
	// deadline is a real failure and surfaces as ErrServe.
	if err == nil || errors.Is(err, context.Canceled) {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrServe, err)
}

// Handler returns the SDK streamable HTTP handler for m (stateless, JSON
// mode) — set it as the Handler of an *http.Server for WithHTTPServer,
// optionally wrapped with middleware or mounted in a mux.
func Handler(m *mcp.Server) http.Handler {
	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return m },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true},
	)
}

func (s *Server) runHTTP(ctx context.Context, hs *http.Server) error {
	scheme, serve := "http", hs.ListenAndServe
	if hs.TLSConfig != nil {
		scheme = "https"
		serve = func() error { return hs.ListenAndServeTLS("", "") }
	}
	slog.InfoContext(ctx, "server running on "+scheme, "addr", hs.Addr)
	serveErr := make(chan error, 1)
	go func() { serveErr <- serve() }()
	select {
	case err := <-serveErr:
		// serve() returned on its own — a real startup/runtime failure.
		return fmt.Errorf("%w: %w", ErrServe, err)
	case <-ctx.Done():
		// Cancellation is the shutdown signal; serve() then returns
		// http.ErrServerClosed into the buffered channel, unread.
		return s.shutdown(hs)
	}
}

func (s *Server) serveBoth(ctx context.Context) error {
	hs := s.httpServer
	bothCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	stdioErrCh := make(chan error, 1)
	httpErrCh := make(chan error, 1)
	go func() {
		defer cancel()
		runTransport(stdioErrCh, "stdio", func() error {
			return s.serveStdio(bothCtx)
		})
	}()
	go func() {
		defer cancel()
		runTransport(httpErrCh, "http", func() error {
			return s.runHTTP(bothCtx, hs)
		})
	}()
	stdioErr := <-stdioErrCh
	httpErr := <-httpErrCh
	if stdioErr != nil {
		stdioErr = fmt.Errorf("stdio: %w", stdioErr)
	}
	if httpErr != nil {
		httpErr = fmt.Errorf("http: %w", httpErr)
	}
	return errors.Join(stdioErr, httpErr)
}

// runTransport runs fn and sends its result to errCh exactly once. A panic is
// recovered and surfaced as ErrServe rather than crashing the process and
// leaving the partner transport's receiver blocked forever.
func runTransport(errCh chan<- error, name string, fn func() error) {
	defer func() {
		if r := recover(); r != nil {
			errCh <- fmt.Errorf("%w: %s: panic: %v", ErrServe, name, r)
		}
	}()
	errCh <- fn()
}

func (s *Server) shutdown(hs *http.Server) error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		s.shutdownTimeout,
	)
	defer cancel()
	if err := hs.Shutdown(ctx); err != nil {
		return fmt.Errorf("%w: %w", ErrShutdown, err)
	}
	return nil
}
