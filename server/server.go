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

// WithHTTPServer sets the caller-owned *http.Server for HTTP and Both.
func WithHTTPServer(srv *http.Server) Option {
	return func(s *Server) { s.httpServer = srv }
}

// New builds a Server wrapping mcpServer (default: Stdio, 30s shutdown).
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
	// HTTP and Both need the caller-owned server to exist and be wired.
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

// ListenAndServe validates, serves on the transport(s), and shuts down on ctx.
func (s *Server) ListenAndServe(ctx context.Context) error {
	if err := s.validate(); err != nil {
		return err
	}
	switch s.transport {
	case Stdio:
		return s.serveStdio(ctx)
	case HTTP:
		return s.serveHTTP(ctx, s.httpServer)
	case Both:
		return s.serveBoth(ctx)
	default:
		return fmt.Errorf("%w: %q", ErrInvalidTransport, s.transport)
	}
}

func (s *Server) serveStdio(ctx context.Context) error {
	slog.InfoContext(ctx, "server running on stdio")
	err := s.MCP.Run(ctx, &mcp.StdioTransport{})
	// context.Canceled is the shutdown signal (nil); anything else is ErrServe.
	if err == nil || errors.Is(err, context.Canceled) {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrServe, err)
}

func (s *Server) serveHTTP(ctx context.Context, hs *http.Server) error {
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
		// serve() returned on its own — a startup/runtime failure.
		return fmt.Errorf("%w: %w", ErrServe, err)
	case <-ctx.Done():
		// Shutdown signal; serve()'s http.ErrServerClosed lands unread.
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
		runWithRecover(stdioErrCh, "stdio", func() error {
			return s.serveStdio(bothCtx)
		})
	}()
	go func() {
		defer cancel()
		runWithRecover(httpErrCh, "http", func() error {
			return s.serveHTTP(bothCtx, hs)
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

// runWithRecover sends fn's result to errCh once; a panic becomes ErrServe.
func runWithRecover(errCh chan<- error, name string, fn func() error) {
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
