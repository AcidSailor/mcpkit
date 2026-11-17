// Package openapi builds tool schemas from a dereferenced OpenAPI document.
//
// Parse accepts JSON and supports OpenAPI 3.0 nullable fields. Schema accessors
// return independent clones. Parse returns ErrParse for invalid JSON. Accessors
// panic with ErrUndefined for unknown document elements.
//
// Keep components.schemas and inline every $ref before Parse. OpenAPI 3.0
// nullable fields become null-permitting JSON Schema types. Only Parse returns
// an error; schema lookup failures are configuration errors and panic.
package openapi
