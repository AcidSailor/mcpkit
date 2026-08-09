package resource

import (
	"cmp"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Default MIME types for Content values.
const (
	// MIMEText is the fallback MIME type for Text content.
	MIMEText = "text/plain"
	// MIMEBlob is the fallback MIME type for Blob content.
	MIMEBlob = "application/octet-stream"
	// MIMEJSON is the fixed MIME type for JSON content.
	MIMEJSON = "application/json"
)

// Content produces one resource read result.
type Content interface {
	// contents creates SDK content with the supplied URI and fallback MIME type.
	contents(uri, fallbackMIME string) ([]*mcp.ResourceContents, error)
}

// Text contains UTF-8 text and an optional MIME override.
type Text struct {
	Text string
	MIME string
}

// Blob contains binary data and an optional MIME override.
type Blob struct {
	Data []byte
	MIME string
}

// JSON marshals Value as text with MIMEJSON.
type JSON[T any] struct {
	Value T
}

// Raw serves non-empty SDK content blocks without modification.
type Raw struct {
	Contents []*mcp.ResourceContents
}

// NewText returns Text that uses the resource MIME or MIMEText.
func NewText(s string) Text { return Text{Text: s} }

// NewBlob returns Blob that uses the resource MIME or MIMEBlob.
func NewBlob(b []byte) Blob { return Blob{Data: b} }

// NewJSON wraps v as JSON content.
func NewJSON[T any](v T) JSON[T] { return JSON[T]{Value: v} }

func (t Text) contents(
	uri, fallback string,
) ([]*mcp.ResourceContents, error) {
	mime := cmp.Or(t.MIME, fallback, MIMEText)
	return []*mcp.ResourceContents{{
		URI: uri, MIMEType: mime, Text: t.Text,
	}}, nil
}

func (b Blob) contents(
	uri, fallback string,
) ([]*mcp.ResourceContents, error) {
	mime := cmp.Or(b.MIME, fallback, MIMEBlob)
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
		URI: uri, MIMEType: MIMEJSON, Text: string(data),
	}}, nil
}

func (r Raw) contents(string, string) ([]*mcp.ResourceContents, error) {
	if len(r.Contents) == 0 {
		return nil, ErrNoContent
	}
	return r.Contents, nil
}
