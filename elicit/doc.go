// Package elicit provides the MCP elicitation gate and write-tool sentinels.
//
// Elicitation fronts a write tool as a multi-round-trip request (SEP-2322). A
// server may not call its client mid-handler, so the gate runs in two passes.
// Ask returns an input-required result carrying the confirmation under a caller
// chosen key (GateID is the default); the client fulfils it and retries the
// same call with the answer attached. The caller reads that answer off the
// retry's InputResponses under the same key, and Decide maps it:
//
//	accept  -> nil (the caller proceeds with the mutating call)
//	decline -> ErrUserDeclined
//	cancel  -> ErrUserCanceled
//	other   -> ErrUnexpectedElicitAction
//
// Decide also rejects a response that is not an elicitation result, with
// ErrElicitationFailed. Only the action gates the call; returned field values
// are not inspected.
//
// # The handler runs twice
//
// Every gated call invokes the handler once to ask and once to act, so whatever
// a caller runs before the gate must be free of side effects. The mutating call
// itself runs only on the second pass.
//
// The SDK bridges both client generations. A client on protocol 2026-07-28 or
// later loops in its own middleware; an older one is served by a server-side
// shim that elicits over the live session and re-invokes the handler. Either
// way the model sees a single tool call.
//
// # What the gate does and does not prove
//
// Ask requires the client to advertise the elicitation capability, returning
// ErrNoElicitation when it does not — but that check runs on the ask alone. A
// call arriving with an input response already under the gate id skips Ask
// entirely and proceeds. The gate therefore proves that a confirmation was
// reported, never that one was requested, rendered, or seen.
//
// mcpkit also leaves the SDK's RequestState unsigned, so nothing binds a retry
// to its ask: the retry carries its own arguments and its own answer, both
// taken at face value. This is deliberate. The client renders the prompt, so a
// hostile one owns the answer whatever the server does; a destructive tool that
// is not idempotent should carry its own idempotency key rather than lean on
// the gate. A server that cannot trust its client needs authentication above
// mcpkit, not a stronger gate. The gate is a guard against an honest client's
// mistakes and a prompt for a human — not an authorization boundary.
//
// # Transports
//
// A stateless HTTP handler serves gated writes to clients on protocol
// 2026-07-28 or later: their capabilities ride each request's _meta, and the
// retry needs no session state. A pre-2026-07-28 client on a stateless handler
// gets ErrNoElicitation instead — the SDK's per-request session carries no
// capabilities, and the shim has no live session to elicit over. Serve those
// clients over stdio or a stateful handler (see package server).
//
// A fulfilment failure — the client's handler erroring, or the SDK's retry cap
// being hit — surfaces as a failed call rather than ErrElicitationFailed,
// since the SDK owns that round trip. ErrElicitationFailed now marks only a
// response that is not an elicitation result.
//
// The sentinels live in errors.go; toolkit re-exports them so callers need not
// import elicit directly.
package elicit
