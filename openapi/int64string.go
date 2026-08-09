package openapi

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/google/jsonschema-go/jsonschema"
)

// Int64String encodes an int64 as a decimal string without precision loss.
type Int64String int64

// Int64 returns the underlying value for passing to int64-typed APIs.
func (v Int64String) Int64() int64 { return int64(v) }

// UnmarshalJSON accepts only a quoted decimal int64.
func (v *Int64String) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf(
			"int64 string: expected a quoted integer: %w", err,
		)
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("int64 string: %w", err)
	}
	*v = Int64String(n)
	return nil
}

// MarshalJSON emits the value as a quoted decimal string.
func (v Int64String) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(strconv.FormatInt(int64(v), 10))), nil
}

// int64StringPattern describes decimal int64 input without encoding its range.
const int64StringPattern = `^-?[0-9]+$`

// Int64StringSchema returns a decimal-string schema with description.
func Int64StringSchema(description string) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        "string",
		Pattern:     int64StringPattern,
		Description: description,
	}
}

// StringifyIntParam rewrites a property as Int64String and preserves its text.
func StringifyIntParam(s *jsonschema.Schema, name string) *jsonschema.Schema {
	p, ok := s.Properties[name]
	if !ok {
		panic(fmt.Errorf("%w: property %q", ErrUndefined, name))
	}
	rewritten := Int64StringSchema(p.Description)
	rewritten.Title = p.Title
	s.Properties[name] = rewritten
	return s
}
