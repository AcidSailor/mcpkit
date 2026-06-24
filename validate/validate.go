// Package validate provides small, generic request validators for MCP tools.
package validate

import (
	"fmt"
	"strings"
)

// RequireNonEmpty wraps ErrEmpty, naming field, when value is blank (trimmed).
func RequireNonEmpty(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%w: %s", ErrEmpty, field)
	}
	return nil
}

// RequireNonZero wraps ErrZero, naming field, when value is the zero of T.
func RequireNonZero[T comparable](field string, value T) error {
	var zero T
	if value == zero {
		return fmt.Errorf("%w: %s", ErrZero, field)
	}
	return nil
}
