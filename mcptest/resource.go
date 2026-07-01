package mcptest

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// readFirst reads uri and returns its first content block, failing the test on
// error or when the read returns no contents.
func readFirst(
	tb testing.TB,
	cs *mcp.ClientSession,
	uri string,
) *mcp.ResourceContents {
	tb.Helper()
	res, err := cs.ReadResource(
		context.Background(),
		&mcp.ReadResourceParams{URI: uri},
	)
	if err != nil {
		tb.Fatalf("ReadResource %q: %v", uri, err)
	}
	if len(res.Contents) == 0 {
		tb.Fatalf("ReadResource %q: no contents", uri)
	}
	return res.Contents[0]
}

// ReadResourceText reads uri and returns the first content's Text.
func ReadResourceText(
	tb testing.TB,
	cs *mcp.ClientSession,
	uri string,
) string {
	tb.Helper()
	return readFirst(tb, cs, uri).Text
}

// ReadResourceBlob reads uri and returns the first content's Blob bytes.
func ReadResourceBlob(
	tb testing.TB,
	cs *mcp.ClientSession,
	uri string,
) []byte {
	tb.Helper()
	return readFirst(tb, cs, uri).Blob
}

// ReadResourceJSON reads uri and unmarshals the first content's Text into T.
func ReadResourceJSON[T any](
	tb testing.TB,
	cs *mcp.ClientSession,
	uri string,
) T {
	tb.Helper()
	var v T
	text := readFirst(tb, cs, uri).Text
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		tb.Fatalf("unmarshal resource %q: %v", uri, err)
	}
	return v
}

// ListResourceURIs returns all advertised static-resource URIs, paginated.
func ListResourceURIs(tb testing.TB, cs *mcp.ClientSession) []string {
	tb.Helper()
	var uris []string
	for r, err := range cs.Resources(context.Background(), nil) {
		if err != nil {
			tb.Fatalf("list resources: %v", err)
		}
		uris = append(uris, r.URI)
	}
	return uris
}

// ListResourceTemplateURIs returns all advertised URI templates, paginated.
func ListResourceTemplateURIs(tb testing.TB, cs *mcp.ClientSession) []string {
	tb.Helper()
	var tmpls []string
	for t, err := range cs.ResourceTemplates(context.Background(), nil) {
		if err != nil {
			tb.Fatalf("list resource templates: %v", err)
		}
		tmpls = append(tmpls, t.URITemplate)
	}
	return tmpls
}
