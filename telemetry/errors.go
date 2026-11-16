package telemetry

import "errors"

var (
	// ErrNilTracerProvider reports a tracing constructor without a provider.
	ErrNilTracerProvider = errors.New("nil tracer provider")
	// ErrNilPropagator reports a nil propagator passed to WithPropagator.
	ErrNilPropagator = errors.New("nil propagator")
	// ErrNilLogger reports a Log call without a logger.
	ErrNilLogger = errors.New("nil logger")
	// ErrNilLevelFunc reports a nil func passed to WithLevelFunc.
	ErrNilLevelFunc = errors.New("nil level func")
	// ErrNilPayloadFunc reports a nil func passed to WithPayload.
	ErrNilPayloadFunc = errors.New("nil payload func")
)
