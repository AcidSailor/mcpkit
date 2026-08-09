// Package server serves an mcp.Server over stdio, HTTP, or both.
//
// HTTP transports require a caller-configured *http.Server. Its handler,
// address, timeouts, TLS settings, and hooks are used unchanged. A non-nil
// TLSConfig selects HTTPS. WithShutdownTimeout controls graceful shutdown.
//
// A stateless streamable HTTP handler supports protocol 2026-07-28 and current
// elicitation. Older clients need stdio or a stateful handler for gated writes.
// Stateful multi-replica deployments need session affinity.
package server
