// Package registry collects MCP tool registrations as plain descriptors and
// binds them to a server in one pass, so the catalogue can be enumerated and
// filtered without a live server. Write tools are gated behind Enable.Write.
//
// Resources and resource templates (Resource / ResourceTemplate) are read-only
// and always bind, regardless of Enable.Write.
package registry
