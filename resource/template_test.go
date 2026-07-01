package resource_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/acidsailor/mcpkit/mcptest"
	"github.com/acidsailor/mcpkit/resource"
)

func TestTemplate_SingleVar(t *testing.T) {
	s := newServer(t)
	resource.NewTemplate(s, "users://{userID}/profile", "profile",
		"a user profile",
		func(_ context.Context, _ string, v resource.Vars) (
			resource.Content, error,
		) {
			return resource.NewText(v.Get("userID")), nil
		}).Add()

	cs := mcptest.NewSession(t, s)
	assert.Equal(t, "42",
		mcptest.ReadResourceText(t, cs, "users://42/profile"))
}

func TestTemplate_MultiVar(t *testing.T) {
	s := newServer(t)
	resource.NewTemplate(s, "repos://{owner}/{repo}", "repo", "a repo",
		func(_ context.Context, _ string, v resource.Vars) (
			resource.Content, error,
		) {
			return resource.NewText(v.Get("owner") + "/" + v.Get("repo")), nil
		}).Add()

	cs := mcptest.NewSession(t, s)
	assert.Equal(t, "acidsailor/mcpkit",
		mcptest.ReadResourceText(t, cs, "repos://acidsailor/mcpkit"))
}

func TestTemplate_ListVar(t *testing.T) {
	s := newServer(t)
	resource.NewTemplate(s, "files://{/segments*}", "files", "path files",
		func(_ context.Context, _ string, v resource.Vars) (
			resource.Content, error,
		) {
			return resource.NewJSON(v.List("segments")), nil
		}).Add()

	cs := mcptest.NewSession(t, s)
	got := mcptest.ReadResourceJSON[[]string](t, cs, "files:///a/b/c")
	assert.Equal(t, []string{"a", "b", "c"}, got)
}

func TestTemplate_IntVar(t *testing.T) {
	s := newServer(t)
	resource.NewTemplate(s, "items://{id}", "item", "an item",
		func(_ context.Context, _ string, v resource.Vars) (
			resource.Content, error,
		) {
			n, err := v.Int("id")
			if err != nil {
				return nil, err
			}
			return resource.NewText(fmt.Sprintf("n=%d", n*2)), nil
		}).Add()

	cs := mcptest.NewSession(t, s)
	assert.Equal(t, "n=14", mcptest.ReadResourceText(t, cs, "items://7"))
}

func TestTemplate_IntVarInvalid(t *testing.T) {
	s := newServer(t)
	resource.NewTemplate(s, "items://{id}", "item", "an item",
		func(_ context.Context, _ string, v resource.Vars) (
			resource.Content, error,
		) {
			_, err := v.Int("id")
			return nil, err
		}).Add()

	cs := mcptest.NewSession(t, s)
	_, err := cs.ReadResource(context.Background(),
		&mcp.ReadResourceParams{URI: "items://abc"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid template variable")
}

func TestTemplate_HasReportsPresence(t *testing.T) {
	s := newServer(t)
	resource.NewTemplate(s, "items://{id}", "item", "an item",
		func(_ context.Context, _ string, v resource.Vars) (
			resource.Content, error,
		) {
			return resource.NewText(fmt.Sprintf("%t,%t",
				v.Has("id"), v.Has("missing"))), nil
		}).Add()

	cs := mcptest.NewSession(t, s)
	assert.Equal(t, "true,false",
		mcptest.ReadResourceText(t, cs, "items://7"))
}

func TestTemplate_LookupDistinguishesAbsence(t *testing.T) {
	s := newServer(t)
	resource.NewTemplate(s, "items://{id}", "item", "an item",
		func(_ context.Context, _ string, v resource.Vars) (
			resource.Content, error,
		) {
			got, gotOK := v.Lookup("id")
			_, missOK := v.Lookup("missing")
			return resource.NewText(fmt.Sprintf("%s,%t,%t",
				got, gotOK, missOK)), nil
		}).Add()

	cs := mcptest.NewSession(t, s)
	assert.Equal(t, "7,true,false",
		mcptest.ReadResourceText(t, cs, "items://7"))
}

func TestTemplate_NotFoundMapsToWireCode(t *testing.T) {
	s := newServer(t)
	resource.NewTemplate(s, "items://{id}", "item", "an item",
		func(_ context.Context, _ string, _ resource.Vars) (
			resource.Content, error,
		) {
			return nil, resource.ErrNotFound
		}).Add()

	cs := mcptest.NewSession(t, s)
	_, err := cs.ReadResource(context.Background(),
		&mcp.ReadResourceParams{URI: "items://9"})
	require.Error(t, err)
	var werr *jsonrpc.Error
	require.ErrorAs(t, err, &werr)
	assert.Equal(t, int64(mcp.CodeResourceNotFound), werr.Code)
}

func TestTemplate_ListAdvertised(t *testing.T) {
	s := newServer(t)
	resource.NewTemplate(s, "items://{id}", "item", "an item",
		func(_ context.Context, _ string, _ resource.Vars) (
			resource.Content, error,
		) {
			return resource.NewText("x"), nil
		}).WithMIMEType("text/plain").Add()

	cs := mcptest.NewSession(t, s)
	assert.Equal(t, []string{"items://{id}"},
		mcptest.ListResourceTemplateURIs(t, cs))
}

func TestTemplate_InvalidTemplatePanics(t *testing.T) {
	s := newServer(t)
	assert.Panics(t, func() {
		resource.NewTemplate(s, "items://{id", "bad", "x",
			func(_ context.Context, _ string, _ resource.Vars) (
				resource.Content, error,
			) {
				return resource.NewText("x"), nil
			})
	})
}
