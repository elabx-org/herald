# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# Herald

Go microservice resolving `op://` 1Password references at deploy time. Port 8765.

## Module

`github.com/elabx-org/herald` · Go 1.24 · `cmd/herald` (server) · `cmd/herald-agent` (CLI)

## Build & Test

```bash
go build ./...                          # verify compilation
go test ./...                           # all tests
go test ./internal/cache/...            # single package
go test ./internal/cache/... -run TestGet  # single test
CGO_ENABLED=1 go build ./cmd/herald/   # full server build (CGO required)
```

- **Always commit + push before triggering Komodo build** — build server pulls from GitHub, not local files
- Build: `mcp__komodo__run_build(build="herald")`
- Deploy: `mcp__komodo__deploy_stack(stack="herald")`

## Key Environment Variables

| Variable | Purpose |
|---|---|
| `HERALD_API_TOKEN` | Bearer token for protected API routes (optional — no auth if unset) |
| `HERALD_CONFIG` | Path to YAML config file |
| `HERALD_CACHE_KEY` | Enables encrypted bbolt cache (cache disabled if unset) |
| `HERALD_CACHE_DATA_PATH` | bbolt file path (default `/data/cache.db`) |
| `HERALD_AUDIT_ENABLED` | `true`/`1` to enable audit log |
| `HERALD_AUDIT_PATH` | Path for audit bbolt file |
| `OP_CONNECT_SERVER_URL` + `OP_CONNECT_TOKEN` | Connect server provider (lower priority number = higher priority) |
| `OP_SERVICE_ACCOUNT_TOKEN` | Service account provider (auto-creates provider if none configured) |
| `KOMODO_URL` + `KOMODO_API_KEY` + `KOMODO_API_SECRET` | Enables stack redeployment on rotation |

## API Routes

Public (no auth): `GET /ping`, `GET /v1/health`, `GET /v1/stats`

Protected (bearer token when `HERALD_API_TOKEN` set):
- `POST /v1/materialize/env` — resolve `op://` refs in env file content
- `POST /v1/provision` — create a new 1Password item
- `GET /v1/audit` — query audit log
- `GET /v1/inventory` / `GET /v1/inventory/{stack}` — list indexed stacks
- `POST /v1/rotate/{itemID}` / `POST /v1/rotate/{vault}/{itemID}` — invalidate cache + redeploy stacks
- `DELETE /v1/cache/{stack}` / `DELETE /v1/cache` — flush cache entries

## Request Data Flow (Materialize)

```
POST /v1/materialize/env
  → api/materialize.go: parse request, extract op:// refs via resolver.ScanEnvContent
  → materialize.EnvMaterializer.Materialize()
      → cache.Store.Get(vault/item/field)  ← cache hit: skip provider
      → provider.Manager.Resolve()         ← tries providers in priority order
          → connect_server (priority 0)
          → service_account (priority 1)
      → cache.Store.Set(vault/item/field)  ← cache write
  → resolver.ResolveEnvContent()           ← substitute values into env content
  → Index.Upsert(stack, StackInfo)         ← persist stack→item mapping for rotation
  → return resolved env content + stats
```

## Package Layout

```
internal/
  api/          # HTTP handlers, server (chi router), middleware, Index
  audit/        # Append-only audit log (bbolt); pruned daily
  cache/        # Encrypted bbolt cache (key = vault/item/field)
  config/       # Config struct + env/yaml loading
  komodo/       # Komodo API client (redeploy on rotation)
  materialize/  # Env file parser + op:// resolver orchestration
  provider/     # 1Password provider manager (connect + SDK); priority-ordered fallback
  provisioner/  # 1Password item creation (connect preferred over SDK)
  resolver/     # op:// URI parsing + env content scanning/substitution
  alert/        # Komodo alerting helpers
```

## Critical Gotchas

- **Cache key format**: `vault/item/field` (e.g. `HomeLab/authelia/jwt_secret`) — never stack-prefixed. Cache operations that assume a stack prefix silently miss all entries.
- **bbolt shared file**: `cache.Store` uses bucket `secrets`; `Index` uses bucket `index` — both share the same `.db` file via `cache.Store.DB()`.
- **Go closures**: Goroutines referencing outer variables require those vars to be declared *before* the goroutine. The audit prune goroutine previously referenced `ctx` before it was declared — always move goroutine spawns after context init.
- **Distroless image** (`gcr.io/distroless/base-debian12:nonroot`): no shell, no standard utilities. Cannot `docker exec` commands into the herald container.
- **CGO required**: `onepassword-sdk-go` wraps a Rust library. Build must use `CGO_ENABLED=1` and a glibc-based runtime (Debian). Alpine will fail at runtime. Tests for packages that don't use CGO can use `CGO_ENABLED=0`.
- **`context.WithoutCancel`** (Go 1.21+): Used in rotation handlers to detach Komodo deploys from the HTTP request lifecycle so they survive client disconnects.

## Index (StackInfo)

`Index.stacks` maps stack name → `StackInfo`. `ItemRefs` is `map[string][]string` where key = item name (no vault), values = raw `op://vault/item/field` URIs. Used to derive exact cache keys and vault-scoped lookups. Persisted to bbolt across restarts when cache is enabled.

## herald-agent CLI

Used in Komodo `pre_deploy` hooks to pull resolved secrets before a stack starts.

```bash
herald-agent sync --stack myapp --out /run/secrets/.env
herald-agent sync --stack myapp --env-file extra.env --out /run/secrets/.env
herald-agent sync --stack myapp --dry-run   # validate without writing
herald-agent health                          # check Herald reachability
herald-agent provision --vault V --item I --field F --value secret
```

Reads `HERALD_URL` (default `http://herald:8765`) and `HERALD_API_TOKEN` from env. Retries transient errors (5xx); 4xx errors are permanent (no retry).

## Provider Priority

Providers are sorted by `Priority` field (lower = tried first). Connect server defaults to priority 0; service account defaults to 1. On rate-limit errors, `materialize` serves stale cache if available rather than failing.

## Docs

`docs/` contains `api.md`, `setup.md`, `architecture.md`, `cache.md`, `troubleshooting.md`, `stack-migration.md`. Keep in sync when adding endpoints or config options.
