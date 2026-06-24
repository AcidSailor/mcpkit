// Package elicit provides the MCP elicitation gate and write-tool sentinels.
//
// Gate runs the elicitation handshake that fronts a write tool. It first
// requires the client to advertise the elicitation capability (else
// ErrNoElicitation), then issues the request and maps the returned action to a
// result:
//
//	accept  -> nil (the caller proceeds with the mutating call)
//	decline -> ErrUserDeclined
//	cancel  -> ErrUserCanceled
//	other   -> ErrUnexpectedElicitAction
//
// A transport/protocol failure is wrapped with ErrElicitationFailed. Only the
// action gates the call; returned field values are not inspected.
//
// ErrNoElicitation has a second, easily-missed cause: a stateless HTTP handler.
// It uses a temporary session with default init params, so the client's
// elicitation capability is never retained and server->client requests are
// rejected — Gate then fails even when the client did advertise the capability.
// Serve write tools over stdio or a stateful HTTP handler (see package server).
//
// The sentinels live in errors.go; toolkit re-exports them so callers need not
// import elicit directly.
package elicit
