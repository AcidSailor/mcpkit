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
- `mcptest` creates in-memory client and server sessions for tests.

## Requirements

- Go 1.26 or later

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

Licensed under the GNU Affero General Public License v3.0. See [LICENSE](LICENSE).
