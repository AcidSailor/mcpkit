package openapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/google/jsonschema-go/jsonschema"
)

// Int64String is an int64 that crosses a JSON/MCP boundary as a quoted decimal
// string, so 64-bit ids survive JavaScript clients (whose only number type is a
// float64, exact solely up to 2^53). Decoding is string-only: a bare JSON
// number is already truncated by such a client, so it is rejected rather than
// acted on. Pair the field with Int64StringSchema / StringifyIntParam so the
// advertised schema tells clients to send a string.
type Int64String int64

// Int64 returns the underlying value for passing to int64-typed APIs.
func (v Int64String) Int64() int64 { return int64(v) }

// UnmarshalJSON accepts only a quoted decimal integer; a bare number, null,
// empty, or non-integer string is an error.
func (v *Int64String) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '"' {
		return fmt.Errorf(
			"int64 string: expected a quoted integer, got %s", data,
		)
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
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

// int64StringPattern constrains the advertised string to an optionally-signed
// decimal integer. Range and exact parsing are enforced on decode.
const int64StringPattern = `^-?[0-9]+$`

// Int64StringSchema returns the input schema for an Int64String field: a string
// carrying a decimal integer, keeping the given description.
func Int64StringSchema(description string) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        "string",
		Pattern:     int64StringPattern,
		Description: description,
	}
}

// StringifyIntParam rewrites property name of an assembled object schema to the
// Int64String form, preserving that property's description. It returns s for
// chaining and is a no-op when the property is absent. Use it to fix an
// OpenAPI-derived integer id param (e.g. from ParamsSchema) at the MCP boundary.
func StringifyIntParam(s *jsonschema.Schema, name string) *jsonschema.Schema {
	if p, ok := s.Properties[name]; ok {
		s.Properties[name] = Int64StringSchema(p.Description)
	}
	return s
}
