package resource

import (
	"context"
	"fmt"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yosida95/uritemplate/v3"
)

// Vars holds the variables extracted from a matched URI template.
type Vars struct {
	vals uritemplate.Values
}

// Get returns a string value or "" when the variable is absent or not a string.
func (v Vars) Get(name string) string {
	return v.vals.Get(name).String()
}

// Lookup returns a string value and reports whether the variable is present.
func (v Vars) Lookup(name string) (string, bool) {
	val := v.vals.Get(name)
	return val.String(), val.Valid()
}

// Has reports whether name was present in the matched URI.
func (v Vars) Has(name string) bool {
	return v.vals.Get(name).Valid()
}

// List returns a multi-valued (exploded) variable, e.g. {/path*}.
func (v Vars) List(name string) []string {
	return v.vals.Get(name).List()
}

// Int parses name as an int, wrapping ErrInvalidVars on failure.
func (v Vars) Int(name string) (int, error) {
	s := v.Get(name)
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%w: %q=%q", ErrInvalidVars, name, s)
	}
	return n, nil
}

// TemplateReadFunc reads a concrete URI and its template variables.
type TemplateReadFunc func(
	ctx context.Context,
	uri string,
	vars Vars,
) (Content, error)

// Template is a value builder for an RFC 6570 resource template.
type Template struct {
	server      *mcp.Server
	uriTemplate string
	name        string
	description string
	readFunc    TemplateReadFunc

	tmpl        *uritemplate.Template
	mimeType    string
	title       string
	annotations *mcp.Annotations
}

// NewTemplate creates a builder and panics if uriTemplate is invalid.
func NewTemplate(
	server *mcp.Server,
	uriTemplate, name, description string,
	read TemplateReadFunc,
) Template {
	parsed, err := uritemplate.New(uriTemplate)
	if err != nil {
		panic(fmt.Errorf("uri template %q: %w", uriTemplate, err))
	}
	return Template{
		server:      server,
		uriTemplate: uriTemplate,
		name:        name,
		description: description,
		readFunc:    read,
		tmpl:        parsed,
	}
}

// WithMIMEType sets the MIME type shared by resources matching the template.
func (t Template) WithMIMEType(mime string) Template {
	t.mimeType = mime
	return t
}

// WithTitle sets the template's human-readable title.
func (t Template) WithTitle(title string) Template {
	t.title = title
	return t
}

// WithDescription overrides the description passed to NewTemplate.
func (t Template) WithDescription(desc string) Template {
	t.description = desc
	return t
}

// WithAnnotations attaches client annotations to the template.
func (t Template) WithAnnotations(a *mcp.Annotations) Template {
	t.annotations = a
	return t
}

// Add registers the template on the server.
func (t Template) Add() {
	tmpl := &mcp.ResourceTemplate{
		Name:        t.name,
		Title:       t.title,
		Description: t.description,
		MIMEType:    t.mimeType,
		URITemplate: t.uriTemplate,
		Annotations: t.annotations,
	}
	t.server.AddResourceTemplate(tmpl, t.handler())
}

func (t Template) handler() mcp.ResourceHandler {
	return func(
		ctx context.Context,
		req *mcp.ReadResourceRequest,
	) (*mcp.ReadResourceResult, error) {
		uri := req.Params.URI
		vals := t.tmpl.Match(uri)
		if vals == nil {
			// Defensive: the SDK only routes a matching URI here.
			return nil, toWireErr(
				uri,
				fmt.Errorf("%s: %w", t.name, ErrTemplateMismatch),
			)
		}
		c, err := t.readFunc(ctx, uri, Vars{vals: vals})
		if err != nil {
			return nil, toWireErr(uri, fmt.Errorf("%s: %w", t.name, err))
		}
		if c == nil {
			return nil, fmt.Errorf("%s: %w", t.name, ErrNoContent)
		}
		conts, err := c.contents(uri, t.mimeType)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", t.name, err)
		}
		return &mcp.ReadResourceResult{Contents: conts}, nil
	}
}
