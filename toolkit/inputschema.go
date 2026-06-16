package toolkit

import (
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
)

// InputSchema reflects a JSON Schema from the Go input type In, for tools whose
// input is a plain struct with json tags. Panics on reflection failure, like
// mcp.AddTool: an unbuildable schema is a registration-time programming error.
func InputSchema[In any]() *jsonschema.Schema {
	s, err := jsonschema.For[In](nil)
	if err != nil {
		panic(fmt.Errorf("input schema: %w", err))
	}
	return s
}
