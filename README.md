# mcpkit

`mcpkit` provides Go helpers for MCP servers built with the official
[`modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk).
It uses the SDK protocol implementation and adds transport serving, typed tool
and resource registration, elicitation gates, OpenAPI schema assembly, and test
helpers. The only executable is `cmd/mcpbstage`, which stages `.mcpb` bundles.

## Packages

- `server` serves an `*mcp.Server` over stdio, streamable HTTP, or both.
- `toolkit` registers typed read and write tools. Write tools use elicitation.
- `resource` registers static and RFC 6570 URI-template resources.
- `registry` defines tools and resources before binding them to a server.
- `elicit` provides the write-tool gate and confirmation builders.
- `openapi` derives tool schemas from a dereferenced OpenAPI document.
- `validate` provides input validators with matchable error sentinels.
- `telemetry` traces MCP requests with a caller-supplied OpenTelemetry
  `TracerProvider` and logs them to a caller-supplied `*slog.Logger`.
- `mcptest` creates in-memory client and server sessions for tests.

## Requirements

- Go 1.27 or later

## Installation

```sh
go get github.com/acidsailor/mcpkit
```

Import only the required subpackages:

```go
import (
    "github.com/acidsailor/mcpkit/server"
    "github.com/acidsailor/mcpkit/toolkit"
    "github.com/acidsailor/mcpkit/validate"
)
```

## Quick start

The following program registers one read tool and one confirmation-gated write
tool, then serves them over HTTP:

```go
package main

import (
    "context"
    "log"
    "net/http"

    "github.com/acidsailor/mcpkit/elicit"
    "github.com/acidsailor/mcpkit/server"
    "github.com/acidsailor/mcpkit/toolkit"
    "github.com/acidsailor/mcpkit/validate"
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

type GreetInput struct {
    Name string `json:"name"`
}

func main() {
    mcpServer := mcp.NewServer(
        &mcp.Implementation{Name: "demo", Version: "0.1.0"},
        nil,
    )

    toolkit.AddRead(
        toolkit.New(
            mcpServer,
            "greet",
            "Greet a user by name",
            toolkit.InputSchema[GreetInput](),
            func(ctx context.Context, in GreetInput) (toolkit.Value[string], error) {
                return toolkit.WrapValue("hello, "+in.Name, nil)
            },
        ).WithValidateFunc(func(ctx context.Context, in GreetInput) error {
            return validate.RequireNonEmpty("name", in.Name)
        }),
    )

    // Elicitation is a user prompt, not an authorization check.
    toolkit.AddWrite(
        toolkit.New(
            mcpServer,
            "delete_thing",
            "Delete a thing",
            toolkit.InputSchema[GreetInput](),
            func(ctx context.Context, in GreetInput) (toolkit.Value[string], error) {
                return toolkit.WrapValue("deleted "+in.Name, nil)
            },
        ).WithElicitParamsFunc(
            elicit.SimpleConfirmation[GreetInput]("Delete this thing?"),
        ),
    )

    handler := mcp.NewStreamableHTTPHandler(
        func(*http.Request) *mcp.Server { return mcpServer },
        &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true},
    )
    srv := server.New(
        mcpServer,
        server.WithTransport(server.HTTP),
        server.WithHTTPServer(&http.Server{
            Addr:    ":8080",
            Handler: handler,
        }),
    )
    if err := srv.ListenAndServe(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

`server.HTTP` and `server.Both` require a caller-configured `*http.Server`.
Use a stateless streamable HTTP handler for protocol `2026-07-28` clients.
Clients on older protocols need stdio or a stateful handler to run gated writes.
`server.WithLogger` sets the logger for lifecycle events; without it, the
server logs nothing.

### Resources

A static resource has one URI and returns a `Content` value:

```go
import "github.com/acidsailor/mcpkit/resource"

resource.New(
    mcpServer,
    "config://app",
    "app-config",
    "The application configuration",
    func(ctx context.Context) (resource.Content, error) {
        return resource.NewJSON(loadConfig(ctx)), nil
    },
).WithTitle("App Config").Add()
```

A template resource receives the concrete URI and its RFC 6570 variables.
Return `resource.ErrNotFound` when the requested resource does not exist:

```go
resource.NewTemplate(
    mcpServer,
    "users://{id}",
    "user",
    "A user by id",
    func(ctx context.Context, uri string, vars resource.Vars) (resource.Content, error) {
        id, err := vars.Int("id")
        if err != nil {
            return nil, err
        }
        user, ok := lookupUser(id)
        if !ok {
            return nil, resource.ErrNotFound
        }
        return resource.NewJSON(user), nil
    },
).WithMIMEType("application/json").Add()
```

`Add` panics on a malformed static URI. `NewTemplate` panics on an invalid RFC
6570 template. These are configuration errors.

The URI identifies a resource; the client does not fetch it. The read handler
produces the content. Use one URI convention per server, and keep static and
template forms consistent, such as `users://42` and `users://{id}`.

Test resource reads and listings through `mcptest`:

```go
session := mcptest.NewSession(t, mcpServer)
cfg := mcptest.ReadResourceJSON[Config](t, session, "config://app")
user := mcptest.ReadResourceJSON[User](t, session, "users://42")
uris := mcptest.ListResourceURIs(t, session)
templates := mcptest.ListResourceTemplateURIs(t, session)
```

### Tracing

`telemetry.TraceReceiving` starts one server span per MCP request. Span names and
parenting follow the MCP semantic conventions, and spans carry a subset of
their attributes. Pass the `TracerProvider` you configured; the
package never reads OpenTelemetry globals and configures no exporter:

```go
mcpServer.AddReceivingMiddleware(telemetry.TraceReceiving(tracerProvider))
```

Spans are named `tools/call <tool>`, `prompts/get <prompt>`, or the method
name. A call error or a tool result with `IsError` marks the span failed; a
JSON-RPC error records its code.
Incoming notifications also get a server span. Outbound calls made with the handler context,
such as `otelhttp` client requests, nest under the request span.

A client continues its own trace by sending W3C `traceparent` and `tracestate`
in the request's `params._meta`. MCP carries trace context in messages, not
HTTP headers, because one HTTP request can carry several MCP messages and
stdio has no headers. When both exist, the `_meta` parent wins and the
ambient span, such as an `otelhttp` handler span, becomes a link. Use
`telemetry.WithPropagator` to read other formats; `baggage` in `_meta` needs
a composite propagator that includes `propagation.Baggage`.

The ambient span is the one in the handler context. Stateless streamable HTTP
gives each request its own context. A stateful session keeps the context of
the HTTP request that created it, so later requests see that request's span.

Outgoing MCP calls and notifications can continue the same trace:

```go
mcpServer.AddSendingMiddleware(telemetry.TraceSending(tracerProvider))
```

`TraceSending` starts a client span from the supplied context and injects
that span into a copy of `params._meta`. It preserves unrelated metadata
and concrete SDK request/params types. Configured propagation fields are
replaced so reused params cannot carry stale trace context. Without a
parent in the context, the send starts a new trace. Notification spans end
when sending completes, not when the peer finishes handling the message.
Both tracing constructors also work on an SDK client.

The middleware copies only the structs and metadata it modifies. It does
not mutate the params it receives, though the SDK may add its own protocol
metadata before invoking middleware.

API migration: `telemetry.Middleware` is now `telemetry.TraceReceiving`;
the previous `LogMiddleware` name is now `Log`.

### Request logging

`telemetry.Log` logs requests and notifications to the
`*slog.Logger` you pass. It complements tracing and does not repeat it: the
record has the span name as its message, such as `tools/call echo`, a level,
and an `outcome`. The request attributes, duration, and error stay on the
span, and the record's trace context links the two:

| Outcome | Level | Cause |
|---|---|---|
| `ok` | Info | Success. |
| `input_required` | Info | A tool, prompt, or resource requested input. |
| `declined`, `canceled` | Info | The user declined or canceled the prompt. |
| `tool_error` | Warn | The tool result has `IsError`. |
| `error` | Error | The request failed with a call error. |

Use `WithLevelFunc` to change severity while keeping the outcome and the
client response unchanged. For example, to skip notifications and log invalid-params errors at Info:

```go
logs := telemetry.Log(logger, telemetry.WithLevelFunc(
    func(method string, o telemetry.Outcome, err error) (slog.Level, bool) {
        if strings.HasPrefix(method, "notifications/") {
            return 0, false
        }
        if e, ok := errors.AsType[*jsonrpc.Error](err); ok &&
            o == telemetry.OutcomeError && e.Code == jsonrpc.CodeInvalidParams {
            return slog.LevelInfo, true
        }
        return o.DefaultLevel(), true
    },
))
```

Params and results are not logged by default because they can hold secrets
and personal data. `telemetry.WithPayload` takes a function that returns the
attributes to add, so you choose what to log and how to redact it.

Pass tracing first in one AddReceivingMiddleware call, as below. With
separate calls, register logging first and tracing last: the last call wraps
the existing handler. Both forms let tracing pass its span to logging. An
OpenTelemetry log bridge such as `otelslog` then sets the record's trace and
span IDs, as it does for records your handlers log with the request context:

```go
logger := slog.New(otelslog.NewHandler(
    "myserver",
    otelslog.WithLoggerProvider(loggerProvider),
))
mcpServer.AddReceivingMiddleware(
    telemetry.TraceReceiving(tracerProvider),
    telemetry.Log(logger),
)
```

The same logger middleware can be installed for outgoing messages:

```go
mcpServer.AddSendingMiddleware(
    telemetry.TraceSending(tracerProvider),
    telemetry.Log(logger.With("direction", "send")),
)
```

For an outgoing notification, `ok` means send completion, not confirmation
that the peer handled it. A tool error received from the peer has no Go
error in `GetError()`, so `WithLevelFunc` receives a nil cause for it.

Pass the same logger as `mcp.ServerOptions.Logger` to include the SDK's
session events. The SDK logs them without a request context, so they carry
no trace. A stdio server must not log to stdout.

### Result envelopes

MCP structured results must have a JSON object at the root. Wrap slice and
scalar results with the supplied envelopes:

```go
return toolkit.WrapItems(client.List(ctx))
return toolkit.WrapValue(client.Count(ctx))
```

`Items.MarshalJSON` encodes a nil slice as `[]`.

### Error matching

Each package defines its own sentinels. Match them with `errors.Is`:

```go
if errors.Is(err, toolkit.ErrUserDeclined) { /* user declined */ }
if errors.Is(err, server.ErrInvalidAddr)   { /* invalid listen address */ }
```

### Test tools

`mcptest` runs registered tools through the SDK in-memory transport:

```go
session := mcptest.NewSession(t, mcpServer)
session = mcptest.NewSessionWithElicitation(t, mcpServer, handler)
```

Use `NewSessionWithElicitation` for gated write tools.

## Development

- `task test` runs all tests.
- `task lint` formats code and applies lint fixes.
- `task ci` runs read-only format and lint checks.
- `task check` runs `task lint` and `task test`.

## License

Licensed under the Apache-2.0. See [LICENSE](LICENSE).
