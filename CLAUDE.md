# mcpkit

Go 1.27 library built on modelcontextprotocol/go-sdk.
Module: github.com/acidsailor/mcpkit. Public APIs live in subpackages;
the root exports nothing. cmd/mcpbstage is a build-time CLI.

## Commands

- `task test`: all tests; `go test ./toolkit -run TestName` for one test.
- `task ci`: read-only formatting and lint checks.
- `task lint`: formatting and lint fixes.
- `task check`: lint fixes, then tests.
- `task update`: update the Copier template.

Use the checked-in golangci-lint config. Keep lines within 80 columns.

## Conventions

- Keep code comments to one concise line; put detailed contracts in doc.go.
- Inject caller-owned dependencies. Do not configure OTel globals/exporters.
- Keep package sentinels in errors.go; wrap errors with context and %w.
- Preserve *jsonrpc.Error unchanged in toolkit: the SDK checks its concrete
  type, and wrapping changes the wire response.
- Keep logging opt-in. server defaults to a discarding logger.
- End every PR description with "🤖 Assisted with AI 🤖".

## Packages

- server: transports and shutdown; callers supply the complete http.Server.
- toolkit: typed tool registration and result envelopes.
- elicit: write confirmation gates and bounded prompts.
- resource: static and URI-template resources.
- registry: descriptors bound to servers; Enable.Write gates write tools.
- openapi: schemas from dereferenced OpenAPI JSON.
- validate: input validators with matchable errors.
- telemetry: receiving/sending traces and slog middleware.
- mcptest: in-memory MCP sessions and resource helpers.

Read each package's doc.go before changing its behavior.

## Constraints

- Builders are values. Finish the chain before binding method values.
- Gated writes validate on both passes; validators must have no side effects.
  Gates confirm intent, not authorization. Clients must check NeedsInput.
- Prefer stateless HTTP with JSONResponse for current clients. Older clients
  need stdio or stateful HTTP for gated writes. Stateful replicas need affinity.
- MCP structured results require an object root; use toolkit envelopes.
- Keep components.schemas when dereferencing OpenAPI documents.
- TraceReceiving extracts from params._meta; TraceSending injects into a copy.
  Preserve concrete SDK request types and caller-owned metadata.
- Put tracing before Log in one middleware registration call. With separate
  calls, register tracing last so it wraps logging.
- Logs omit attributes already on spans. Payloads require WithPayload;
  WithLevelFunc controls severity and emission.
- OTel SDK and exporter imports belong only in tests.

## Tests and SDK follow-ups

Use testify/require and mcptest. Gated writes need
NewSessionWithElicitation. Sending tests may connect SDK in-memory transports
directly to control both endpoints.

Hand-build requests only for cases sessions cannot produce, such as ambient
spans, typed-nil sending params, or legacy resources/subscribe. Supply a
non-nil session. Current clients send server/discover, not initialize.

On an SDK upgrade, check for mcp.HasParams to replace local reflection and
Request.GetID to populate jsonrpc.request.id.
