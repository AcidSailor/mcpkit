package registry

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/acidsailor/mcpkit/resource"
)

// resourceOptions holds the optional resource config captured by the factories.
type resourceOptions struct {
	mime        string
	title       string
	size        int64
	annotations *mcp.Annotations
}

// ResourceOption configures a Resource/ResourceTemplate registration.
type ResourceOption func(*resourceOptions)

// WithMIMEType sets the resource's declared MIME type.
func WithMIMEType(s string) ResourceOption {
	return func(o *resourceOptions) { o.mime = s }
}

// WithTitle sets the resource's human-readable title.
func WithTitle(s string) ResourceOption {
	return func(o *resourceOptions) { o.title = s }
}

// WithSize sets the raw content size in bytes; ignored by ResourceTemplate.
func WithSize(n int64) ResourceOption {
	return func(o *resourceOptions) { o.size = n }
}

// WithAnnotations attaches client annotations to the resource.
func WithAnnotations(a *mcp.Annotations) ResourceOption {
	return func(o *resourceOptions) { o.annotations = a }
}

// Resource describes a static resource bound server-independently. It always
// binds (resources are read-only; Enable.Write does not gate them).
func Resource(
	uri, name, description string,
	read resource.ReadFunc,
	opts ...ResourceOption,
) Registration {
	return Registration{
		Name:   name,
		Access: AccessResource,
		bind: func(s *mcp.Server) {
			buildResource(s, uri, name, description, read, opts).Add()
		},
	}
}

// ResourceTemplate describes a templated resource bound server-independently.
// It always binds. WithSize is ignored (a template has no fixed size).
func ResourceTemplate(
	uriTemplate, name, description string,
	read resource.TemplateReadFunc,
	opts ...ResourceOption,
) Registration {
	return Registration{
		Name:   name,
		Access: AccessResource,
		bind: func(s *mcp.Server) {
			buildTemplate(s, uriTemplate, name, description, read, opts).Add()
		},
	}
}

// buildResource applies opts onto a fresh resource.Resource via the chain.
func buildResource(
	s *mcp.Server,
	uri, name, description string,
	read resource.ReadFunc,
	opts []ResourceOption,
) resource.Resource {
	o := applyResourceOpts(opts)
	r := resource.New(s, uri, name, description, read)
	if o.mime != "" {
		r = r.WithMIMEType(o.mime)
	}
	if o.title != "" {
		r = r.WithTitle(o.title)
	}
	if o.size != 0 {
		r = r.WithSize(o.size)
	}
	if o.annotations != nil {
		r = r.WithAnnotations(o.annotations)
	}
	return r
}

// buildTemplate applies opts onto a fresh resource.Template via the chain.
func buildTemplate(
	s *mcp.Server,
	uriTemplate, name, description string,
	read resource.TemplateReadFunc,
	opts []ResourceOption,
) resource.Template {
	o := applyResourceOpts(opts)
	t := resource.NewTemplate(s, uriTemplate, name, description, read)
	if o.mime != "" {
		t = t.WithMIMEType(o.mime)
	}
	if o.title != "" {
		t = t.WithTitle(o.title)
	}
	if o.annotations != nil {
		t = t.WithAnnotations(o.annotations)
	}
	return t
}

func applyResourceOpts(opts []ResourceOption) resourceOptions {
	var o resourceOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
