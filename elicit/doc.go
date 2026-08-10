// Package elicit implements confirmation gates for MCP write tools.
//
// Ask returns an input-required result. The client supplies an action and
// retries the call. Decide accepts the action or returns a matchable sentinel.
// The handler therefore runs once to ask and once to act. Work before the gate
// must not have side effects.
//
// A response supplied with the initial call bypasses Ask. RequestState is not
// signed, so the retry is not bound to the original request. The gate records
// reported user intent; it does not authenticate the client or authorize the
// operation. Use authentication and idempotency controls where required.
//
// Ask does not inspect client capabilities. The retry is client-driven and
// needs no back-channel. The SDK still refuses a client that cannot answer:
// its client middleware fails the call for a 2026-07-28 client, and
// ServerSession.Elicit runs the same check for an earlier one. Either way
// the call fails; it does not return a tool error, and no sentinel matches.
//
// A client that ignores InputRequests sees an empty successful result and
// the write does not run. Use CallToolResult.NeedsInput to detect the gate.
//
// Stateless HTTP supports gated writes for protocol 2026-07-28 and later.
// Earlier clients need stdio or a stateful HTTP handler.
package elicit
