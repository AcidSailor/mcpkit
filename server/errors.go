package server

import "errors"

// Sentinels for server failures, wrapped with detail at the entry point.
var (
	ErrNilServer        = errors.New("nil mcp server")
	ErrInvalidTransport = errors.New("invalid transport")
	ErrInvalidAddr      = errors.New("invalid server address")
	ErrServe            = errors.New("serve")
	ErrShutdown         = errors.New("shutdown")
	ErrNilHandler       = errors.New("nil http handler")
	ErrNoHTTPServer     = errors.New("no http server")
)
