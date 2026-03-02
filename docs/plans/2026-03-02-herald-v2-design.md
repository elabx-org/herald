# Herald v2 Design

**Date**: 2026-03-02
**Status**: Draft
**Replaces**: Herald v1 (`github.com/elabx-org/herald`)

---

## 1. Vision & Goals

Herald v2 is a clean rewrite of the Herald secret management microservice. The core purpose remains the same — resolve secret references in env files at deploy time — but the architecture is rebuilt to be **provider-agnostic**, **integration-agnostic**, and **operator-friendly** via an embedded web UI.

### Goals

- Support multiple secret vault providers (1Password Connect, 1Password SDK, Bitwarden/Vaultwarden, HashiCorp Vault, Azure Key Vault) through a unified interface
- Support multiple integration targets (Komodo, Kubernetes, webhooks) as first-class plugins
- Embedded web UI for configuration, monitoring, and operations
- Provider-agnostic URI scheme (`herald://vault/item/field`) with backward-compatible `op://` alias support
- Inline secret substitution within larger strings (e.g., connection URLs)
- Proactive cache warming + stale-while-revalidate fallback
- Local account auth + OIDC/SSO support
- Single Go binary, self-contained deployment
- Zero-downtime migration from v1

### Non-Goals

- Runtime plugin loading (dlopen/so files) — providers are Go packages implementing shared interfaces
- Multi-tenant isolation — Herald is a single-team deployment
- Secret value mutation at resolve time (Herald resolves, never transforms)

---

## 2. Architecture

### 2.1 Overall Approach: Layered Monolith

Herald v2 is a single Go binary with clearly separated internal packages. Providers and integrations are Go packages implementing shared interfaces — no runtime loading, no microservices.

```
┌─────────────────────────────────────────────────────────────────┐
│                         Herald v2 Binary                        │
│                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────────┐  │
│  │  cmd/herald  │  │ cmd/herald-  │  │   ui/ (embed.FS)      │  │
│  │  (server)    │  │ agent (CLI)  │  │   React + Vite        │  │
│  └──────┬───────┘  └──────┬───────┘  └───────────────────────┘  │
│         │                 │                                      │
│  ┌──────▼─────────────────▼──────────────────────────────────┐  │
│  │                    internal/api                            │  │
│  │  HTTP handlers · WebSocket/SSE · middleware · routing      │  │
│  └──────┬─────────────────────────────────────────────────────┘  │
│         │                                                        │
│  ┌──────▼────────────┐  ┌─────────────────────────────────────┐  │
│  │   internal/core   │  │         internal/auth               │  │
│  │  materialize      │  │  local accounts · OIDC · JWT        │  │
│  │  cache (BoltDB)   │  │  API keys · sessions                │  │
│  │  rotation         │  └─────────────────────────────────────┘  │
│  │  audit (SQLite)   │                                           │
│  │  poller           │  ┌─────────────────────────────────────┐  │
│  │  index (SQLite)   │  │          internal/db                │  │
│  └──────┬────────────┘  │  SQLite schema · migrations         │  │
│         │               └─────────────────────────────────────┘  │
│  ┌──────▼────────────────────────────────────────────────────┐  │
│  │               internal/providers                          │  │
│  │  registry · interface · onepassword/ · bitwarden/         │  │
│  │  vault/ · azure/ · mock/                                  │  │
│  └──────┬────────────────────────────────────────────────────┘  │
│         │                                                        │
│  ┌──────▼────────────────────────────────────────────────────┐  │
│  │               internal/integrations                       │  │
│  │  registry · interface · komodo/ · kubernetes/ · webhook/  │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 Package Layout

```
cmd/
  herald/             # Server entrypoint
    main.go
    embed.go          # //go:embed ui/dist
  herald-agent/       # CLI entrypoint
    main.go
    sync.go
    health.go
    provision.go
    alias.go          # New: manage aliases

internal/
  api/                # HTTP handlers, SSE, middleware, routing (chi)
  auth/               # Local accounts, OIDC, JWT, API keys, sessions
  core/               # materialize, cache, index, rotation, audit, poller
  db/                 # SQLite schema + migrations (go-migrate)
  providers/          # Provider registry + implementations
    interface.go
    registry.go
    onepassword/      # Connect + SDK (CGO-gated by build tag)
    bitwarden/        # Vaultwarden-compatible REST API (pure Go)
    vault/            # HashiCorp Vault KV v2 (pure Go)
    azure/            # Azure Key Vault (pure Go)
    mock/             # Local YAML file provider (pure Go, dev/test)
  integrations/       # Integration registry + implementations
    interface.go
    registry.go
    komodo/           # Komodo deploy + alert
    kubernetes/       # Kubernetes secret sync
    webhook/          # Generic HTTP webhook
  config/             # Startup config (env + YAML)
  resolver/           # herald:// + op:// URI parsing + env scanning

ui/                   # React frontend (built to ui/dist/, gitignored)
  src/
    components/
    pages/
    hooks/
    stores/

docs/                 # Documentation
  api.md
  setup.md
  architecture.md
  cache.md
  providers/          # Per-provider setup guides
  integrations/       # Per-integration setup guides
```

---

## 3. URI Scheme & Provider Abstraction

### 3.1 URI Scheme

Herald v2 introduces a provider-agnostic URI scheme:

```
herald://vault/item/field
```

Backward compatibility: `op://vault/item/field` is accepted as an alias and resolved identically.

**Inline substitution** is fully supported — a `herald://` reference can appear anywhere within a value string:

```bash
# Full value
DB_PASSWORD=herald://HomeLab/myapp/db_password

# Inline within a larger string
DATABASE_URL=postgresql://admin:herald://HomeLab/myapp/db_password@db:5432/myapp
SMTP_DSN=smtp://herald://HomeLab/myapp/smtp_user:herald://HomeLab/myapp/smtp_pass@mail:587
```

The resolver scans env content, extracts all `herald://` references (including inline), resolves each, and substitutes back into the original string before returning.

### 3.2 Alias Table (UUID Support)

Providers like Bitwarden/Vaultwarden identify items by UUID, not human-readable names. Herald maintains an alias table in SQLite:

```
herald://HomeLab/myapp/db_password
    → resolves alias "HomeLab/myapp" → bitwarden:vaults/abc123/items/xyz789
    → fetches field "db_password" by label
```

The alias table maps:
```
(provider_name, herald_vault, herald_item) → (native_vault_id, native_item_id)
```

The alias wizard in the UI guides operators through provider-native discovery (vault list → item search → confirm) to populate this table.

### 3.3 Provider Interface

```go
type Provider interface {
    Name() string
    Type() string   // "connect_server", "service_account", "bitwarden", "vault", "azure", "mock"
    Priority() int  // Lower = tried first

    // Core resolution
    Resolve(ctx context.Context, vault, item, field string) (string, error)

    // Health
    Healthy(ctx context.Context) (ok bool, latencyMs int64, err error)

    // Discovery (for alias wizard and UI)
    ListVaults(ctx context.Context) ([]Vault, error)
    ListItems(ctx context.Context, vault string) ([]Item, error)
    ListFields(ctx context.Context, vault, item string) ([]Field, error)

    // Lifecycle
    Close() error
}
```

Providers are registered at startup via a registry. The manager tries providers in priority order, falling back to the next on error.

### 3.4 CGO Build Tags

The 1Password SDK provider requires CGO (wraps a Rust library). Use build tags to make it optional:

```go
//go:build onepassword_sdk
```

Providers without CGO (bitwarden, vault, azure, mock) compile without it. This enables cross-compilation for non-1Password deployments.

Build matrix:
```
make build PROVIDERS=onepassword,bitwarden   # CGO required
make build PROVIDERS=bitwarden,vault,azure   # pure Go, cross-compilable
```

---

## 4. Cache, Polling & Secret Lifecycle

### 4.1 Cache Architecture

Herald maintains an encrypted BoltDB cache for hot secret reads, separate from the SQLite config/audit store.

**BoltDB Buckets:**

| Bucket | Key Format | Value | Notes |
|--------|-----------|-------|-------|
| `secrets` | `provider/vault/item/field` | encrypted CacheEntry | Persistent cache |
| `index` | stack name | serialized StackInfo | Stack→secret mapping |

**Cache Entry:**
```go
type CacheEntry struct {
    Value     string    // plaintext (decrypted on read)
    Provider  string    // which provider resolved this
    Policy    Policy    // "memory" or "persistent"
    ExpiresAt time.Time // TTL applied
    FetchedAt time.Time // for stale detection
}
```

**Encryption:** AES-256-GCM with HKDF-derived keys. `HERALD_CACHE_KEY` is the passphrase; HKDF derives separate keys for cache vs other encrypted fields. Salt must be a random per-installation value stored in SQLite `settings` table (never hardcoded).

### 4.2 Three-Level TTL

TTL is resolved in priority order:
1. Per-secret override (stored in SQLite `stack_secrets` table)
2. Per-provider default (stored in `providers` config)
3. Global default (`herald.yaml`: `cache.default_ttl`, default 1h)

### 4.3 Polling Strategy

Herald uses both strategies simultaneously:

**Proactive polling**: Background goroutine wakes every `poll_interval` (configurable, default 10 minutes), fetches secrets approaching expiry (within `poll_threshold`, default 5 minutes), and refreshes them before they expire. This keeps the cache warm.

**Stale-while-revalidate**: On a cache miss or expired entry, if the provider is unavailable (rate-limited, unhealthy), serve the stale entry and log a warning. Do not fail the materialize request.

**Rate-limit protection**: The 1Password service account has a 1000 req/hr limit. The poller self-caps at ~800/hr to leave headroom for materialize calls. When the provider signals rate-limit (429), the manager suspends that provider and marks it `rate_limited_since`.

**Singleflight**: Concurrent requests for the same `provider/vault/item/field` key are deduplicated — only one provider call is made, all waiters receive the result. This prevents thundering herd on cold starts.

### 4.4 Cache Invalidation

| Trigger | Scope | Action |
|---------|-------|--------|
| `POST /v2/rotate/{item}` | All vaults, that item | Delete matching BoltDB entries → redeploy stacks |
| `POST /v2/rotate/{vault}/{item}` | Specific vault+item | Delete matching BoltDB entries → redeploy stacks |
| `DELETE /v2/cache/{stack}` | That stack's entries | Delete BoltDB entries, remove from index |
| `DELETE /v2/cache` | All entries | Flush entire BoltDB `secrets` bucket |
| Rotation webhook (external) | Provider-defined | As above |

---

## 5. Integration System

### 5.1 Integration Interface

```go
type Integration interface {
    Name() string
    Type() string  // "komodo", "kubernetes", "webhook"

    // Deploy (re)starts a stack after secret rotation
    Deploy(ctx context.Context, stack string) error

    // Notify sends an alert/event (optional — may be no-op)
    Notify(ctx context.Context, event Event) error

    // Healthy checks connectivity to the integration target
    Healthy(ctx context.Context) (bool, error)
}
```

### 5.2 Rotation Fanout

On rotation, Herald fans out to all integrations registered for the affected stacks:

```
POST /v2/rotate/{item}
  → identify stacks referencing this item (from index)
  → invalidate cache entries
  → for each stack:
      → for each registered integration:
          → integration.Deploy(ctx, stack)   // in parallel goroutines
  → collect results, return summary
```

Deployment uses `context.WithoutCancel` so it survives client disconnect. Timeout: 5 minutes per stack.

**Deduplication**: Rotation events triggered within 10 seconds of a previous rotation for the same item are deduplicated — second trigger is a no-op with a log warning.

### 5.3 Integration Fanout Pattern

All integration deploys for a rotation run in parallel via goroutines with individual timeouts. Results (success/failure per integration per stack) are collected and returned in the rotation response.

---

## 6. Auth, API & CLI

### 6.1 Authentication

Herald v2 supports two auth methods simultaneously:

**Local accounts**: Username + Argon2id-hashed password stored in SQLite `users` table. Sessions via JWT (HS256, `HERALD_JWT_SECRET`, 24h expiry). Refresh tokens are single-use with rotation (new token issued on each refresh). Admin bootstraps first account via env vars or `herald-agent bootstrap`.

**OIDC/SSO**: Configurable OIDC provider (Authelia, Keycloak, etc.) for UI login. OIDC tokens are validated and mapped to local user roles. Group membership from OIDC claims maps to Herald roles.

**API keys**: Long-lived tokens for programmatic access (herald-agent, external webhooks). SHA-256 hashed in SQLite. **Scoped**:
- `read` — health, inventory, stats, audit queries
- `materialize` — resolve secrets (herald-agent sync)
- `admin` — provision, rotate, cache flush, provider management

**No auth mode**: If `HERALD_API_TOKEN` is unset (v1 compat), all endpoints are open. Logs a warning on startup.

### 6.2 API Surface (v2)

**Public (no auth):**
```
GET  /ping
GET  /v2/health    # provider readiness + latency (cached 60s)
GET  /v2/stats     # in-memory counters
GET  /metrics      # Prometheus format
```

**Protected:**
```
# Secret resolution
POST /v2/materialize/env

# Inventory
GET  /v2/inventory
GET  /v2/inventory/{stack}

# Audit
GET  /v2/audit?stack=&secret=&hours=&cursor=&limit=

# Rotation
POST /v2/rotate/{item}
POST /v2/rotate/{vault}/{item}

# Cache management
DELETE /v2/cache/{stack}
DELETE /v2/cache

# Provisioning
POST /v2/provision

# Provider management (admin)
GET    /v2/providers
POST   /v2/providers
PUT    /v2/providers/{id}
DELETE /v2/providers/{id}
GET    /v2/providers/{id}/health
GET    /v2/providers/{id}/vaults
GET    /v2/providers/{id}/vaults/{vault}/items

# Alias management (admin)
GET    /v2/aliases
POST   /v2/aliases
DELETE /v2/aliases/{id}

# Integration management (admin)
GET    /v2/integrations
POST   /v2/integrations
PUT    /v2/integrations/{id}
DELETE /v2/integrations/{id}

# User management (admin)
GET    /v2/users
POST   /v2/users
PUT    /v2/users/{id}
DELETE /v2/users/{id}
POST   /v2/users/{id}/api-keys
DELETE /v2/users/{id}/api-keys/{key_id}

# Auth
POST /v2/auth/login
POST /v2/auth/refresh
POST /v2/auth/logout
GET  /v2/auth/oidc/callback

# Live events
GET  /v2/events    # SSE stream (audit, rotation, health changes)
```

**V1 compatibility aliases** (all v1 routes forward to v2 handlers):
```
POST /v1/materialize/env  → /v2/materialize/env
GET  /v1/health           → /v2/health
GET  /v1/stats            → /v2/stats
GET  /v1/inventory        → /v2/inventory
POST /v1/rotate/{item}    → /v2/rotate/{item}
DELETE /v1/cache          → /v2/cache
```

### 6.3 Error Response Format

All errors use a canonical envelope:
```json
{
  "error": "validation_failed",
  "message": "vault is required",
  "request_id": "abc123"
}
```

`request_id` is injected by the `X-Request-ID` middleware and echoed in response headers and structured logs.

### 6.4 Live Events (SSE)

`GET /v2/events` streams Server-Sent Events for:
- `rotation` — cache invalidation + redeploy triggered
- `health_change` — provider health status changed
- `audit` — new audit entry (for live audit feed in UI)

SSE is preferred over WebSocket for these server→client-only streams: works through HTTP proxies, automatic browser reconnect, simpler implementation.

### 6.5 herald-agent v2 CLI

```bash
# Secret resolution (primary use case in pre_deploy hooks)
herald-agent sync --stack myapp --env-file extra.env --out /run/secrets/.env
herald-agent sync --stack myapp --env-file -              # stdin → stdout
herald-agent sync --stack myapp --dry-run --env-file -    # validate without writing

# Health check (exits 1 if degraded)
herald-agent health

# Provisioning
herald-agent provision --vault HomeLab --item myapp \
  --field db_password:concealed \
  --field api_key:value=known-value:concealed \
  --field username:value=myapp-user

# Alias management
herald-agent alias add --provider bitwarden \
  --herald HomeLab/myapp --native vault-uuid/item-uuid
herald-agent alias list
herald-agent alias remove HomeLab/myapp

# Bootstrap first admin account
herald-agent bootstrap --username admin
```

Reads `HERALD_URL` (default `http://herald:8765`) and `HERALD_API_TOKEN` from env.
Retries transient errors (5xx, network); 4xx errors are permanent (no retry).

---

## 7. Data Model

### 7.1 SQLite Schema (Config + Audit)

```sql
-- Global configuration
CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
-- Stores: install_id, cache_salt, jwt_secret (encrypted), schema_version

-- Users
CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    username      TEXT UNIQUE NOT NULL,
    password_hash TEXT,           -- argon2id; NULL for OIDC-only users
    oidc_sub      TEXT UNIQUE,    -- OIDC subject claim
    role          TEXT NOT NULL DEFAULT 'viewer',
    created_at    DATETIME NOT NULL,
    last_login_at DATETIME
);

-- API keys
CREATE TABLE api_keys (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id),
    name       TEXT NOT NULL,
    key_hash   TEXT NOT NULL,      -- SHA-256 of raw key
    scope      TEXT NOT NULL,      -- "read", "materialize", "admin"
    created_at DATETIME NOT NULL,
    last_used_at DATETIME,
    expires_at DATETIME            -- NULL = no expiry
);

-- Provider configurations
CREATE TABLE providers (
    id         TEXT PRIMARY KEY,
    name       TEXT UNIQUE NOT NULL,
    type       TEXT NOT NULL,      -- "connect_server", "bitwarden", etc.
    priority   INTEGER NOT NULL DEFAULT 10,
    config     TEXT NOT NULL,      -- encrypted JSON blob (provider-specific)
    config_ver INTEGER NOT NULL DEFAULT 1,
    enabled    BOOLEAN NOT NULL DEFAULT true,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- Integration configurations
CREATE TABLE integrations (
    id         TEXT PRIMARY KEY,
    name       TEXT UNIQUE NOT NULL,
    type       TEXT NOT NULL,      -- "komodo", "kubernetes", "webhook"
    config     TEXT NOT NULL,      -- encrypted JSON blob
    enabled    BOOLEAN NOT NULL DEFAULT true,
    created_at DATETIME NOT NULL
);

-- Human-readable → provider-native alias mapping
CREATE TABLE aliases (
    id             TEXT PRIMARY KEY,
    provider_id    TEXT NOT NULL REFERENCES providers(id),
    herald_vault   TEXT NOT NULL,    -- "HomeLab"
    herald_item    TEXT NOT NULL,    -- "myapp"
    native_vault_id TEXT NOT NULL,   -- provider-native vault UUID/path
    native_item_id  TEXT NOT NULL,   -- provider-native item UUID/path
    created_at     DATETIME NOT NULL,
    UNIQUE(provider_id, herald_vault, herald_item)
);

-- Stack index (which stacks reference which items)
CREATE TABLE stacks (
    name         TEXT PRIMARY KEY,
    last_synced  DATETIME,
    secrets_count INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE stack_secrets (
    stack_name   TEXT NOT NULL REFERENCES stacks(name),
    ref          TEXT NOT NULL,     -- "op://HomeLab/myapp/field" or "herald://..."
    ttl_seconds  INTEGER,           -- NULL = use provider/global default
    PRIMARY KEY (stack_name, ref)
);

-- Audit log
CREATE TABLE audit (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ts          DATETIME NOT NULL,
    action      TEXT NOT NULL,      -- "materialize", "rotate", "provision", "flush"
    stack       TEXT,
    secret_ref  TEXT,
    provider    TEXT,
    user_id     TEXT,               -- NULL for automated/agent calls
    api_key_id  TEXT,
    cache_hit   BOOLEAN,
    duration_ms INTEGER,
    error       TEXT,               -- NULL on success
    request_id  TEXT
);
CREATE INDEX audit_ts ON audit(ts);
CREATE INDEX audit_stack ON audit(stack, ts);

-- Schema migrations tracking
CREATE TABLE migrations (
    version    INTEGER PRIMARY KEY,
    applied_at DATETIME NOT NULL
);
```

**SQLite settings:**
- WAL mode enabled on startup: `PRAGMA journal_mode=WAL`
- Foreign keys enabled: `PRAGMA foreign_keys=ON`
- Busy timeout: `PRAGMA busy_timeout=5000`

### 7.2 BoltDB Buckets (Cache)

| Bucket | Key | Value |
|--------|-----|-------|
| `secrets` | `provider/vault/item/field` | Encrypted `CacheEntry` (AES-256-GCM) |
| `index` | stack name | Serialized `StackInfo` |

Note: `index` bucket is deprecated in v2 (stack data moves to SQLite). Kept for v1 BoltDB file compatibility during migration.

---

## 8. UI Design

### 8.1 Design Language: Aurora

Herald v2's UI uses a distinctive dark glassmorphic design language called **Aurora**:

- **Background**: `#060b14` — deep midnight blue, subtle animated mesh gradient with cyan + violet nebula glow
- **Palette**: Cyan `#22d3ee` / Violet `#818cf8` / Emerald `#34d399` / Amber `#fbbf24`
- **Cards**: `rgba(255,255,255,0.04)` glass surface, `1px solid rgba(255,255,255,0.08)` border, `blur(20px)` backdrop
- **Typography**: Inter variable font, tight tracking for headings
- **Motion**: Framer Motion spring physics for all transitions; Canvas API particle field on login screen

### 8.2 Login: Book Split

The login screen uses a **book split layout**:
- Left panel (60%): animated particle canvas, Herald wordmark, tagline
- Right panel (40%): frosted glass login card with local account form and "Sign in with OIDC" button
- The two panels share a vertical seam with a subtle gradient blend

### 8.3 Dashboard Structure

```
Sidebar (collapsible):
  ◆ Dashboard        — overview, stats, provider health gauges
  ⬡ Providers        — list, add, configure, test health
  ⬡ Integrations     — Komodo, Kubernetes, webhook configs
  ⬡ Aliases          — human-readable → UUID mapping table + wizard
  ◆ Inventory        — stacks, secrets count, last synced
  ◆ Audit Log        — filterable table with live SSE updates
  ◆ Settings         — cache config, polling intervals, auth, API keys
```

**Provider card**: health dot (green/amber/red), name, type badge, latency sparkline, priority badge, "Test" button.

**Alias wizard**: 3-step — (1) select provider, (2) browse/search items (skeleton loading), (3) confirm mapping. Gracefully degrades to manual UUID entry if provider is unhealthy.

**Audit log**: Date-range filter, stack filter, action filter. Cursor-based pagination. Live SSE feed appends new entries to top with a "New entries" banner rather than disrupting scroll position.

### 8.4 Frontend Stack

| Layer | Choice | Reason |
|-------|--------|--------|
| Build | Vite | Fast HMR, small bundles |
| Framework | React 18 | Team familiarity, ecosystem |
| UI primitives | Radix UI | Accessible, unstyled |
| Styling | Tailwind CSS | Design token consistency |
| Animation | Framer Motion + Canvas API | Spring physics, GPU-accelerated |
| State | React Context + useReducer | Sufficient for this scope |
| HTTP | Fetch API | No extra dependency needed |
| Live events | EventSource (SSE) | Server→client only, auto-reconnect |
| Tables | react-virtual | Virtual scrolling for large audit logs |

**Development setup**: Vite dev server proxies `/v1/` and `/v2/` to Go server at `:8765`. No Go rebuild needed for UI changes during development.

---

## 9. Observability

### 9.1 Structured Logging

`zerolog` with JSON output. Consistent fields: `request_id`, `stack`, `provider`, `duration_ms`. Log levels via `LOG_LEVEL` env var (default `info`). Debug level logs cache decisions and provider selection — never logs secret values.

### 9.2 Prometheus Metrics

`GET /metrics` in Prometheus format:

```
herald_materialize_duration_seconds{provider,stack}  # histogram
herald_cache_hits_total{provider}
herald_cache_misses_total{provider}
herald_cache_stale_hits_total{provider}
herald_provider_healthy{provider,type}               # gauge (0/1)
herald_rotation_total{item}
herald_poll_errors_total{provider}
herald_active_stacks                                 # gauge
```

### 9.3 Health Endpoints

- `GET /ping` — liveness only (process up, no external calls)
- `GET /v2/health` — readiness (provider connectivity check, cached 60s)

Kubernetes deployments should configure separate liveness and readiness probes.

---

## 10. Security Design

The following security requirements incorporate findings from the OWASP security review:

### 10.1 Secrets in Transit

- All API communication via TLS in production (configured at reverse proxy, not Herald directly)
- API keys are SHA-256 hashed in SQLite — raw keys are never stored
- Passwords use Argon2id (not bcrypt, not PBKDF2)
- Audit log entries never include resolved secret values

### 10.2 Secrets at Rest

- BoltDB cache: AES-256-GCM, HKDF-derived key per data class (cache, config fields)
- Per-installation random salt generated at first start, stored in SQLite `settings`
- SQLite provider `config` blobs: individual sensitive fields encrypted, non-sensitive fields (name, type, priority) stored plaintext for queryability

### 10.3 API Security

- `out_path` parameter in materialize is path-traversal sanitized: validated against an allowlist of directories (configurable `HERALD_OUT_PATH_PREFIX`)
- Request body limit: 1MB (http.MaxBytesReader)
- Request ID injected by middleware, echoed in logs and `X-Request-ID` response header
- Scoped API keys: `read` / `materialize` / `admin`
- Login rate limiting: 5 attempts per minute per IP
- Provision endpoint rate limiting: 10 provisions per minute per API key

### 10.4 Bootstrap Security

- First admin account created via `herald-agent bootstrap` or env vars `HERALD_BOOTSTRAP_USER`/`HERALD_BOOTSTRAP_PASS` (cleared from env after first read)
- Bootstrap endpoint (`POST /v2/auth/bootstrap`) disabled once any admin user exists
- Default configuration has no default passwords

### 10.5 SSRF Protection

- All provider URLs and integration URLs validated against an allowlist (configurable, defaults deny private/link-local ranges)
- Webhook integration URLs similarly validated

### 10.6 Refresh Token Security

- Refresh tokens are single-use with rotation (new token issued on each use)
- Stored as SHA-256 hash in SQLite
- Expiry: 30 days (sliding)

---

## 11. Developer Experience

### 11.1 Makefile

```makefile
make dev       # Go server + Vite dev server (Vite proxies API calls to :8765)
make ui        # Build React assets to ui/dist/
make build     # make ui && go build -tags onepassword_sdk ./...
make test      # go test ./... (unit tests, mock provider)
make test-e2e  # Playwright against local Herald + mock provider
make lint      # golangci-lint + eslint
```

### 11.2 Mock Provider

Provider `type: mock` reads secrets from a local YAML file — no vault credentials needed for local development or CI:

```yaml
providers:
  - name: local-mock
    type: mock
    priority: 0
    config:
      path: ./testdata/secrets.yaml
```

### 11.3 Config Hot Reload

`SIGHUP` reloads `herald.yaml` from disk without restart. Structural changes (new provider type) require restart; credential updates take effect immediately.

### 11.4 Build Tags for Providers

```bash
# Build without 1Password SDK (pure Go, cross-compilable)
go build -tags bitwarden,vault,azure ./...

# Build with 1Password SDK (CGO required)
CGO_ENABLED=1 go build -tags onepassword,bitwarden ./...
```

---

## 12. Migration from v1

Zero-downtime migration path:

1. Deploy Herald v2 (replaces v1 — same port, same binary location)
2. Herald v2 accepts v1 routes (`/v1/materialize/env` etc.) as permanent aliases
3. Existing BoltDB cache file is readable by v2 (same key format)
4. SQLite database is new — no migration from v1 data needed
5. Update Komodo variables to add any new v2 config (`HERALD_JWT_SECRET`, etc.)
6. Update herald-agent binary in stacks (v2 agent is backward-compatible with v1 API)
7. Existing `op://` references continue to work unchanged

---

## 13. Operational Considerations

### 13.1 Graceful Shutdown

Shutdown order on SIGTERM:
1. Stop accepting new HTTP connections
2. Wait for in-flight requests (30s timeout)
3. Signal background workers to stop (close done channel)
4. Wait for workers to drain (10s timeout)
5. Close BoltDB + SQLite
6. Exit 0

Coordinated via `errgroup` with `signal.NotifyContext`.

### 13.2 Background Worker Lifecycle

Workers managed by a single `errgroup` context:
- Poller (secret refresh before TTL expiry)
- Health watcher (periodic provider health checks)
- Audit pruner (daily: delete entries older than retention window)
- SSE broadcaster (fan events to connected clients)

All workers honour context cancellation. Worker panic is recovered and logged (not fatal).

### 13.3 Load Testing Targets

Before v2 general availability:
- `<50ms` p99 for cache hits (concurrent materialize)
- `<2s` p99 for provider fetches (cold cache)
- `>100` concurrent stacks without SQLite lock contention

Use `k6` against a Herald instance with mock provider.

---

## 14. Outstanding Decisions

These are deferred to implementation:

1. **SQLite library choice**: `modernc.org/sqlite` (pure Go, no CGO) vs `mattn/go-sqlite3` (CGO, better performance). Recommendation: `modernc.org/sqlite` for non-1P builds, `mattn` when CGO is already required.
2. **OIDC library**: `coreos/go-oidc` is the standard choice.
3. **Provider interface versioning**: Define stability contract before v2.0 release.
4. **Kubernetes integration scope**: Secret sync only, or also ConfigMap support?
5. **Vite build output location**: `ui/dist/` — confirm embed path in Go.
