package resource

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ReadFunc reads a static resource's content.
type ReadFunc func(ctx context.Context) (Content, error)

// Resource is a fluent registration builder, distinct from the SDK's
// mcp.Resource. It is a value type — builder methods return a copy.
type Resource struct {
	server      *mcp.Server
	uri         string
	name        string
	description string
	readFunc    ReadFunc

	mimeType    string
	title       string
	size        int64
	annotations *mcp.Annotations
}

// New starts a static-resource registration.
func New(
	server *mcp.Server,
	uri, name, description string,
	read ReadFunc,
) Resource {
	return Resource{
		server:      server,
		uri:         uri,
		name:        name,
		description: description,
		readFunc:    read,
	}
}

// WithMIMEType sets the resource's declared MIME type.
func (r Resource) WithMIMEType(mime string) Resource {
	r.mimeType = mime
	return r
}

// WithTitle sets the resource's human-readable title.
func (r Resource) WithTitle(title string) Resource {
	r.title = title
	return r
}

// WithDescription overrides the description passed to New.
func (r Resource) WithDescription(desc string) Resource {
	r.description = desc
	return r
}

// WithSize sets the raw content size in bytes, advertised to clients.
func (r Resource) WithSize(n int64) Resource {
	r.size = n
	return r
}

// WithAnnotations attaches client annotations to the resource.
func (r Resource) WithAnnotations(a *mcp.Annotations) Resource {
	r.annotations = a
	return r
}

// Add registers the resource on the server. It panics if the URI is malformed
// (fails url.Parse), surfacing the SDK's AddResource panic unchanged.
func (r Resource) Add() {
	res := &mcp.Resource{
		Name:        r.name,
		Title:       r.title,
		Description: r.description,
		MIMEType:    r.mimeType,
		URI:         r.uri,
		Size:        r.size,
		Annotations: r.annotations,
	}
	r.server.AddResource(res, r.handler())
}

func (r Resource) handler() mcp.ResourceHandler {
	return func(
		ctx context.Context,
		_ *mcp.ReadResourceRequest,
	) (*mcp.ReadResourceResult, error) {
		c, err := r.readFunc(ctx)
		if err != nil {
			return nil, toWireErr(r.uri, fmt.Errorf("%s: %w", r.name, err))
		}
		if c == nil {
			return nil, fmt.Errorf("%s: %w", r.name, ErrNoContent)
		}
		conts, err := c.contents(r.uri, r.mimeType)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", r.name, err)
		}
		return &mcp.ReadResourceResult{Contents: conts}, nil
	}
}
