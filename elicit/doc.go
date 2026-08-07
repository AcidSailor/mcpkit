// Package elicit provides the MCP elicitation gate and write-tool sentinels.
//
// Elicitation fronts a write tool as a multi-round-trip request (SEP-2322). A
// server may not call its client mid-handler, so the gate runs in two passes.
// Ask returns an input-required result carrying the confirmation under GateID;
// the client fulfils it and retries the same call with the answer attached.
// Response pulls that answer off the retry and Decide maps it:
//
//	accept  -> nil (the caller proceeds with the mutating call)
//	decline -> ErrUserDeclined
//	cancel  -> ErrUserCanceled
//	other   -> ErrUnexpectedElicitAction
//
// Decide also rejects a response that is not an elicitation result, with
// ErrElicitationFailed. Only the action gates the call; returned field values
// are not inspected. Ask still requires the client to advertise the elicitation
// capability, returning ErrNoElicitation when it does not.
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
// way the model sees a single tool call, and the retry needs no session state —
// which is why a stateless HTTP handler can now serve write tools (see package
// server).
//
// # The confirmation is not bound to its arguments
//
// The retry carries its own arguments and mcpkit leaves the SDK's RequestState
// unsigned, so nothing proves the retry's arguments are the ones the user was
// shown. A buggy client or a tampering intermediary could swap them. This is
// deliberate: a hostile client can misreport the answer regardless, being the
// party that renders the prompt, and a destructive tool that is not idempotent
// should carry its own idempotency key rather than lean on the gate.
//
// The sentinels live in errors.go; toolkit re-exports them so callers need not
// import elicit directly.
package elicit
