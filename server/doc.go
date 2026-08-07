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
// # Stateless is the modern path
//
// The handler's mcp.StreamableHTTPOptions are the caller's choice, and that
// choice decides which MCP protocol a session negotiates. With Stateless: true
// the SDK speaks 2026-07-28; with Stateless: false it falls back to the legacy
// initialize handshake, which caps the negotiated version at 2025-11-25.
// JSONResponse does not affect the negotiation.
//
// Elicitation-gated write tools (toolkit.AddWrite / registry.Write) are served
// in both modes, by different mechanisms. On 2026-07-28 the confirmation is a
// multi-round-trip request (SEP-2322): the server answers with an input-required
// result and the client retries, so nothing is held between the two passes. On
// the legacy protocol the SDK's server-side shim elicits over the live session
// instead — which is the path that needs one.
//
// Prefer mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true}. It
// serves write tools, scales horizontally without session affinity, and is the
// only mode in which the SDK's client-side result caching (ttlMs, SEP-2549) is
// active. Reach for Stateless: false only to serve clients predating 2026-07-28
// — they cannot retry, and the shim's server->client elicitation is unavailable
// in stateless mode — or for an EventStore aiding stream resumption. Stateful
// sessions live in-process in the SDK transport, so multi-replica deployments
// on that path need session affinity (sticky routing).
//
// # Errors
//
// Following the stdlib convention (no package.Err umbrella), the package
// declares its own sentinels in errors.go and wraps them with detail via
// fmt.Errorf("%w: …", …) at the entry point, preserving errors.Is matching.
package server
