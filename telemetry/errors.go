package telemetry

import "errors"

// ErrNilTracerProvider reports a Middleware call without a provider.
var ErrNilTracerProvider = errors.New("nil tracer provider")
