# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`mcpkit` is a Go **library** (no `main`/`cmd`) of shared primitives for building
MCP servers on the official
[`modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk).
It doesn't reimplement the MCP protocol; it wraps the SDK with ergonomic
serving, tool-registration, schema-assembly, and test helpers used across
`acidsailor` MCP servers.

- Module path: `github.com/acidsailor/mcpkit`
- Go 1.26
- The root package exports nothing (see `doc.go`); all functionality lives in
  subpackages.

## Common commands

Tooling is driven by [Task](https://taskfile.dev) (`taskfile.yml`):

- `task test` — run all tests (`go test ./...`)
- `task lint` — formatters + linters **with autofix** (mutates files:
  `golangci-lint fmt` then `golangci-lint run --fix`)
- `task ci` — read-only fmt + lint check (`golangci-lint fmt --diff` then
  `golangci-lint run`); no mutation
- `task check` — composite: `lint` (mutating) then `test`
- `task update` — pull latest go-scaffolds template tooling via `uvx copier update`

Run a single test directly: `go test ./toolkit/ -run TestName -v`

Linting is golangci-lint **v2** (`.golangci.yaml`): standard linters plus
`modernize`, with `gofumpt` (extra-rules) and `golines` formatters. **Max line
length is 80 columns** — keep lines under 80.

CI runs the reusable workflow `acidsailor/go-scaffolds/.github/workflows/go-ci.yml@v1`
on push/PR to `main`.

## Architecture

Subpackages layered on the SDK:

- **`server/`** — wraps an `*mcp.Server` and serves it over a transport.
- **`toolkit/`** — generic fluent builder for registering tools on an `*mcp.Server`.
- **`registry/`** — server-independent tool descriptors bound to a server in one pass.
- **`elicit/`** — the write-tool elicitation gate and confirmation prompt helpers.
- **`openapi/`** — assembles per-tool JSON Schemas from a dereferenced OpenAPI document.
- **`validate/`** — small generic input validators.
- **`mcptest/`** — in-memory client↔server session helpers for tests.

### Error convention (repo-wide)

There is **no root umbrella sentinel**, deliberately — matching the Go stdlib
(`io.EOF`, `sql.ErrNoRows` have no `package.Err` parent). Each package declares
its **own sentinels** in its `errors.go` (e.g. `server.ErrInvalidTransport`,
`openapi.ErrParse`, `elicit.ErrUserDeclined`, `validate.ErrEmpty`). A public
entry point wraps its sentinel with detail via `fmt.Errorf("%w: …", ErrX, …)`,
preserving it for `errors.Is`. When adding an error path: declare the sentinel
in that package's `errors.go`, wrap it with `%w` plus context at the boundary,
and don't introduce a cross-package umbrella. `toolkit`'s shared handler
pipeline (`runValidated`) wraps any validator/gate/call error with the tool name
via `%w`, so a `validate`/`elicit` sentinel raised inside a tool stays matchable
after registration.

### `server`

`server.New(mcpServer, opts...)` builds a `*Server` configured via functional
`Option`s (`WithAddr`, `WithTransport`, `WithReadHeaderTimeout`,
`WithShutdownTimeout`). `ListenAndServe(ctx)` validates config, dispatches on
the `Transport` (`Stdio` / `HTTP` / `Both`), blocks until `ctx` is cancelled,
then shuts down gracefully. HTTP uses the SDK's streamable handler in
stateless + JSON mode. `Both` runs stdio and HTTP concurrently; either exiting
cancels the other. `Transport` implements `UnmarshalText` (plus a
`ParseTransport`), so env/flag/json config loaders can parse it. The exported
`MCP` field is an escape hatch to the underlying server.

### `toolkit`

A type-safe fluent builder. `New[In, Out](server, name, description,
inputSchema, call)` infers `In`/`Out` from `call`, so generic type params are
rarely written at call sites. The input schema is required (the SDK panics on
nil). Chain optional config, then register:

- `.WithOutputSchema(schema)` — optional; when set the SDK validates structured results.
- `.WithValidateFunc(f)` — runs on decoded input before the call (and before elicitation for writes).
- `.WithElicitParamsFunc(f)` — builds the confirmation prompt for write tools.
- `AddRead(tool)` — registers a read-only tool (ReadOnly + Idempotent hints).
  **Panics** if an elicit-params func was set (meaningless for reads).
- `AddWrite(tool)` — registers a state-mutating tool (Destructive hint) **gated
  by MCP elicitation**: the client must support elicitation (else
  `ErrNoElicitation`); the call runs only on an `accept` action
  (`decline`→`ErrUserDeclined`, `cancel`→`ErrUserCanceled`).

`toolkit` re-exports the `elicit` sentinels (`ErrUserDeclined`,
`ErrUserCanceled`, `ErrNoElicitation`, `ErrUnexpectedElicitAction`,
`ErrElicitationFailed`) so callers need not import `elicit`.

`InputSchema[In]()` reflects a schema from a plain Go struct via
`jsonschema.For`, panicking on failure like `mcp.AddTool` does.

`Tool` is a value type — builder methods return a copy, not a pointer.

Handlers are marshalled as-is (no auto-wrapping), so a handler returning a bare
slice or scalar would violate MCP's object-root `structuredContent` contract.
`result.go` provides envelopes: `Items[T]`/`Value[T]` (shapes `{"items":…}` /
`{"value":…}`) and the `WrapItems`/`WrapValue` adapters that consume a
`(slice|scalar, error)` pair directly (e.g. `WrapItems(client.List(ctx))`).
`Items.MarshalJSON` normalizes a nil slice to `[]` so an array-typed output
schema still accepts it.

### `registry`

Collects tool registrations as server-independent descriptors and binds them to
a server in one pass, so the tool catalogue can be enumerated/filtered without a
live server. `registry.Read(...)` / `registry.Write(...)` mirror the toolkit
builder (`WithOutputSchema` / `WithValidateFunc` / `WithElicitFunc` options) and
return a `Registration`. `New(groups...)` flattens `[]Registration` slices into
an ordered `Registry`, preserving order. `(Registry).Bind(s, Enable{Write:
bool})` installs registrations; `AccessWrite` tools are skipped unless
`Enable.Write` is true.

### `openapi`

Input is the JSON of a **dereferenced** OpenAPI 3.1 document (every `$ref`
inlined, `components.schemas` retained). `Parse` decodes it once (returning
`ErrParse` on bad JSON; it normalizes OpenAPI 3.0 `nullable: true` to a
null-permitting type). The returned `*Schemas` then composes self-contained
schemas: `ParamsSchema`/`ParamSchema` (optionally a named subset), `BodySchema`
(application/json request body), `Ref` (named component), `OutputObject` /
`OutputItems` / `OutputValue` (response wrappers), and `Summary`. Returned
schemas are deep-cloned, so callers and the SDK may mutate them freely.
**Methods panic (wrapping `ErrUndefined`)** on an unknown
name/path/operation/parameter — static, programmer-level errors; only `Parse`
returns an error.

### `validate`

`RequireNonEmpty(field, value)` for strings (wraps `ErrEmpty`) and
`RequireNonZero[T comparable](field, value)` for non-string required inputs such
as numeric ids (wraps `ErrZero`). Both name the offending field.

## Testing

Tests use `testify` (`require`). The exported `mcptest` package drives a real
client↔server pair over the SDK's in-memory transport: `NewSession(tb, server)`
and `NewSessionWithElicitation(tb, server, handler)` (both take a `testing.TB`).
`NewSession` advertises no elicitation capability, so elicitation-gated write
tools fail under it — use `NewSessionWithElicitation` for those. Use these to
exercise registered tools end-to-end rather than calling internals directly.
