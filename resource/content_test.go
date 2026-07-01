package resource

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestText_MIMEFallback(t *testing.T) {
	// Explicit MIME wins.
	c, err := Text{Text: "hi", MIME: "text/markdown"}.
		contents("u", "text/html")
	require.NoError(t, err)
	require.Len(t, c, 1)
	assert.Equal(t, "u", c[0].URI)
	assert.Equal(t, "text/markdown", c[0].MIMEType)
	assert.Equal(t, "hi", c[0].Text)

	// Falls back to the resource's declared MIME.
	c, _ = NewText("hi").contents("u", "text/html")
	assert.Equal(t, "text/html", c[0].MIMEType)

	// Then to text/plain.
	c, _ = NewText("hi").contents("u", "")
	assert.Equal(t, "text/plain", c[0].MIMEType)
}

func TestBlob_MIMEFallback(t *testing.T) {
	c, err := NewBlob([]byte{1, 2, 3}).contents("u", "")
	require.NoError(t, err)
	assert.Equal(t, "application/octet-stream", c[0].MIMEType)
	assert.Equal(t, []byte{1, 2, 3}, c[0].Blob)

	c, _ = Blob{Data: []byte{1}, MIME: "image/png"}.contents("u", "")
	assert.Equal(t, "image/png", c[0].MIMEType)
}

func TestJSON_MarshalAndFailure(t *testing.T) {
	c, err := NewJSON(map[string]int{"n": 1}).contents("u", "")
	require.NoError(t, err)
	assert.Equal(t, "application/json", c[0].MIMEType)
	assert.JSONEq(t, `{"n":1}`, c[0].Text)

	// A channel cannot be marshalled: the error surfaces honestly.
	_, err = JSON[chan int]{Value: make(chan int)}.contents("u", "")
	require.Error(t, err)
}

func TestRaw_Passthrough(t *testing.T) {
	want := []*mcp.ResourceContents{{URI: "x", Text: "verbatim"}}
	c, err := Raw{Contents: want}.contents("ignored", "ignored")
	require.NoError(t, err)
	assert.Equal(t, want, c)
}
