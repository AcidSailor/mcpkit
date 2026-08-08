// Package toolkit provides a type-safe fluent builder for registering MCP tools.
//
// New[In, Out](server, name, description, inputSchema, call) infers In/Out from
// call, so generic type params are rarely written at call sites. The input
// schema is required (the SDK panics on nil). Chain optional config, then
// register:
//
//   - WithOutputSchema(schema) — when set, the SDK validates structured results.
//   - WithValidateFunc(f) — runs on decoded input before the call (and before
//     elicitation for writes).
//   - WithElicitParamsFunc(f) — builds the confirmation prompt for write tools.
//   - WithAnnotations(a) — sets the tool's hints (see below).
//   - WithGateID(id) — overrides the confirmation's input-request key,
//     elicit.GateID by default.
//   - AddRead(tool) — registers a read-only tool; panics if an elicit-params
//     func or a gate id was set (both meaningless for reads).
//   - AddWrite(tool) — registers a state-mutating tool gated by MCP
//     elicitation: asking requires the client to support elicitation (else
//     ErrNoElicitation), and the call runs only on an accept action (decline
//     -> ErrUserDeclined, cancel -> ErrUserCanceled). The gate is a prompt for
//     a human, not an authorization boundary — a call that already carries an
//     answer skips the ask (see package elicit).
//
// # Annotations
//
// Without WithAnnotations a tool gets its category's defaults: a read is
// read-only and idempotent, a write is destructive and not. With it, the hints
// are the caller's to own whole — they are checked against the category, then
// used verbatim. Nothing is merged or filled in, so a write that sets only a
// Title omits DestructiveHint, which the spec already reads as true.
//
// The category owns ReadOnlyHint: it is the same choice that decides whether
// the gate wraps the handler and, via registry.Access, whether Enable.Write
// binds the tool at all. A mismatch panics at registration, wrapped with the
// tool name, as does DestructiveHint on a read:
//
//	ErrReadOnlyMismatch — ReadOnlyHint disagrees with AddRead / AddWrite
//	ErrDestructiveRead  — DestructiveHint true on a read-only tool
//
// The SDK types ReadOnlyHint a plain bool, so a read passing annotations must
// set ReadOnlyHint true; an unset one is indistinguishable from an explicit
// false and is rejected rather than silently corrected. DestructiveHint and
// IdempotentHint are marshalled either way, but the spec deems them meaningful
// only when ReadOnlyHint is false — which is where an additive create or an
// idempotent upsert needs to say so.
//
// A gated write runs its handler twice per call — once to ask for confirmation,
// once to act (see package elicit). The validator therefore runs on both passes
// and must be free of side effects; the call func runs only on the second. A
// write registered without WithElicitParamsFunc still asks, with a default
// "Run <name>?" prompt.
//
// AddReadFunc / AddWriteFunc register a custom mcp.ToolHandlerFor as-is,
// keeping the Read/Write annotations: AddReadFunc skips validation, AddWriteFunc
// runs whatever it is given. The two default handlers are exported so a custom
// one can build on them — Call (validate, then call) and Gate (the two-pass
// confirmation); a handler that wraps neither is ungated. Bind either method
// value after the chain is complete: it captures the builder as it was.
//
// toolkit re-exports the elicit sentinels (ErrUserDeclined, ErrUserCanceled,
// ErrNoElicitation, ErrUnexpectedElicitAction, ErrElicitationFailed) so callers
// need not import elicit. Every handler path wraps its errors with the tool
// name via %w (Tool.wrap), so a validate/elicit sentinel raised inside a tool
// stays matchable after registration.
//
// InputSchema[In]() reflects a schema from a plain Go struct via jsonschema.For,
// panicking on failure like mcp.AddTool does. Tool is a value type — builder
// methods return a copy, not a pointer.
//
// Handlers are marshalled as-is (no auto-wrapping), so a handler returning a
// bare slice or scalar would violate MCP's object-root structuredContent
// contract. result.go provides envelopes: Items[T]/Value[T] (shapes {"items":…}
// / {"value":…}) and the WrapItems/WrapValue adapters that consume a
// (slice|scalar, error) pair directly. Items.MarshalJSON normalizes a nil slice
// to [] so an array-typed output schema still accepts it.
package toolkit
