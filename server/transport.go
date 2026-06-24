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

// ParseTransport parses s, wrapping ErrInvalidTransport for a bad value.
func ParseTransport(s string) (Transport, error) {
	switch t := Transport(s); t {
	case Stdio, HTTP, Both:
		return t, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidTransport, s)
	}
}

// UnmarshalText lets text config loaders (env, flag, json) parse a Transport.
func (t *Transport) UnmarshalText(b []byte) error {
	parsed, err := ParseTransport(string(b))
	if err != nil {
		return err
	}
	*t = parsed
	return nil
}
