// Package server provides MCP transport serving (stdio/http/both) and the
// Transport type shared across acidsailor MCP servers.
package server

import "fmt"

// Transport selects the MCP transport mechanism.
type Transport string

// Supported transports.
const (
	Stdio Transport = "stdio" // stdin/stdout
	HTTP  Transport = "http"  // streamable HTTP
	Both  Transport = "both"  // stdio + HTTP concurrently
)

// ParseTransport parses s into a Transport, wrapping ErrInvalidTransport for
// an unsupported value.
func ParseTransport(s string) (Transport, error) {
	switch t := Transport(s); t {
	case Stdio, HTTP, Both:
		return t, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidTransport, s)
	}
}

// UnmarshalText lets text-based config loaders (env, flag, json) parse a
// Transport.
func (t *Transport) UnmarshalText(b []byte) error {
	parsed, err := ParseTransport(string(b))
	if err != nil {
		return err
	}
	*t = parsed
	return nil
}
