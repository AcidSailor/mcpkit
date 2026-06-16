package openapi

import "errors"

// ErrParse is wrapped with the decode error when Parse cannot decode the
// document.
var ErrParse = errors.New("openapi parse")

// ErrUndefined is wrapped in the panic value when a method is asked for a
// name/path/operation/parameter the document does not define. A recovering
// caller can match it with errors.Is(recover().(error), ErrUndefined).
var ErrUndefined = errors.New("openapi undefined")
