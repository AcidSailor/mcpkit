// Package openapi assembles per-tool JSON Schemas from a dereferenced OpenAPI
// 3.1 document.
//
// Input is the JSON of a dereferenced document (every $ref inlined,
// components.schemas retained). Parse decodes it once (returning ErrParse on bad
// JSON; it normalizes OpenAPI 3.0 nullable: true to a null-permitting type). The
// returned *Schemas then composes self-contained schemas: ParamsSchema /
// ParamSchema (optionally a named subset), BodySchema (application/json request
// body), Ref (named component), OutputObject / OutputItems / OutputValue
// (response wrappers), and Summary. Returned schemas are deep-cloned, so callers
// and the SDK may mutate them freely.
//
// Only Parse returns an error. The composition methods instead panic (wrapping
// ErrUndefined) on an unknown name/path/operation/parameter — these are static,
// programmer-level mistakes, surfaced loudly rather than threaded through every
// call site.
package openapi
