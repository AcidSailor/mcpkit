// Package resource registers static and RFC 6570 URI-template MCP resources.
//
// Read handlers return Text, Blob, JSON, or Raw content. The package supplies
// the URI and default MIME type except for Raw. Template handlers also receive
// the concrete URI and extracted Vars.
//
// NewTemplate panics on an invalid template, and Resource.Add panics on an
// invalid URI. ErrNotFound and ErrTemplateMismatch become the SDK resource
// not-found error. ErrNoContent reports a nil Content or empty Raw result.
//
// The package does not configure resource subscriptions. Configure subscription
// handlers when constructing the underlying mcp.Server.
//
// Text defaults to text/plain, Blob to application/octet-stream, and JSON uses
// application/json. Raw content is unchanged and must not be empty.
// WithSize applies only to static resources. ErrInvalidVars wraps failed
// Vars.Int conversion. SDK list-change notifications follow add/remove calls.
package resource
