// Package registry collects MCP tool registrations as plain descriptors and
// binds them to a server in one pass, so the catalogue can be enumerated and
// filtered without a live server. Write tools are gated behind Enable.Write.
//
// Resources and resource templates (Resource / ResourceTemplate) are read-only
// and always bind, regardless of Enable.Write.
//
// Read and Write take input and output schemas positionally; nil schemas are
// reflected from In/Out. Tool options mirror toolkit. WithToolAnnotations
// configures tool hints; WithAnnotations applies to resources.
// New flattens groups and preserves order.
package registry
