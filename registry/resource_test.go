package registry_test

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/acidsailor/mcpkit/mcptest"
	"github.com/acidsailor/mcpkit/registry"
	"github.com/acidsailor/mcpkit/resource"
)

func TestRegistry_ResourceRoundTrip(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	reg := registry.New([]registry.Registration{
		registry.Resource("config://name", "name", "the name",
			func(context.Context) (resource.Content, error) {
				return resource.NewText("mcpkit"), nil
			}, registry.WithTitle("Title"), registry.WithSize(6)),
	})
	reg.Bind(s, registry.Enable{Write: false})

	cs := mcptest.NewSession(t, s)
	assert.Equal(t, "mcpkit",
		mcptest.ReadResourceText(t, cs, "config://name"))

	res, err := cs.ListResources(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, res.Resources, 1)
	assert.Equal(t, "Title", res.Resources[0].Title)
	assert.Equal(t, int64(6), res.Resources[0].Size)
}

func TestRegistry_ResourceTemplateRoundTrip(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	reg := registry.New([]registry.Registration{
		registry.ResourceTemplate("users://{userID}/profile", "profile",
			"a profile",
			func(_ context.Context, _ string, v resource.Vars) (
				resource.Content, error,
			) {
				return resource.NewText(v.Get("userID")), nil
			}, registry.WithMIMEType("text/plain")),
	})
	reg.Bind(s, registry.Enable{})

	cs := mcptest.NewSession(t, s)
	assert.Equal(t, "7",
		mcptest.ReadResourceText(t, cs, "users://7/profile"))
	assert.Equal(t, []string{"users://{userID}/profile"},
		mcptest.ListResourceTemplateURIs(t, cs))
}

func TestRegistry_ResourceAllOptions(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	reg := registry.New([]registry.Registration{
		registry.Resource("config://x", "x", "x",
			func(context.Context) (resource.Content, error) {
				return resource.NewText("hi"), nil
			},
			registry.WithMIMEType("text/markdown"),
			registry.WithTitle("Title"),
			registry.WithSize(2),
			registry.WithAnnotations(&mcp.Annotations{Priority: 0.5}),
		),
	})
	reg.Bind(s, registry.Enable{})

	cs := mcptest.NewSession(t, s)
	res, err := cs.ListResources(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, res.Resources, 1)
	r := res.Resources[0]
	assert.Equal(t, "Title", r.Title)
	assert.Equal(t, "text/markdown", r.MIMEType)
	assert.Equal(t, int64(2), r.Size)
	require.NotNil(t, r.Annotations)
	assert.Equal(t, 0.5, r.Annotations.Priority)
}

func TestRegistry_ResourcesBindWithoutWriteEnabled(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	reg := registry.New([]registry.Registration{
		registry.Resource("config://x", "x", "x",
			func(context.Context) (resource.Content, error) {
				return resource.NewText("hi"), nil
			}),
	})
	// Write disabled: resources still bind (AccessResource is not gated).
	reg.Bind(s, registry.Enable{Write: false})

	cs := mcptest.NewSession(t, s)
	assert.Equal(t, []string{"config://x"}, mcptest.ListResourceURIs(t, cs))
}
