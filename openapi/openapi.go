// Package openapi assembles per-tool JSON Schemas for MCP tools from a
// dereferenced OpenAPI document.
//
// The input is the JSON of an OpenAPI 3.1 document whose Schema Objects are
// already JSON Schema 2020-12 (except OpenAPI 3.0's `nullable: true`, which
// Parse normalizes to a null-permitting type) and is fully dereferenced: every
// "#/components/schemas/X" reference inlined while components.schemas is
// retained for name->schema lookups. With no "$ref" in the document, every
// assembled schema is self-contained — no "$defs" attachment and nothing for
// the MCP SDK's JSON Schema resolver to chase.
//
// Parse unmarshals the document once into typed jsonschema.Schema values; the
// Schemas methods then compose self-contained schemas. They expose an
// operation's parameters (ParamsSchema/ParamSchema, optionally a named subset),
// its application/json request body (BodySchema), a named component (Ref), a
// response wrapper (OutputObject/OutputItems/OutputValue), and its summary
// (Summary). Returned component and parameter schemas are deep-cloned (via
// jsonschema's CloneSchemas), so they never alias the parsed document and
// callers — and the MCP SDK — may mutate them freely.
//
// Methods panic on a name/path/operation/parameter the document does not
// define: static, programmer-level errors that surface immediately in tests.
// The panic value wraps ErrUndefined. Only Parse, whose input is runtime bytes,
// returns an error.
package openapi

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/google/jsonschema-go/jsonschema"
)

type document struct {
	Paths      map[string]map[string]operation `json:"paths"`
	Components struct {
		Schemas map[string]*jsonschema.Schema `json:"schemas"`
	} `json:"components"`
}

type operation struct {
	Summary     string       `json:"summary"`
	Parameters  []parameter  `json:"parameters"`
	RequestBody *requestBody `json:"requestBody"`
}

type parameter struct {
	Name     string             `json:"name"`
	In       string             `json:"in"`
	Required bool               `json:"required"`
	Schema   *jsonschema.Schema `json:"schema"`
}

type requestBody struct {
	Content map[string]mediaType `json:"content"`
}

type mediaType struct {
	Schema *jsonschema.Schema `json:"schema"`
}

// Schemas assembles per-tool JSON Schemas from a parsed OpenAPI document.
type Schemas struct {
	paths map[string]map[string]operation
	defs  map[string]*jsonschema.Schema
}

// Parse decodes a dereferenced OpenAPI document into a Schemas. It returns an
// error wrapping ErrParse when doc is not valid JSON.
func Parse(doc []byte) (*Schemas, error) {
	var d document
	if err := json.Unmarshal(doc, &d); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrParse, err)
	}
	// OpenAPI 3.0's `nullable: true` keyword is undefined in JSON
	// Schema 2020-12 — jsonschema parses it into Extra but ignores it when
	// validating, so a JSON null would be rejected for a spec-nullable value.
	// Rewrite each nullable schema to a null-permitting type up front, so every
	// assembled schema (and its clones) inherits the fix.
	for _, s := range d.Components.Schemas {
		applyNullable(s)
	}
	for _, methods := range d.Paths {
		for _, op := range methods {
			for _, p := range op.Parameters {
				applyNullable(p.Schema)
			}
			if op.RequestBody != nil {
				for _, mt := range op.RequestBody.Content {
					applyNullable(mt.Schema)
				}
			}
		}
	}
	return &Schemas{paths: d.Paths, defs: d.Components.Schemas}, nil
}

// applyNullable translates OpenAPI 3.0's `nullable: true` into a JSON Schema
// 2020-12 null-permitting type, dropping the now-redundant keyword. It recurses
// through every subschema position, so a nullable field nested under
// properties, items, composition, etc. is covered.
func applyNullable(s *jsonschema.Schema) {
	if s == nil {
		return
	}
	if n, ok := s.Extra["nullable"].(bool); ok && n {
		addNullType(s)
		delete(s.Extra, "nullable")
		if len(s.Extra) == 0 {
			s.Extra = nil
		}
	}

	applyNullable(s.Items)
	applyNullable(s.AdditionalItems)
	applyNullable(s.AdditionalProperties)
	applyNullable(s.PropertyNames)
	applyNullable(s.Contains)
	applyNullable(s.Not)
	applyNullable(s.If)
	applyNullable(s.Then)
	applyNullable(s.Else)
	applyNullable(s.ContentSchema)
	applyNullable(s.UnevaluatedItems)
	applyNullable(s.UnevaluatedProperties)

	for _, list := range [][]*jsonschema.Schema{
		s.ItemsArray,
		s.PrefixItems,
		s.AllOf,
		s.AnyOf,
		s.OneOf,
	} {
		for _, sub := range list {
			applyNullable(sub)
		}
	}
	for _, m := range []map[string]*jsonschema.Schema{
		s.Properties,
		s.PatternProperties,
		s.DependentSchemas,
		s.Defs,
		s.Definitions,
	} {
		for _, sub := range m {
			applyNullable(sub)
		}
	}
}

// addNullType makes a schema admit JSON null. A single Type moves into the
// Types pair (jsonschema forbids setting both); an existing Types list gains
// "null" once. A typeless schema already permits null and is left alone.
func addNullType(s *jsonschema.Schema) {
	if len(s.Types) > 0 {
		if !slices.Contains(s.Types, "null") {
			s.Types = append(s.Types, "null")
		}
		return
	}
	if s.Type == "" || s.Type == "null" {
		return
	}
	s.Types = []string{s.Type, "null"}
	s.Type = ""
}

// Ref returns a deep clone of the inlined component schema named by name,
// independent of the parsed document. (The document is pre-dereferenced, so
// there is no "$ref" to emit.) It panics if name is not a known component.
func (s *Schemas) Ref(name string) *jsonschema.Schema {
	comp, ok := s.defs[name]
	if !ok {
		panic(
			fmt.Errorf(
				"%w: unknown component %q",
				ErrUndefined,
				name,
			),
		)
	}
	return comp.CloneSchemas()
}

// Object assembles a self-contained object schema from properties and required
// field names. An empty required slice is omitted from the schema.
func Object(
	properties map[string]*jsonschema.Schema,
	required []string,
) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
		Required:   required,
	}
}

// ParamsSchema builds an object input schema from an operation's path + query
// parameters (used by GET tools). With no names it includes every parameter;
// with names, only that subset in the order given — letting a tool expose a
// curated slice of an operation's parameters. A parameter is required exactly
// when the spec marks it so. It panics if the operation is unknown or a
// requested name is not one of its parameters.
func (s *Schemas) ParamsSchema(
	method, path string,
	names ...string,
) *jsonschema.Schema {
	op := s.mustOp(method, path)
	if len(names) == 0 {
		properties := make(map[string]*jsonschema.Schema, len(op.Parameters))
		var required []string
		for _, p := range op.Parameters {
			properties[p.Name] = paramSchema(p)
			if p.Required {
				required = append(required, p.Name)
			}
		}
		return Object(properties, required)
	}

	byName := make(map[string]parameter, len(op.Parameters))
	for _, p := range op.Parameters {
		byName[p.Name] = p
	}
	properties := make(map[string]*jsonschema.Schema, len(names))
	var required []string
	for _, n := range names {
		p, ok := byName[n]
		if !ok {
			panic(
				fmt.Errorf(
					"%w: %s %s has no parameter %q",
					ErrUndefined,
					method,
					path,
					n,
				),
			)
		}
		properties[n] = paramSchema(p)
		if p.Required {
			required = append(required, n)
		}
	}
	return Object(properties, required)
}

// Summary returns an operation's one-line summary, suitable as a tool
// description. It panics if the operation is unknown.
func (s *Schemas) Summary(method, path string) string {
	return s.mustOp(method, path).Summary
}

// BodySchema returns a deep clone of an operation's application/json
// request-body schema (used by POST/PUT tools), self-contained and embeddable
// freely. It panics if the operation is unknown or has no application/json
// request body.
func (s *Schemas) BodySchema(method, path string) *jsonschema.Schema {
	op := s.mustOp(method, path)
	if op.RequestBody != nil {
		if mt, ok := op.RequestBody.Content["application/json"]; ok &&
			mt.Schema != nil {
			return mt.Schema.CloneSchemas()
		}
	}
	panic(
		fmt.Errorf(
			"%w: %s %s has no application/json request body",
			ErrUndefined,
			method,
			path,
		),
	)
}

// ParamSchema returns a single parameter's self-contained schema, for embedding
// inside an Object. It panics if the parameter is not found.
func (s *Schemas) ParamSchema(method, path, name string) *jsonschema.Schema {
	op := s.mustOp(method, path)
	for _, p := range op.Parameters {
		if p.Name == name {
			return paramSchema(p)
		}
	}
	panic(
		fmt.Errorf(
			"%w: %s %s has no parameter %q",
			ErrUndefined,
			method,
			path,
			name,
		),
	)
}

// paramSchema returns an independent clone of a parameter's schema, embeddable
// without aliasing the parsed document. It panics if the parameter carries no
// schema.
func paramSchema(p parameter) *jsonschema.Schema {
	if p.Schema == nil {
		panic(
			fmt.Errorf(
				"%w: param %q has no schema",
				ErrUndefined,
				p.Name,
			),
		)
	}
	return p.Schema.CloneSchemas()
}

// OutputObject builds an output schema for a single-object response whose body
// is the named component, returned inlined (a deep clone via Ref) so the schema
// carries the top-level "type":"object" the MCP SDK requires. It panics if name
// is unknown or the component is not an object.
func (s *Schemas) OutputObject(name string) *jsonschema.Schema {
	comp := s.Ref(name)
	if comp.Type != "object" {
		panic(
			fmt.Errorf(
				"%w: component %q is not an object (type %q)",
				ErrUndefined,
				name,
				comp.Type,
			),
		)
	}
	return comp
}

// OutputItems builds an output schema for a slice response, wrapped under an
// "items" array (the MCP structuredContent contract requires an object root).
func (s *Schemas) OutputItems(name string) *jsonschema.Schema {
	return Object(
		map[string]*jsonschema.Schema{
			"items": {Type: "array", Items: s.Ref(name)},
		},
		[]string{"items"},
	)
}

// OutputValue builds an output schema for a scalar response, wrapped under a
// "value" property of the given JSON Schema type (e.g. "number").
func OutputValue(jsonType string) *jsonschema.Schema {
	return Object(
		map[string]*jsonschema.Schema{
			"value": {Type: jsonType},
		},
		[]string{"value"},
	)
}

func (s *Schemas) mustOp(method, path string) operation {
	methods, ok := s.paths[path]
	if !ok {
		panic(
			fmt.Errorf(
				"%w: unknown path %q",
				ErrUndefined,
				path,
			),
		)
	}
	op, ok := methods[method]
	if !ok {
		panic(
			fmt.Errorf(
				"%w: %s has no %s operation",
				ErrUndefined,
				path,
				method,
			),
		)
	}
	return op
}
