// Package resource provides a type-safe fluent builder for serving MCP
// resources and URI-template (dynamic) resources, mirroring toolkit's style.
//
// New(server, uri, name, description, read) builds a static resource; read is
// a typed func(ctx) (Content, error) — no raw SDK request/result. NewTemplate
// builds a templated resource whose read func also receives the concrete URI
// and the variables extracted from it (the SDK extracts none). Chain optional
// config, then register with Add:
//
//   - WithMIMEType / WithTitle / WithDescription / WithAnnotations — both.
//   - WithSize — static resources only.
//
// Add panics if the URI is malformed (fails url.Parse), and NewTemplate panics
// on an invalid RFC 6570 template — surfacing the SDK's panics, consistent with
// toolkit.InputSchema.
//
// Content envelopes (content.go) spare handlers from building
// []*mcp.ResourceContents by hand, filling the URI and a default MIME type:
// Text (text/plain), Blob (application/octet-stream), JSON[T]
// (application/json), and Raw as a verbatim escape hatch, with NewText /
// NewBlob / NewJSON constructors.
//
// Template variables are exposed as a Vars map (not a typed struct): Get /
// Lookup / Has / List / Int accessors over the matched RFC 6570 values (Lookup
// is the comma-ok form that distinguishes absent from present-but-empty). The
// SDK gives no decode for templates, so a map keeps the surface minimal; a
// typed variant can be added later.
//
// Sentinels (errors.go) follow the repo convention. ErrNotFound and
// ErrTemplateMismatch are return-side sentinels: returning either from a read
// func yields mcp.ResourceNotFoundError (CodeResourceNotFound) on the wire.
// Cross-transport errors.Is is not promised. ErrInvalidVars wraps a failed
// Vars.Int conversion. ErrNoContent is returned when a read func produces no
// content (a nil Content or an empty Raw), surfaced as a real error rather than
// a silent empty read.
//
// Subscriptions (resources/updated) are not yet supported. list-changed works
// for free (the SDK fires it on AddResource/RemoveResources), but
// subscriptions need SubscribeHandler/UnsubscribeHandler set at
// mcp.NewServer construction, which the caller — not this package — owns.
package resource
