// Package telemetry traces MCP requests with OpenTelemetry.
//
// Middleware starts one server span per request and follows the MCP
// semantic conventions. Install it with Server.AddReceivingMiddleware.
// Notifications are not traced.
//
// The caller owns the OpenTelemetry setup. Middleware takes a
// TracerProvider and never reads the global provider or propagator. The
// package configures no exporter, resource, or sampler.
//
// The span continues a trace that the client sends in params._meta, not in
// transport headers: one HTTP request can carry several MCP messages, and
// stdio has no headers. The default propagator reads W3C traceparent and
// tracestate; WithPropagator replaces it. A parent from _meta replaces the
// ambient span, such as an HTTP server span, which becomes a link. Without
// either, each request starts a new trace. Spans from outbound calls made
// with the request context nest under the request span.
//
// A gated write runs twice, so it produces two tools/call spans: one that
// asks and one that acts.
//
// The MCP semantic conventions are in development and live in the
// OpenTelemetry GenAI semantic conventions repository. Attribute names may
// change.
package telemetry
