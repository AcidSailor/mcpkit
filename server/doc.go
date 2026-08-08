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
// choice bounds which MCP protocol a session can negotiate. Stateless: true is
// a precondition for 2026-07-28 — the SDK rejects new-protocol requests on a
// stateful handler — while Stateless: false caps every session at 2025-11-25
// via the legacy initialize handshake. The client still picks within that
// bound, so a stateless handler serves both generations. JSONResponse does not
// affect the negotiation.
//
// Elicitation-gated write tools (toolkit.AddWrite / registry.Write) are served
// by different mechanisms per generation. On 2026-07-28 the confirmation is a
// multi-round-trip request (SEP-2322): the server answers with an input-needed
// result and the client retries, so nothing is held between the two passes, and
// the client's capabilities ride each request's _meta. On the legacy protocol
// the SDK's server-side shim elicits over the live session instead — which is
// the path that needs one, so a pre-2026-07-28 client on a stateless handler
// gets elicit.ErrNoElicitation and cannot run write tools at all.
//
// Prefer mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true}. It
// serves write tools to current clients, scales horizontally without session
// affinity, and is the only mode in which the SDK's client-side caching of list
// results (ttlMs, SEP-2549) is active; tool-call results are not cached. Reach
// for Stateless: false only to serve clients predating 2026-07-28 — they cannot
// retry, and the shim needs a live session — or for an EventStore aiding stream
// resumption. Stateful sessions live in-process in the SDK transport, so
// multi-replica deployments on that path need session affinity (sticky
// routing).
//
// # Errors
//
// Following the stdlib convention (no package.Err umbrella), the package
// declares its own sentinels in errors.go and wraps them with detail via
// fmt.Errorf("%w: …", …) at the entry point, preserving errors.Is matching.
package server
