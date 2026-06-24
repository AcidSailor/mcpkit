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

// Parse decodes a dereferenced OpenAPI doc; wraps ErrParse on invalid JSON.
func Parse(doc []byte) (*Schemas, error) {
	var d document
	if err := json.Unmarshal(doc, &d); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrParse, err)
	}
	// Rewrite OpenAPI 3.0 `nullable: true` to a null-permitting type up front.
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

// applyNullable rewrites `nullable: true` to a null-permitting type, recursing.
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

// addNullType makes a schema admit JSON null; a typeless schema is left alone.
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

// Ref returns a deep clone of the component named by name; panics if unknown.
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

// Object assembles an object schema from properties and required field names.
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

// ParamsSchema builds an input schema from an op's params (all, or a named
// subset); panics if the op is unknown or a requested name is not a param.
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

// Summary returns an operation's one-line summary; panics if the op is unknown.
func (s *Schemas) Summary(method, path string) string {
	return s.mustOp(method, path).Summary
}

// BodySchema clones an op's application/json request-body schema; panics if the
// op is unknown or has no application/json request body.
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

// ParamSchema returns a single parameter's schema; panics if not found.
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

// paramSchema clones a parameter's schema; panics if it carries no schema.
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

// OutputObject builds an object-response output schema from the named component;
// panics if name is unknown or the component is not an object.
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

// OutputItems builds a slice-response output schema, wrapped under "items".
func (s *Schemas) OutputItems(name string) *jsonschema.Schema {
	return Object(
		map[string]*jsonschema.Schema{
			"items": {Type: "array", Items: s.Ref(name)},
		},
		[]string{"items"},
	)
}

// OutputValue builds a scalar-response output schema, wrapped under "value".
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
