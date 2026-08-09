// Package toolkit registers typed MCP tools with a fluent value builder.
//
// AddRead registers a read-only tool. AddWrite registers a write tool that uses
// MCP elicitation. A gated write validates twice but mutates only after an
// accepted response. Validators must not have side effects. The gate confirms
// user intent; it is not an authorization boundary.
//
// WithAnnotations replaces all default hints. ReadOnlyHint must match the tool
// category, and a read tool must not set DestructiveHint to true. Invalid
// combinations panic during registration.
//
// AddReadFunc and AddWriteFunc register custom handlers without adding
// validation or elicitation. Custom handlers can call Tool.Call or Tool.Gate.
// Bind method values after completing the builder chain because Tool is a value
// type.
//
// MCP structured results require an object root. Items, Value, WrapItems, and
// WrapValue provide object envelopes for slice and scalar results.
package toolkit
