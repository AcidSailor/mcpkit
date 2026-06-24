// Package server wraps an mcp.Server and serves it over stdio, HTTP, or both.
//
// New(mcpServer, opts...) builds a *Server configured by functional Options
// (WithTransport, WithShutdownTimeout, WithHTTPServer). ListenAndServe(ctx)
// validates the config, dispatches on the Transport, blocks until ctx is
// cancelled, then shuts down gracefully. Both runs stdio and HTTP concurrently;
// whichever exits first cancels the other. Transport implements UnmarshalText
// (and ParseTransport), so env/flag/json loaders can parse it. The exported MCP
// field is an escape hatch to the underlying server.
//
// # HTTP is caller-owned
//
// The package owns no HTTP defaults and ships no handler helper. The HTTP and
// Both transports require a caller-built *http.Server via WithHTTPServer (else
// ErrNoHTTPServer), served exactly as given: its Handler, Addr, timeouts,
// ErrorLog, ConnState, TLSConfig, … are all used unchanged. Build the Handler
// with mcp.NewStreamableHTTPHandler — wrap it with middleware (auth, CORS,
// logging) or mount it in a mux alongside other routes (health, metrics). A nil
// Handler is rejected with ErrNilHandler and a malformed Addr with
// ErrInvalidAddr. A non-nil TLSConfig serves HTTPS via ListenAndServeTLS (the
// config must carry its own certificates). Only WithShutdownTimeout (the
// graceful-shutdown deadline, not an http.Server field) is the package's own.
//
// # Elicitation requires a stateful handler
//
// The handler's mcp.StreamableHTTPOptions are the caller's choice, and
// elicitation constrains them. A stateless handler cannot serve write tools
// gated by elicitation (toolkit.AddWrite / registry.Write): it uses a temporary
// session with default init params, so the client's elicitation capability is
// never retained, and it rejects server->client requests — so elicit.Gate fails
// with ErrNoElicitation at call time. Servers with write tools must build a
// stateful handler, mcp.StreamableHTTPOptions{Stateless: false, JSONResponse:
// false}, which keeps the initialized session (Mcp-Session-Id) and serves the
// GET SSE stream so server->client elicitation can be delivered; an optional
// EventStore aids stream resumption. Read-only servers can use {Stateless: true,
// JSONResponse: true}, the only mode that scales horizontally without session
// affinity.
//
// Only Stateless: false is strictly required for elicitation (it retains the
// capability and permits server->client requests); JSONResponse: false is the
// robust companion, letting the elicitation ride the in-flight POST's own SSE
// stream rather than depending on the client holding a standalone GET stream
// open. Stateful sessions live in-process in the SDK transport, so multi-replica
// deployments need session affinity (sticky routing).
//
// # Errors
//
// Following the stdlib convention (no package.Err umbrella), the package
// declares its own sentinels in errors.go and wraps them with detail via
// fmt.Errorf("%w: …", …) at the entry point, preserving errors.Is matching.
package server
