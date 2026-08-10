package toolkit

import (
	"errors"

	"github.com/acidsailor/mcpkit/elicit"
)

// Registration errors panic when tool settings conflict with access.
var (
	ErrElicitOnRead    = errors.New("elicitation set on a read-only tool")
	ErrGateIDOnRead    = errors.New("gate id set on a read-only tool")
	ErrDestructiveRead = errors.New(
		"DestructiveHint true on a read-only tool",
	)
	ErrReadOnlyMismatch = errors.New(
		"ReadOnlyHint does not match the tool's access " +
			"(set it true on a read, false on a write)",
	)
)

// Elicitation errors are aliases for the elicit package sentinels.
var (
	ErrUserDeclined           = elicit.ErrUserDeclined
	ErrUserCanceled           = elicit.ErrUserCanceled
	ErrUnexpectedElicitAction = elicit.ErrUnexpectedElicitAction
	ErrElicitationFailed      = elicit.ErrElicitationFailed
)
