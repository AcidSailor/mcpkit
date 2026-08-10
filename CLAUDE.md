# CLAUDE.md

Guidance for work in this repository.

## Project

`mcpkit` is a Go library of shared primitives for MCP servers built with the
official
[`modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk).
It wraps the SDK; it does not implement the MCP protocol.

- Module: `github.com/acidsailor/mcpkit`
- Go: 1.26
- The root package exports no API.
- Importable code is in subpackages.
- `cmd/mcpbstage` is a build-time CLI, not library API.

## Commands

Use [Task](https://taskfile.dev) through `taskfile.yml`:

- `task test`: run `go test ./...`.
- `task lint`: format and lint with automatic fixes.
- `task ci`: run read-only format and lint checks.
- `task check`: run `task lint` and `task test`.
- `task update`: update the go-scaffolds template with Copier.

Run one test with `go test ./toolkit/ -run TestName -v`.

Linting uses golangci-lint v2, `modernize`, `gofumpt`, and `golines`. Keep lines
at 80 columns or fewer. CI uses
`acidsailor/go-scaffolds/.github/workflows/go-ci.yml@v1`.

## Packages

- `server`: transport selection, serving, and graceful shutdown.
- `toolkit`: typed tool registration.
- `resource`: static and URI-template resources.
- `registry`: server-independent tool and resource descriptors.
- `elicit`: write-tool confirmation gates.
- `openapi`: schema assembly from dereferenced OpenAPI documents.
- `validate`: input validators.
- `mcptest`: in-memory MCP test sessions.
- `cmd/mcpbstage`: stages `.mcpb` bundle contents from GoReleaser output.

## Errors

Each package defines its own sentinels in `errors.go`. Do not add a root
sentinel. Public entry points must wrap the package sentinel and useful context
with `%w`, so callers can use `errors.Is`.

`toolkit` adds the tool name to ordinary handler errors at registration. It
must return `*jsonrpc.Error` unchanged because the SDK uses its concrete type to
preserve the JSON-RPC code and data.

## `server`

`server.New` accepts `WithTransport`, `WithShutdownTimeout`, and
`WithHTTPServer`. `ListenAndServe` validates the configuration, serves until
the context is canceled, and shuts down gracefully. `Both` runs stdio and HTTP
concurrently; either transport stopping cancels the other.

HTTP configuration belongs to the caller. `HTTP` and `Both` require a complete
`*http.Server`, including its handler. The package uses its address, timeouts,
hooks, and TLS configuration unchanged. A non-nil `TLSConfig` selects
`ListenAndServeTLS`; the configuration must provide certificates.

The caller constructs the handler with `mcp.NewStreamableHTTPHandler` and may
add middleware or other routes. Invalid HTTP configuration returns:

- `ErrNoHTTPServer` for no server.
- `ErrNilHandler` for no handler.
- `ErrInvalidAddr` for a malformed address.

Prefer `mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true}`.
`Stateless: true` permits protocol `2026-07-28`; the client still selects the
negotiated version. `Stateless: false` limits negotiation to `2025-11-25`.
`JSONResponse` does not affect negotiation.

Current clients run gated writes over stateless HTTP through multi-round-trip
input requests. Older clients need stdio or a stateful handler because the SDK
shim requires a live session. Stateful sessions are in-process; multiple
replicas require session affinity. SDK list-result caching is active only on
the stateless path. Tool-call results are not cached.

## `toolkit`

`New` returns a value builder and infers input and output types from the call
function. The input schema is required. Use these options before registration:

- `WithOutputSchema`
- `WithValidateFunc`
- `WithElicitParamsFunc`
- `WithAnnotations`
- `WithGateID`

`AddRead` registers a validated read tool. It panics if elicitation or a gate ID
is configured. `AddWrite` registers a validated, elicitation-gated write tool.
Without a custom prompt, it asks `Run <name>?`.

Default read annotations are read-only and idempotent. Default write annotations
are destructive and non-idempotent. `WithAnnotations` replaces the complete
default set; it does not merge fields. `ReadOnlyHint` must match the access
category. A read annotation must set `ReadOnlyHint: true` because the SDK uses a
plain `bool`. A read must not set `DestructiveHint: true`. Conflicts panic with
`ErrReadOnlyMismatch` or `ErrDestructiveRead`.

A gated handler runs twice: once to ask and once after the client retries. The
validator runs on both passes and must not have side effects. The call function
runs only after acceptance.

The gate is not an authorization boundary. A request that already contains a
response under the gate ID bypasses the prompt. `RequestState` is unsigned, so
the retry is not bound to the original arguments. Use authentication and
idempotency controls where required.

The gate does not require the `elicitation` client capability. The SDK rejects
unsupported clients in its middleware for protocol `2026-07-28` and in
`ServerSession.Elicit` for earlier protocols. This call error has no matchable
sentinel.

Clients must check `CallToolResult.NeedsInput`. Ignoring `inputRequests` returns
an empty successful result without running the write.

`AddReadFunc` and `AddWriteFunc` register custom handlers without adding
validation or gating. Custom handlers can wrap the exported `Call` and `Gate`
methods. Bind method values after the builder chain is complete because the
builder is a value.

MCP structured results require an object root. Use `Items`, `Value`,
`WrapItems`, or `WrapValue` for slice and scalar results. `Items.MarshalJSON`
encodes a nil slice as `[]`.

`toolkit` re-exports the `elicit` sentinels.

## `resource`

`New` builds a static resource. `NewTemplate` builds an RFC 6570 resource
template and supplies the concrete URI and extracted `Vars` to its handler.
Both are value builders and register with `Add`.

Configuration options are `WithMIMEType`, `WithTitle`, `WithDescription`, and
`WithAnnotations`. `WithSize` applies only to static resources.

Handlers return one of these `Content` forms:

- `Text`: default `text/plain`.
- `Blob`: default `application/octet-stream`.
- `JSON`: always `application/json`.
- `Raw`: unchanged SDK content blocks; the caller supplies all fields.

`NewText`, `NewBlob`, and `NewJSON` construct the common forms. `Raw` must not
be empty. The package supplies the URI and fallback MIME type except for `Raw`.

Static `Add` panics on an invalid URI. `NewTemplate` panics on an invalid
template. `ErrNotFound` and `ErrTemplateMismatch` become the SDK resource
not-found error on the wire. `ErrInvalidVars` wraps failed `Vars.Int`
conversion. `ErrNoContent` reports nil content or an empty `Raw`.

The package does not configure resource subscriptions. Configure
`SubscribeHandler` and `UnsubscribeHandler` when constructing the SDK server.
SDK list-change notifications work through resource add and remove operations.

## `registry`

`Read`, `Write`, `Resource`, and `ResourceTemplate` create server-independent
`Registration` values. `New` flattens groups and preserves order. `Bind`
installs entries on a server. `Enable.Write` controls write tools; resources
always bind.

Tool options mirror `toolkit`. Use `WithToolAnnotations` for tool hints because
`WithAnnotations` configures resource annotations. Resource options mirror the
`resource` builder.

## `openapi`

`Parse` accepts JSON for a dereferenced OpenAPI document. Keep
`components.schemas`; inline all `$ref` values. It also converts OpenAPI 3.0
`nullable: true` fields to null-permitting JSON Schema types.

`Schemas` provides `ParamsSchema`, `ParamSchema`, `BodySchema`, `Ref`,
`OutputObject`, `OutputItems`, `OutputValue`, and `Summary`. Returned schemas
are independent clones.

Only `Parse` returns an error. Accessors panic with `ErrUndefined` for unknown
paths, operations, parameters, or components because these are configuration
errors.

## `validate`

- `RequireNonEmpty` rejects blank strings with `ErrEmpty`.
- `RequireNonZero` rejects zero comparable values with `ErrZero`.

Both validators include the field name in the wrapped error.

## Tests

Tests use `testify/require`. Prefer end-to-end tests through `mcptest`:

- `NewSession` advertises no elicitation capability.
- `NewSessionWithElicitation` installs an elicitation handler.
- `ReadResourceText`, `ReadResourceBlob`, and `ReadResourceJSON` read resources.
- `ListResourceURIs` and `ListResourceTemplateURIs` test listings.

Use `NewSessionWithElicitation` for gated write tools.
