package openapi

import "errors"

// ErrParse is wrapped with the decode error when Parse cannot decode the doc.
var ErrParse = errors.New("openapi parse")

// ErrUndefined is wrapped in the panic value for an unknown name/path/op/param.
var ErrUndefined = errors.New("openapi undefined")
