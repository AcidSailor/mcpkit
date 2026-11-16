// Package server serves an mcp.Server over stdio, HTTP, or both.
//
// HTTP transports require a caller-configured *http.Server. Its handler,
// address, timeouts, TLS settings, and hooks are used unchanged. A non-nil
// TLSConfig selects HTTPS. WithShutdownTimeout controls graceful shutdown.
//
// A stateless streamable HTTP handler supports protocol 2026-07-28 and current
// elicitation. Older clients need stdio or a stateful handler for gated writes.
// Stateful multi-replica deployments need session affinity.
//
// Without WithLogger, or with WithLogger(nil), lifecycle logs are discarded.
// Both runs stdio and HTTP concurrently; either transport stopping cancels the
// other. New defaults to Stdio and a 30-second shutdown timeout.
//
// HTTP and Both require a server, handler, and valid address. Invalid
// configurations return ErrNoHTTPServer, ErrNilHandler, or ErrInvalidAddr.
// HTTPS requires caller-provided certificates.
//
// Prefer StreamableHTTPOptions{Stateless: true, JSONResponse: true}. Stateless
// permits negotiation of protocol 2026-07-28; stateful limits it to 2025-11-25.
// JSONResponse does not affect negotiation. SDK list-result caching applies
// only to stateless sessions; tool-call results are not cached.
package server
