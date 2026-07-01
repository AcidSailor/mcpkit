package resource

import (
	"cmp"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Content produces the contents of a single resource read. The uri and a
// fallback MIME type are supplied by the package at read time, so handlers
// need not repeat them.
type Content interface {
	// contents is unexported so only this package defines Content shapes;
	// it returns an error so JSON can report marshal failures honestly.
	contents(uri, fallbackMIME string) ([]*mcp.ResourceContents, error)
}

// Text is UTF-8 textual content. MIME overrides the resource's declared
// MIMEType; absent both, it defaults to "text/plain".
type Text struct {
	Text string
	MIME string
}

// Blob is binary content, base64-encoded on the wire by the SDK. MIME
// overrides the resource's MIMEType; absent both, "application/octet-stream".
type Blob struct {
	Data []byte
	MIME string
}

// JSON marshals Value and serves it as text with MIME "application/json"
// unconditionally — the resource's declared MIMEType (and WithMIMEType) do not
// apply to JSON content.
type JSON[T any] struct {
	Value T
}

// Raw is an escape hatch: it serves the given contents verbatim, for handlers
// that need multiple sub-resources or custom per-content metadata. Unlike the
// other shapes, Raw does not stamp the URI or fill a fallback MIME — the caller
// owns each block's fields. Contents must be non-empty; an empty Raw yields
// ErrNoContent rather than a silent empty read.
type Raw struct {
	Contents []*mcp.ResourceContents
}

// NewText wraps s as Text content with a default MIME. To override the MIME
// per content, use a Text struct literal (Text{Text: s, MIME: …}).
func NewText(s string) Text { return Text{Text: s} }

// NewBlob wraps b as Blob content with a default MIME. To override the MIME per
// content, use a Blob struct literal (Blob{Data: b, MIME: …}).
func NewBlob(b []byte) Blob { return Blob{Data: b} }

// NewJSON wraps v as JSON content.
func NewJSON[T any](v T) JSON[T] { return JSON[T]{Value: v} }

func (t Text) contents(
	uri, fallback string,
) ([]*mcp.ResourceContents, error) {
	mime := cmp.Or(t.MIME, fallback, "text/plain")
	return []*mcp.ResourceContents{{
		URI: uri, MIMEType: mime, Text: t.Text,
	}}, nil
}

func (b Blob) contents(
	uri, fallback string,
) ([]*mcp.ResourceContents, error) {
	mime := cmp.Or(b.MIME, fallback, "application/octet-stream")
	return []*mcp.ResourceContents{{
		URI: uri, MIMEType: mime, Blob: b.Data,
	}}, nil
}

func (j JSON[T]) contents(
	uri, _ string,
) ([]*mcp.ResourceContents, error) {
	data, err := json.Marshal(j.Value)
	if err != nil {
		return nil, err
	}
	return []*mcp.ResourceContents{{
		URI: uri, MIMEType: "application/json", Text: string(data),
	}}, nil
}

func (r Raw) contents(string, string) ([]*mcp.ResourceContents, error) {
	if len(r.Contents) == 0 {
		return nil, ErrNoContent
	}
	return r.Contents, nil
}
