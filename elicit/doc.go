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
// Ask does not inspect client capabilities. The SDK rejects clients that
// cannot answer with a call error that has no matchable sentinel.
//
// Clients must check CallToolResult.NeedsInput. Ignoring InputRequests returns
// an empty successful result without running the write.
//
// Stateless HTTP supports gated writes for protocol 2026-07-28 and later.
// Earlier clients need stdio or a stateful HTTP handler.
//
// BoundedConfirmation checks the message before returning an input request.
// It panics with ErrInvalidLimits when either limit is below 1; oversized
// messages return ErrPromptOverflow and prevent the write. CheckPrompt returns
// ErrInvalidLimits instead of panicking.
//
// DefaultLimits allows 4 lines and 100 runes per line. This targets Claude Code
// 2.1.312, whose collapsed display hides excess lines and clips terminal width.
// Width counts runes, not display columns: CJK, emoji, and tabs can be wider.
// Put the action, object, and irreversible consequences first.
package elicit
