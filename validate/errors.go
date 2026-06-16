package validate

import "errors"

// ErrEmpty is returned (wrapped with the offending field name) when a required
// value is blank. As a toolkit ValidateFunc, the toolkit further wraps it with
// the tool name, preserving this sentinel for errors.Is.
var ErrEmpty = errors.New("empty value")

// ErrZero is returned (wrapped with the offending field name) when a required
// value equals the zero value for its type, e.g. a numeric id of 0.
var ErrZero = errors.New("zero value")
