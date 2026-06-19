# mcpkit

Shared Go primitives for building [Model Context Protocol](https://modelcontextprotocol.io)
servers on the official
[`modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk).
`mcpkit` doesn't reimplement the protocol — it wraps the SDK with ergonomic
helpers for serving over transports, registering type-safe read/write tools,
gating mutations behind MCP elicitation, assembling JSON Schemas from OpenAPI
documents, and driving in-memory tests. Library only (no command, no `main`).

## Features

- **`server`** — serve an `*mcp.Server` over stdio, streamable HTTP, or both
  concurrently, with graceful shutdown and a parseable `Transport` type.
- **`toolkit`** — a generic, fluent builder for registering read and write
  tools; write tools are auto-gated behind MCP elicitation.
- **`registry`** — describe tools independently of a server and bind them in one
  pass, with write tools gated by a single `Enable.Write` flag.
- **`elicit`** — the elicitation gate plus static and dynamic confirmation
  prompt builders.
- **`openapi`** — assemble per-tool input/output JSON Schemas from a
  dereferenced OpenAPI 3.1 document.
- **`validate`** — small generic input validators with matchable error
  sentinels.
- **`mcptest`** — connect an in-memory client↔server session for end-to-end tool
  tests.

## Requirements

- Go 1.26 or later

## Installation

```sh
go get github.com/acidsailor/mcpkit
```

Import the subpackages you need:

```go
import (
    "github.com/acidsailor/mcpkit/server"
    "github.com/acidsailor/mcpkit/toolkit"
    "github.com/acidsailor/mcpkit/validate"
)
```

## Quickstart

Register a read tool and a confirmation-gated write tool, then serve over the
configured transport.

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

    // A read-only tool. In/Out are inferred from the call func.
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

    // A write tool, gated by MCP elicitation. The client must support
    // elicitation and accept the prompt before the call runs.
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

    // The HTTP and Both transports serve a caller-built *http.Server as-is;
    // set its Handler to server.Handler(mcpServer) (optionally wrapped or muxed).
    srv := server.New(
        mcpServer,
        server.WithTransport(server.HTTP),
        server.WithHTTPServer(&http.Server{
            Addr:    ":8080",
            Handler: server.Handler(mcpServer),
        }),
    )
    if err := srv.ListenAndServe(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

### Result envelopes

Handlers are marshalled as-is, and MCP requires the structured result root to be
a JSON object. For a handler that produces a bare slice or scalar, wrap it:

```go
// {"items": [...]} — nil slices marshal to [] so an array schema still accepts them
return toolkit.WrapItems(client.List(ctx))

// {"value": ...}
return toolkit.WrapValue(client.Count(ctx))
```

### Error matching

Each package owns its sentinels (no root umbrella error). Match the specific
condition with `errors.Is`:

```go
if errors.Is(err, toolkit.ErrUserDeclined) { /* user declined the write */ }
if errors.Is(err, server.ErrInvalidAddr)   { /* bad listen address */ }
```

### Testing tools

Use `mcptest` to drive a registered server over the SDK's in-memory transport:

```go
session := mcptest.NewSession(t, mcpServer)
// For write tools gated by elicitation, supply a handler:
session = mcptest.NewSessionWithElicitation(t, mcpServer, handler)
```

## Development

Tooling is driven by [Task](https://taskfile.dev):

- `task test` — run all tests
- `task lint` — format and lint with autofix
- `task ci` — read-only format + lint verification
- `task check` — lint then test

## License

Licensed under the GNU Affero General Public License v3.0. See [LICENSE](LICENSE).
