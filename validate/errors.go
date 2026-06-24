package validate

import "errors"

// ErrEmpty is wrapped with the field name when a required value is blank.
var ErrEmpty = errors.New("empty value")

// ErrZero is wrapped with the field name when a required value is the zero T.
var ErrZero = errors.New("zero value")
