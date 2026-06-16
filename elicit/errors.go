// Package elicit provides the shared MCP elicitation gate and write-tool
// sentinels used by the toolkit's write-tool builder.
package elicit

import "errors"

// Sentinels for write-tool elicitation outcomes, wrapped with detail at the
// call site.
var (
	ErrUserDeclined  = errors.New("declined by user")
	ErrUserCanceled  = errors.New("canceled by user")
	ErrNoElicitation = errors.New(
		"client must support MCP elicitation for write tools",
	)
	ErrUnexpectedElicitAction = errors.New("unexpected elicit action")
	ErrElicitationFailed      = errors.New("elicitation failed")
)
