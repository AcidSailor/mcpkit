// Package mcpkit is the module root for shared primitives that build MCP
// servers on the official go-sdk. Functionality lives in subpackages: server
// (transport serving), toolkit (tool registration), resource (static and
// templated resources), registry (server-independent registration), elicit
// (write-tool confirmation), openapi (schema assembly), validate (input
// validators), and mcptest (in-memory test sessions). Each subpackage exports
// its own errors.Is-matchable sentinels.
package mcpkit
