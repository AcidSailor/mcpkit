package resource

import (
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Resource errors are package-specific matchable sentinels.
var (
	// ErrNotFound signals a resource (or templated instance) does not exist.
	ErrNotFound = errors.New("resource not found")
	// ErrTemplateMismatch signals a read URI did not match the URI template.
	ErrTemplateMismatch = errors.New("uri does not match template")
	// ErrInvalidVars signals a template variable could not be converted.
	ErrInvalidVars = errors.New("invalid template variable")
	// ErrNoContent signals a nil Content or empty Raw result.
	ErrNoContent = errors.New("resource produced no content")
)

// toWireErr converts not-found sentinels to the SDK wire error.
func toWireErr(uri string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrTemplateMismatch) {
		return mcp.ResourceNotFoundError(uri)
	}
	return err
}
