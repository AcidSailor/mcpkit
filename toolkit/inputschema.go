package toolkit

import (
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
)

// InputSchema reflects a JSON Schema from In; panics on reflection failure.
func InputSchema[In any]() *jsonschema.Schema {
	s, err := jsonschema.For[In](nil)
	if err != nil {
		panic(fmt.Errorf("input schema: %w", err))
	}
	return s
}
