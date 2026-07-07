# mcpkit

Shared Go primitives for building [Model Context Protocol](https://modelcontextprotocol.io)
servers on the official
[`modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk).
`mcpkit` doesn't reimplement the protocol — it wraps the SDK with ergonomic
helpers for serving over transports, registering type-safe read/write tools,
gating mutations behind MCP elicitation, assembling JSON Schemas from OpenAPI
documents, and driving in-memory tests. The importable API is library only; the
sole `main` is `cmd/mcpbstage`, a build-time CLI for staging `.mcpb` bundles.

## Features

- **`server`** — serve an `*mcp.Server` over stdio, streamable HTTP, or both
  concurrently, with graceful shutdown and a parseable `Transport` type.
- **`toolkit`** — a generic, fluent builder for registering read and write
  tools; write tools are auto-gated behind MCP elicitation.
- **`resource`** — a fluent builder for static and URI-template (dynamic)
  resources, with typed handlers, content envelopes, and extracted template
  variables.
- **`registry`** — describe tools and resources independently of a server and
  bind them in one pass, with write tools gated by a single `Enable.Write` flag.
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

    // The HTTP and Both transports serve a caller-built *http.Server as-is.
    // Build the Handler with mcp.NewStreamableHTTPHandler (optionally wrapped or
    // muxed). This server has a write tool, so the handler must be stateful — a
    // stateless one can't deliver the server->client elicitation request.
    handler := mcp.NewStreamableHTTPHandler(
        func(*http.Request) *mcp.Server { return mcpServer },
        &mcp.StreamableHTTPOptions{Stateless: false, JSONResponse: false},
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

### Resources

Register resources with the `resource` builder. A **static** resource has a
fixed URI; its typed read func returns a `Content` envelope (`NewText`,
`NewBlob`, `NewJSON`, or a `Raw` escape hatch) — the package stamps the URI and
a fallback MIME for you.

```go
import "github.com/acidsailor/mcpkit/resource"

// Static: fixed URI, JSON body (MIME is always application/json for JSON).
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

A **URI-template** (dynamic) resource matches many URIs. Its read func also
receives the concrete URI and a `Vars` of the RFC 6570 variables extracted from
it. Return `resource.ErrNotFound` to surface a not-found on the wire.

```go
// Dynamic: users://{id} — one template, many concrete URIs.
resource.NewTemplate(
    mcpServer,
    "users://{id}",
    "user",
    "A user by id",
    func(ctx context.Context, uri string, vars resource.Vars) (resource.Content, error) {
        id, err := vars.Int("id")
        if err != nil {
            return nil, err // wraps resource.ErrInvalidVars
        }
        u, ok := lookupUser(id)
        if !ok {
            return nil, resource.ErrNotFound
        }
        return resource.NewJSON(u), nil
    },
).WithMIMEType("application/json").Add()
```

`Add` panics on a malformed URI, and `NewTemplate` on an invalid RFC 6570
template — static, programmer-level errors, like `toolkit.InputSchema`.

The URI is an **identifier**, not a fetch target: the client passes it back in a
`resources/read` call and your read func turns it into bytes — the client never
dereferences it. The scheme is freeform (RFC 3986; MCP allows custom schemes),
so `config://`, `users://`, and the like are illustrative, not required. Reuse a
standard scheme when it *is* the thing (`file://`, `https://`); use a custom one
named after the entity for app-domain concepts. For a remote HTTP resource you
may put the real URL as the URI, but the `https://` scheme won't make the client
fetch it — your handler still does. Keep one convention per server, and make
templates match the static shape (`users://42` ↔ `users://{id}`), since the URI
is a stable identity clients may cache.

Drive resource reads and listing in tests with the `mcptest` helpers:

```go
session := mcptest.NewSession(t, mcpServer)

cfg := mcptest.ReadResourceJSON[Config](t, session, "config://app")
user := mcptest.ReadResourceJSON[User](t, session, "users://42")

uris := mcptest.ListResourceURIs(t, session)                 // static URIs
tmpls := mcptest.ListResourceTemplateURIs(t, session)        // URI templates
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
