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

const (
	defaultAddr              = ":8080"
	defaultReadHeaderTimeout = 30 * time.Second
	defaultShutdownTimeout   = 30 * time.Second
)

// Server serves an mcp.Server over the configured transport(s).
type Server struct {
	MCP *mcp.Server // escape hatch to the underlying server

	addr              string
	transport         Transport
	readHeaderTimeout time.Duration
	shutdownTimeout   time.Duration
}

// Option configures a Server.
type Option func(*Server)

// WithAddr sets the HTTP listen address (default ":8080"). Ignored for the
// stdio transport.
func WithAddr(addr string) Option { return func(s *Server) { s.addr = addr } }

// WithTransport sets the transport (default Stdio).
func WithTransport(t Transport) Option {
	return func(s *Server) { s.transport = t }
}

// WithReadHeaderTimeout sets the HTTP server's read-header timeout
// (default 30s).
func WithReadHeaderTimeout(d time.Duration) Option {
	return func(s *Server) { s.readHeaderTimeout = d }
}

// WithShutdownTimeout sets the HTTP graceful-shutdown timeout (default 30s).
func WithShutdownTimeout(d time.Duration) Option {
	return func(s *Server) { s.shutdownTimeout = d }
}

// New builds a Server wrapping mcpServer. Defaults: addr ":8080", Stdio
// transport, both timeouts 30s.
func New(mcpServer *mcp.Server, opts ...Option) *Server {
	s := &Server{
		MCP:               mcpServer,
		addr:              defaultAddr,
		transport:         Stdio,
		readHeaderTimeout: defaultReadHeaderTimeout,
		shutdownTimeout:   defaultShutdownTimeout,
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
	if s.transport != Stdio {
		if _, _, err := net.SplitHostPort(s.addr); err != nil {
			return fmt.Errorf("%w: %q: %w", ErrInvalidAddr, s.addr, err)
		}
	}
	return nil
}

// ListenAndServe validates config, serves on the configured transport(s),
// blocks until ctx is cancelled, then runs graceful shutdown.
func (s *Server) ListenAndServe(ctx context.Context) error {
	serve := func() error {
		if err := s.validate(); err != nil {
			return err
		}
		switch s.transport {
		case Stdio:
			return s.serveStdio(ctx)
		case HTTP:
			return s.runHTTP(ctx, s.newHTTPServer())
		case Both:
			return s.serveBoth(ctx)
		default:
			return fmt.Errorf("%w: %q", ErrInvalidTransport, s.transport)
		}
	}

	return serve()
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

func (s *Server) newHTTPServer() *http.Server {
	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return s.MCP },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true},
	)
	return &http.Server{
		Addr:              s.addr,
		Handler:           handler,
		ReadHeaderTimeout: s.readHeaderTimeout,
	}
}

func (s *Server) runHTTP(ctx context.Context, hs *http.Server) error {
	slog.InfoContext(ctx, "server running on http", "addr", hs.Addr)
	shutdownErrCh := make(chan error, 1)
	stop := context.AfterFunc(ctx, func() { shutdownErrCh <- s.shutdown(hs) })
	if err := hs.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		stop()
		return fmt.Errorf("%w: %w", ErrServe, err)
	}
	if err := <-shutdownErrCh; err != nil {
		return err
	}
	return nil
}

func (s *Server) serveBoth(ctx context.Context) error {
	slog.InfoContext(ctx, "server running on stdio and http", "addr", s.addr)
	bothCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	hs := s.newHTTPServer()
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
