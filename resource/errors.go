package resource

import (
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Package sentinels, following the repo convention (no umbrella sentinel).
//
// ErrNotFound and ErrTemplateMismatch are return-side sentinels: a read func
// returning either yields mcp.ResourceNotFoundError (CodeResourceNotFound) on
// the wire. Cross-transport errors.Is is not promised — the SDK serializes
// errors to a JSON-RPC code — same caveat as the elicit sentinels.
var (
	// ErrNotFound signals a resource (or templated instance) does not exist.
	ErrNotFound = errors.New("resource not found")
	// ErrTemplateMismatch signals a read URI did not match the URI template.
	ErrTemplateMismatch = errors.New("uri does not match template")
	// ErrInvalidVars signals a template variable could not be converted.
	ErrInvalidVars = errors.New("invalid template variable")
	// ErrNoContent signals a read func produced no content — a nil Content or
	// an empty Raw. It is a real internal error (not not-found), so it passes
	// through toWireErr unchanged rather than masquerading as a missing
	// resource.
	ErrNoContent = errors.New("resource produced no content")
)

// toWireErr maps the not-found sentinels to the SDK's typed not-found error so
// the client sees CodeResourceNotFound; other errors pass through unchanged.
func toWireErr(uri string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrTemplateMismatch) {
		return mcp.ResourceNotFoundError(uri)
	}
	return err
}
