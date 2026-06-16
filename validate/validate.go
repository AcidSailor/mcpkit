// Package validate provides small, generic request validators for MCP tools.
package validate

import (
	"fmt"
	"strings"
)

// RequireNonEmpty returns an error wrapping ErrEmpty, naming field, when value
// is blank after trimming whitespace.
func RequireNonEmpty(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%w: %s", ErrEmpty, field)
	}
	return nil
}

// RequireNonZero returns an error wrapping ErrZero, naming field, when value
// equals the zero value for its type. It complements RequireNonEmpty for
// non-string required inputs such as numeric ids.
func RequireNonZero[T comparable](field string, value T) error {
	var zero T
	if value == zero {
		return fmt.Errorf("%w: %s", ErrZero, field)
	}
	return nil
}
