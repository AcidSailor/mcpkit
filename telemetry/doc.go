// Package telemetry traces and logs MCP requests.
//
// TraceReceiving starts one server span per request. Span names and parenting
// follow the MCP semantic conventions; the span carries a subset of their
// attributes. Install it with Server.AddReceivingMiddleware.
// Incoming notifications get the same span and parent handling.
//
// The caller owns the OpenTelemetry setup. TraceReceiving takes a
// TracerProvider and never reads the global provider or propagator. The
// package configures no exporter, resource, or sampler.
//
// The span continues a trace that the client sends in params._meta, not in
// transport headers: one HTTP request can carry several MCP messages, and
// stdio has no headers. The default propagator reads W3C traceparent and
// tracestate; WithPropagator replaces it, for example with a composite
// propagator that also reads baggage. A parent from _meta replaces the
// ambient span, such as an HTTP server span, which becomes a link. Without
// either, each request starts a new trace. Spans from outbound calls made
// with the request context nest under the request span.
//
// The ambient span is whatever the handler context carries. Stateless
// streamable HTTP gives each request its own context. A stateful session
// keeps the context of the HTTP request that created it, so every later
// request on the session sees that request's span as ambient.
//
// A gated write runs twice. Protocol 2026-07-28 clients resend the call, so
// it produces two tools/call spans: one that asks and one that acts. For
// older clients the SDK shim runs both passes inside one request, which
// gets one span.
//
// TraceSending installs with AddSendingMiddleware on a client or server.
// It starts a client span under the supplied context and injects that span
// into a copy of params._meta. It preserves the concrete SDK request type
// and unrelated metadata, replacing configured propagation fields. Nil
// params get metadata-only params. Notification spans end on send completion,
// without waiting for peer handling. The middleware leaves its input unchanged;
// SDK code outside middleware may still add protocol metadata.
//
// Log works in both directions, including notifications. Return
// emit=false from WithLevelFunc to skip a record. Each record has the
// method or span name as its message, a level, and an outcome:
// ok, input_required, declined, canceled,
// tool_error, or error. It does not repeat what the span holds, such as the
// request attributes, duration, and error. Pass TraceReceiving first and
// Log second in one AddReceivingMiddleware call, or register
// logging first and tracing last in separate calls. For sending, use
// TraceSending in the same position. An OpenTelemetry bridge
// then sets the record's trace and span IDs. Params and results are logged
// only through WithPayload, because they can hold secrets.
//
// The MCP semantic conventions are in development and live in the
// OpenTelemetry GenAI semantic conventions repository. Attribute names may
// change.
//
// # Configuration and callbacks
//
// Both tracing constructors panic with ErrNilTracerProvider for a nil provider
// and ErrNilPropagator for WithPropagator(nil). Log panics with ErrNilLogger
// for a nil logger. WithPayload(nil) and WithLevelFunc(nil) panic with
// ErrNilPayloadFunc and ErrNilLevelFunc respectively.
//
// Outcome.DefaultLevel returns Error for call errors, Warn for tool errors,
// and Info otherwise, including declined and canceled confirmations.
// LevelFunc receives the method, outcome, and call or retained tool error.
// Returning emit=false skips both logging and PayloadFunc. Otherwise the
// logger's Enabled check runs before payload work. Tool errors received from
// a peer have no retained Go error, so their cause can be nil.
//
// PayloadFunc receives nil params for a request without params and a plain nil
// result when the call failed. Payloads are off by default; the caller selects
// and redacts fields. The result returned to the SDK is not changed.
//
// # Errors and SDK details
//
// JSON-RPC errors set error.type and rpc.response.status_code to the code.
// Other call errors use semconv.ErrorType. IsError tool results use tool_error;
// their retained Go error, when present, becomes an exception event. All mark
// the span failed.
//
// MCP keys are declared locally because semconv v1.42.0 moved them to the
// GenAI conventions repository. These spans omit jsonrpc.request.id,
// mcp.protocol.version, and network attributes.
//
// Receiving params may be typed nil; normalize them before calling GetMeta.
// Sending copies only the top-level request, params, and metadata map. Nested
// arguments and content remain shared and must not be mutated by injection.
// A typed-nil params pointer becomes a fresh value of its concrete type.
package telemetry
