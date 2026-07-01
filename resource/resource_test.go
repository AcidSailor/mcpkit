package resource_test

import (
	"context"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/acidsailor/mcpkit/mcptest"
	"github.com/acidsailor/mcpkit/resource"
)

func newServer(t *testing.T) *mcp.Server {
	t.Helper()
	return mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
}

func TestResource_ReadText(t *testing.T) {
	s := newServer(t)
	resource.New(s, "config://name", "name", "the name",
		func(context.Context) (resource.Content, error) {
			return resource.NewText("mcpkit"), nil
		}).WithMIMEType("text/plain").Add()

	cs := mcptest.NewSession(t, s)
	assert.Equal(t, "mcpkit",
		mcptest.ReadResourceText(t, cs, "config://name"))
}

func TestResource_ReadBlob(t *testing.T) {
	s := newServer(t)
	want := []byte{0x89, 0x50, 0x4e, 0x47}
	resource.New(s, "asset://logo", "logo", "a logo",
		func(context.Context) (resource.Content, error) {
			return resource.Blob{Data: want, MIME: "image/png"}, nil
		}).Add()

	cs := mcptest.NewSession(t, s)
	assert.Equal(t, want, mcptest.ReadResourceBlob(t, cs, "asset://logo"))
}

func TestResource_ReadJSON(t *testing.T) {
	type settings struct {
		Theme string `json:"theme"`
		Limit int    `json:"limit"`
	}
	s := newServer(t)
	resource.New(s, "config://settings", "settings", "app settings",
		func(context.Context) (resource.Content, error) {
			return resource.NewJSON(settings{Theme: "dark", Limit: 50}), nil
		}).Add()

	cs := mcptest.NewSession(t, s)
	got := mcptest.ReadResourceJSON[settings](t, cs, "config://settings")
	assert.Equal(t, settings{Theme: "dark", Limit: 50}, got)
}

func TestResource_MIMEDefaultsToTextPlain(t *testing.T) {
	s := newServer(t)
	resource.New(s, "config://x", "x", "x",
		func(context.Context) (resource.Content, error) {
			return resource.NewText("hi"), nil
		}).Add()

	cs := mcptest.NewSession(t, s)
	res, err := cs.ReadResource(context.Background(),
		&mcp.ReadResourceParams{URI: "config://x"})
	require.NoError(t, err)
	assert.Equal(t, "text/plain", res.Contents[0].MIMEType)
}

func TestResource_ListAdvertisesMetadata(t *testing.T) {
	s := newServer(t)
	resource.New(s, "config://x", "x", "described",
		func(context.Context) (resource.Content, error) {
			return resource.NewText("hi"), nil
		}).WithTitle("Title").WithSize(42).Add()

	cs := mcptest.NewSession(t, s)
	res, err := cs.ListResources(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, res.Resources, 1)
	r := res.Resources[0]
	assert.Equal(t, "config://x", r.URI)
	assert.Equal(t, "Title", r.Title)
	assert.Equal(t, "described", r.Description)
	assert.Equal(t, int64(42), r.Size)
}

func TestResource_AnnotationsAdvertised(t *testing.T) {
	s := newServer(t)
	resource.New(s, "config://x", "x", "x",
		func(context.Context) (resource.Content, error) {
			return resource.NewText("hi"), nil
		}).WithAnnotations(&mcp.Annotations{Priority: 0.5}).Add()

	cs := mcptest.NewSession(t, s)
	res, err := cs.ListResources(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, res.Resources, 1)
	require.NotNil(t, res.Resources[0].Annotations)
	assert.Equal(t, 0.5, res.Resources[0].Annotations.Priority)
}

func TestResource_RawMultipleBlocks(t *testing.T) {
	s := newServer(t)
	resource.New(s, "config://multi", "multi", "x",
		func(context.Context) (resource.Content, error) {
			return resource.Raw{Contents: []*mcp.ResourceContents{
				{URI: "config://multi", Text: "first"},
				{URI: "config://multi#2", Text: "second"},
			}}, nil
		}).Add()

	cs := mcptest.NewSession(t, s)
	res, err := cs.ReadResource(context.Background(),
		&mcp.ReadResourceParams{URI: "config://multi"})
	require.NoError(t, err)
	require.Len(t, res.Contents, 2)
	assert.Equal(t, "first", res.Contents[0].Text)
	assert.Equal(t, "second", res.Contents[1].Text)
}

func TestResource_NilContentErrors(t *testing.T) {
	s := newServer(t)
	resource.New(s, "config://nil", "nilres", "x",
		func(context.Context) (resource.Content, error) {
			return nil, nil // handler bug: no content, no error
		}).Add()

	cs := mcptest.NewSession(t, s)
	_, err := cs.ReadResource(context.Background(),
		&mcp.ReadResourceParams{URI: "config://nil"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no content")
}

func TestResource_EmptyRawErrors(t *testing.T) {
	s := newServer(t)
	resource.New(s, "config://empty", "empty", "x",
		func(context.Context) (resource.Content, error) {
			return resource.Raw{}, nil
		}).Add()

	cs := mcptest.NewSession(t, s)
	_, err := cs.ReadResource(context.Background(),
		&mcp.ReadResourceParams{URI: "config://empty"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no content")
}

func TestResource_DescriptionOverride(t *testing.T) {
	s := newServer(t)
	resource.New(s, "config://x", "x", "original",
		func(context.Context) (resource.Content, error) {
			return resource.NewText("hi"), nil
		}).WithDescription("overridden").Add()

	cs := mcptest.NewSession(t, s)
	uris := mcptest.ListResourceURIs(t, cs)
	require.Equal(t, []string{"config://x"}, uris)
	res, err := cs.ListResources(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, "overridden", res.Resources[0].Description)
}

func TestResource_NotFoundMapsToWireCode(t *testing.T) {
	s := newServer(t)
	resource.New(s, "config://missing", "missing", "x",
		func(context.Context) (resource.Content, error) {
			return nil, resource.ErrNotFound
		}).Add()

	cs := mcptest.NewSession(t, s)
	_, err := cs.ReadResource(context.Background(),
		&mcp.ReadResourceParams{URI: "config://missing"})
	require.Error(t, err)
	var werr *jsonrpc.Error
	require.ErrorAs(t, err, &werr)
	assert.Equal(t, int64(mcp.CodeResourceNotFound), werr.Code)
}

func TestResource_ReadErrorPropagates(t *testing.T) {
	s := newServer(t)
	resource.New(s, "config://boom", "boom", "x",
		func(context.Context) (resource.Content, error) {
			return nil, errors.New("kaboom")
		}).Add()

	cs := mcptest.NewSession(t, s)
	_, err := cs.ReadResource(context.Background(),
		&mcp.ReadResourceParams{URI: "config://boom"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestResource_InvalidURIPanics(t *testing.T) {
	s := newServer(t)
	assert.Panics(t, func() {
		resource.New(s, "://not-absolute", "bad", "x",
			func(context.Context) (resource.Content, error) {
				return resource.NewText("hi"), nil
			}).Add()
	})
}
