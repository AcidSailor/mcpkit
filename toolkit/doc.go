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
//   - AddRead(tool) — registers a read-only tool (ReadOnly + Idempotent hints);
//     panics if an elicit-params func was set (meaningless for reads).
//   - AddWrite(tool) — registers a state-mutating tool (Destructive hint) gated
//     by MCP elicitation: the client must support elicitation (else
//     ErrNoElicitation); the call runs only on an accept action (decline ->
//     ErrUserDeclined, cancel -> ErrUserCanceled).
//
// toolkit re-exports the elicit sentinels (ErrUserDeclined, ErrUserCanceled,
// ErrNoElicitation, ErrUnexpectedElicitAction, ErrElicitationFailed) so callers
// need not import elicit. The shared handler pipeline (runValidated) wraps any
// validator/gate/call error with the tool name via %w, so a validate/elicit
// sentinel raised inside a tool stays matchable after registration.
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
