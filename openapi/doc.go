// Package openapi builds tool schemas from a dereferenced OpenAPI document.
//
// Parse accepts JSON and supports OpenAPI 3.0 nullable fields. Schema accessors
// return independent clones. Parse returns ErrParse for invalid JSON. Accessors
// panic with ErrUndefined for unknown document elements.
package openapi
