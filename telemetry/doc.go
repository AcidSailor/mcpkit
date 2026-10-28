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
// The span continues a trace that the client sends in params._meta. The
// default propagator reads W3C traceparent and tracestate; WithPropagator
// replaces it. Without a parent, each request starts a new trace, and spans
// from outbound calls made with the request context nest under it.
//
// A gated write runs twice, so it produces two tools/call spans: one that
// asks and one that acts.
//
// The MCP semantic conventions are in development. Attribute names follow
// semconv v1.41.0 and may change in later versions.
package telemetry
