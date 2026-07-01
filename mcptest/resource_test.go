package mcptest_test

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/acidsailor/mcpkit/mcptest"
)

// addResource registers a resource serving fixed contents via the raw SDK.
func addResource(s *mcp.Server, uri string, c *mcp.ResourceContents) {
	s.AddResource(
		&mcp.Resource{Name: uri, URI: uri},
		func(context.Context, *mcp.ReadResourceRequest) (
			*mcp.ReadResourceResult, error,
		) {
			c.URI = uri
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{c},
			}, nil
		},
	)
}

func TestResourceHelpers(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	addResource(s, "x://text", &mcp.ResourceContents{Text: "hello"})
	addResource(s, "x://blob", &mcp.ResourceContents{Blob: []byte{1, 2}})
	addResource(s, "x://json", &mcp.ResourceContents{Text: `{"n":3}`})
	s.AddResourceTemplate(
		&mcp.ResourceTemplate{Name: "tmpl", URITemplate: "x://{id}/t"},
		func(context.Context, *mcp.ReadResourceRequest) (
			*mcp.ReadResourceResult, error,
		) {
			return &mcp.ReadResourceResult{}, nil
		},
	)

	cs := mcptest.NewSession(t, s)

	assert.Equal(t, "hello", mcptest.ReadResourceText(t, cs, "x://text"))
	assert.Equal(t, []byte{1, 2}, mcptest.ReadResourceBlob(t, cs, "x://blob"))

	type payload struct {
		N int `json:"n"`
	}
	assert.Equal(t, payload{N: 3},
		mcptest.ReadResourceJSON[payload](t, cs, "x://json"))

	assert.ElementsMatch(t,
		[]string{"x://text", "x://blob", "x://json"},
		mcptest.ListResourceURIs(t, cs))
	require.Equal(t, []string{"x://{id}/t"},
		mcptest.ListResourceTemplateURIs(t, cs))
}
