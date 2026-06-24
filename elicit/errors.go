package elicit

import "errors"

// Sentinels for write-tool elicitation outcomes, wrapped with detail.
var (
	ErrUserDeclined  = errors.New("declined by user")
	ErrUserCanceled  = errors.New("canceled by user")
	ErrNoElicitation = errors.New(
		"client must support MCP elicitation for write tools " +
			"(a stateless HTTP server handler also drops this capability)",
	)
	ErrUnexpectedElicitAction = errors.New("unexpected elicit action")
	ErrElicitationFailed      = errors.New("elicitation failed")
)
