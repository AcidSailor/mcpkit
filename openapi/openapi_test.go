package openapi_test

import (
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/acidsailor/mcpkit/openapi"
)

// fixture is a minimal dereferenced OpenAPI 3.1 document covering the methods.
const fixture = `{
  "paths": {
    "/things": {
      "get": {
        "summary": "List things",
        "parameters": [
          {"name": "kind", "in": "query", "required": false,
           "schema": {"type": "string", "enum": ["a", "b"]}},
          {"name": "limit", "in": "query", "required": false,
           "schema": {"type": "integer"}}
        ]
      },
      "post": {
        "summary": "Create a thing",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {"type": "object",
                         "properties": {"name": {"type": "string"}},
                         "required": ["name"]}
            }
          }
        }
      }
    },
    "/things/{id}": {
      "get": {
        "parameters": [
          {"name": "id", "in": "path", "required": true,
           "schema": {"type": "string"}}
        ]
      }
    }
  },
  "components": {
    "schemas": {
      "Thing": {
        "type": "object",
        "properties": {
          "id": {"type": "string"},
          "child": {"type": "object",
                    "properties": {"x": {"type": "number"}}}
        }
      },
      "Count": {"type": "integer"}
    }
  }
}`

func mustParse(t *testing.T) *openapi.Schemas {
	t.Helper()
	s, err := openapi.Parse([]byte(fixture))
	require.NoError(t, err)
	return s
}

func TestParse_Invalid(t *testing.T) {
	_, err := openapi.Parse([]byte("{not json"))
	require.Error(t, err)
	assert.ErrorIs(t, err, openapi.ErrParse)
}

func TestParamsSchema_Optional(t *testing.T) {
	s := mustParse(t).ParamsSchema("get", "/things")
	require.NotNil(t, s.Properties)
	kind := s.Properties["kind"]
	require.NotNil(t, kind, "kind param missing")
	assert.NotNil(t, kind.Enum, "kind enum not carried through")
	// Self-contained: no $defs; optional-only op has no required list.
	assert.Nil(t, s.Defs, "$defs unexpectedly attached")
	assert.Nil(t, s.Required, "unexpected required list")
}

func TestParamsSchema_Required(t *testing.T) {
	s := mustParse(t).ParamsSchema("get", "/things/{id}")
	assert.Contains(t, s.Required, "id", "id should be required")
}

func TestParamSchema(t *testing.T) {
	p := mustParse(t).ParamSchema("get", "/things", "kind")
	assert.NotNil(t, p.Enum, "kind enum not carried through")
}

func TestParamsSchema_Subset(t *testing.T) {
	s := mustParse(t).ParamsSchema("get", "/things", "kind")
	require.NotNil(t, s.Properties["kind"], "selected param missing")
	assert.Nil(t, s.Properties["limit"], "unselected param should be omitted")
	assert.Len(t, s.Properties, 1, "only the selected param should be present")
}

func TestParamsSchema_SubsetUnknownPanics(t *testing.T) {
	s := mustParse(t)
	assert.Panics(t, func() { s.ParamsSchema("get", "/things", "nope") })
}

func TestSummary(t *testing.T) {
	assert.Equal(t, "List things", mustParse(t).Summary("get", "/things"))
}

func TestSummary_UnknownPanics(t *testing.T) {
	s := mustParse(t)
	assert.Panics(t, func() { s.Summary("get", "/nope") })
}

func TestBodySchema(t *testing.T) {
	b := mustParse(t).BodySchema("post", "/things")
	require.Equal(t, "object", b.Type)
	require.NotNil(t, b.Properties["name"])
	assert.Equal(t, "string", b.Properties["name"].Type)
	assert.Equal(t, []string{"name"}, b.Required)

	// Self-contained clone: mutating it must not affect a fresh lookup.
	delete(b.Properties, "name")
	again := mustParse(t).BodySchema("post", "/things")
	assert.NotNil(t, again.Properties["name"], "clone aliased the document")
}

func TestBodySchema_AbsentPanics(t *testing.T) {
	s := mustParse(t)
	defer func() {
		r := recover()
		require.NotNil(t, r, "operation without a JSON body should panic")
		err, ok := r.(error)
		require.True(t, ok, "panic value should be an error")
		assert.ErrorIs(t, err, openapi.ErrUndefined)
	}()
	s.BodySchema("get", "/things") // GET has no request body
}

func TestRef_DeepClonesAndInlines(t *testing.T) {
	s := mustParse(t)
	thing := s.Ref("Thing")
	require.Equal(t, "object", thing.Type)
	child := thing.Properties["child"]
	require.NotNil(t, child, "nested child missing")
	assert.Empty(t, child.Ref, "child still a $ref")
	assert.Equal(t, "object", child.Type, "child not inlined")
	require.NotNil(t, child.Properties["x"], "child.x missing")

	// Mutating the clone must not affect a fresh Ref of the same component.
	delete(thing.Properties, "child")
	again := s.Ref("Thing")
	assert.NotNil(t, again.Properties["child"], "clone aliased the document")
}

func TestRef_UnknownPanics(t *testing.T) {
	s := mustParse(t)
	assert.Panics(t, func() { s.Ref("Nope") })
}

func TestOutputItems(t *testing.T) {
	s := mustParse(t).OutputItems("Thing")
	items := s.Properties["items"]
	require.NotNil(t, items)
	assert.Equal(t, "array", items.Type)
	require.NotNil(t, items.Items)
	assert.Equal(t, "object", items.Items.Type, "inner item not inlined")
	assert.Nil(t, s.Defs, "$defs unexpectedly attached")
}

func TestOutputObject(t *testing.T) {
	s := mustParse(t).OutputObject("Thing")
	assert.Equal(t, "object", s.Type)
	assert.NotNil(t, s.Properties["id"])
}

func TestOutputObject_NonObjectPanics(t *testing.T) {
	s := mustParse(t)
	defer func() {
		r := recover()
		require.NotNil(t, r, "non-object component should panic")
		err, ok := r.(error)
		require.True(t, ok, "panic value should be an error")
		assert.ErrorIs(t, err, openapi.ErrUndefined)
	}()
	s.OutputObject("Count") // integer component, not an object
}

func TestObject(t *testing.T) {
	s := openapi.Object(map[string]*jsonschema.Schema{
		"a": {Type: "string"},
	}, []string{"a"})
	assert.Equal(t, "object", s.Type)
	assert.Equal(t, []string{"a"}, s.Required)
	assert.NotNil(t, s.Properties["a"])
}

func TestOutputValue(t *testing.T) {
	s := openapi.OutputValue("number")
	assert.Equal(t, "object", s.Type)
	require.NotNil(t, s.Properties["value"])
	assert.Equal(t, "number", s.Properties["value"].Type)
	assert.Equal(t, []string{"value"}, s.Required)
}

// nullableFixture exercises "nullable": true at several nesting depths.
const nullableFixture = `{
  "paths": {},
  "components": {
    "schemas": {
      "Sec": {
        "type": "object",
        "properties": {
          "currency": {"type": "string", "nullable": true},
          "nested": {"type": "object", "properties": {
            "yield": {"type": "string", "nullable": true}
          }},
          "tags": {"type": "array",
                   "items": {"type": "string", "nullable": true}}
        }
      }
    }
  }
}`

// TestNullable_TypeRewritten checks "nullable" becomes a null type at depth.
func TestNullable_TypeRewritten(t *testing.T) {
	s, err := openapi.Parse([]byte(nullableFixture))
	require.NoError(t, err)
	sec := s.OutputObject("Sec")

	cur := sec.Properties["currency"]
	require.NotNil(t, cur)
	assert.Empty(t, cur.Type, "single Type should move into Types")
	assert.ElementsMatch(t, []string{"string", "null"}, cur.Types)
	assert.NotContains(
		t,
		cur.Extra,
		"nullable",
		"nullable keyword should be dropped",
	)

	yield := sec.Properties["nested"].Properties["yield"]
	require.NotNil(t, yield)
	assert.ElementsMatch(t, []string{"string", "null"}, yield.Types)

	item := sec.Properties["tags"].Items
	require.NotNil(t, item)
	assert.ElementsMatch(t, []string{"string", "null"}, item.Types)
}

// TestNullable_ValidatesNull proves the rewritten schema accepts a JSON null.
func TestNullable_ValidatesNull(t *testing.T) {
	s, err := openapi.Parse([]byte(nullableFixture))
	require.NoError(t, err)

	resolved, err := s.OutputObject("Sec").Resolve(nil)
	require.NoError(t, err)

	null := map[string]any{
		"currency": nil,
		"nested":   map[string]any{"yield": nil},
		"tags":     []any{nil},
	}
	assert.NoError(t, resolved.Validate(null), "null must be accepted")

	nonNull := map[string]any{"currency": "RUB"}
	assert.NoError(
		t,
		resolved.Validate(nonNull),
		"non-null must still validate",
	)
}

func TestUnknownPathAndOperationPanic(t *testing.T) {
	s := mustParse(t)
	assert.Panics(t, func() { s.ParamsSchema("get", "/nope") }, "unknown path")
	assert.Panics(
		t,
		func() { s.ParamsSchema("delete", "/things") },
		"unknown op",
	)
	assert.Panics(
		t,
		func() { s.ParamSchema("get", "/things", "nope") },
		"unknown param",
	)
}
