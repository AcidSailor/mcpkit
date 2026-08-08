package toolkit

import (
	"errors"

	"github.com/acidsailor/mcpkit/elicit"
)

// Registration-time programmer errors; each is raised as a panic, wrapped with
// the tool name, when a builder's config contradicts the tool's access.
var (
	ErrElicitOnRead     = errors.New("elicitation set on a read-only tool")
	ErrDestructiveRead  = errors.New("DestructiveHint set on a read-only tool")
	ErrReadOnlyMismatch = errors.New(
		"ReadOnlyHint does not match the tool's access",
	)
)

// Re-exported so callers can match elicit sentinels without importing elicit.
var (
	ErrUserDeclined           = elicit.ErrUserDeclined
	ErrUserCanceled           = elicit.ErrUserCanceled
	ErrNoElicitation          = elicit.ErrNoElicitation
	ErrUnexpectedElicitAction = elicit.ErrUnexpectedElicitAction
	ErrElicitationFailed      = elicit.ErrElicitationFailed
)
