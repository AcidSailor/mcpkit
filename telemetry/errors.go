package telemetry

import "errors"

var (
	// ErrNilTracerProvider reports a Middleware call without a provider.
	ErrNilTracerProvider = errors.New("nil tracer provider")
	// ErrNilPropagator reports a nil propagator passed to WithPropagator.
	ErrNilPropagator = errors.New("nil propagator")
)
