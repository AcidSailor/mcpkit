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
// Stateless HTTP supports gated writes for protocol 2026-07-28 and later.
// Earlier clients need stdio or a stateful HTTP handler.
package elicit
