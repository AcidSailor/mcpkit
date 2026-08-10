package elicit

import "errors"

// Sentinels for write-tool elicitation outcomes, wrapped with detail.
var (
	ErrUserDeclined           = errors.New("declined by user")
	ErrUserCanceled           = errors.New("canceled by user")
	ErrUnexpectedElicitAction = errors.New("unexpected elicit action")
	ErrElicitationFailed      = errors.New("elicitation failed")
)
