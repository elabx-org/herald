# Herald

Herald is a secret middleware service that bridges [1Password](https://1password.com) to [Komodo](https://komo.do)-managed Docker Compose stacks. It replaces plaintext secrets in `extra.env` files with `op://` references that are resolved at deploy time — keeping secrets out of git while using the same env var delivery model as tools like Doppler and Infisical.

## How it works

```
extra.env (committed to git)              .env.resolved (ephemeral during deploy)
─────────────────────────────────────     ──────────────────────────────────────────
APP_URL=https://myapp.example.com     ──▶ APP_URL=https://myapp.example.com
DB_PASSWORD=op://HomeLab/myapp/db_pw  ──▶ DB_PASSWORD=xK9mP2qR7vNsLd
SMTP_KEY=op://HomeLab/myapp/smtp_key  ──▶ SMTP_KEY=sk-live-abc123...
```

At deploy time, Komodo's `pre_deploy` hook pipes `extra.env` through Herald, which resolves every `op://` reference via the 1Password SDK and outputs the complete resolved env file. Docker Compose reads this file and passes the values to containers as standard environment variables. The resolved file is deleted by `post_deploy`.

## Security model

| Risk | Status |
|------|--------|
| Secrets in git | ✅ Eliminated — only `op://` refs in git |
| Secrets on disk (persistent) | ✅ Eliminated — `.env.resolved` deleted post-deploy |
| Secrets on disk (transient) | ⚠️ Exists ~seconds during deploy window |
| Secrets in `docker inspect env` | ⚠️ Accepted — inherent to env var model |
| Unauthorised secret access | ✅ Controlled via 1Password service accounts |
| Audit trail | ✅ Full — 1Password logs every service account access |
| Cache at rest | ✅ AES-256-GCM encrypted (see [Cache](#cache)) |

This is the same model used by Doppler, Infisical, and 1Password's `op run` CLI. The `docker inspect` risk requires an attacker to already have Docker socket access, at which point full host access is already compromised.

## Cache

Herald maintains an internal cache backed by BoltDB. It is **not used during normal deploys** — the `herald-agent sync` command always fetches secrets fresh from 1Password so deployed containers always receive current values. The cache exists for:

- **`/v1/health` result** — provider health is cached for 60s to avoid repeated API calls when diagnostics are running
- **Future use** — the infrastructure supports cache-backed reads via `bypass_cache: false` in the materialize request, but this is not the default

The cache has two storage policies:

### Encryption

On-disk cache entries are encrypted with **AES-256-GCM**:

1. `HERALD_CACHE_KEY` is stretched into a 256-bit key using **PBKDF2** (100,000 iterations, SHA-256)
2. A fresh random **nonce** is generated for every write, so the same secret stored twice produces different ciphertext
3. GCM provides authenticated encryption — any tampering with the ciphertext causes decryption to fail

The cache file at `/data/cache.db` contains only ciphertext. Without `HERALD_CACHE_KEY`, it cannot be read. The key itself is never written to disk — it lives only in the container's environment (sourced from Komodo's `HERALD_CACHE_KEY` variable).

### Cache TTL

Entries expire after `HERALD_CACHE_DEFAULT_TTL` seconds (default: **300s / 5 minutes**). This only applies when `bypass_cache: false` is explicitly set on a materialize request — deploys via `herald-agent sync` always bypass the cache entirely.

### Cache invalidation after secret updates

If a secret value is changed in 1Password (via web UI or otherwise), Herald will serve the **stale cached value** until the TTL expires. To force immediate refresh:

**By 1Password item ID** (invalidates cache + redeploys affected stacks):
```
herald_rotate(item_id="<1password-item-uuid>")
```
The item UUID is visible in the 1Password web UI URL and in the `item_id` field returned by `herald_provision_secret`.

**By stack name** (clears cache for a stack, takes effect on next deploy):
```
herald_rotate_cache(stack="mystack")
```

Herald has no automatic mechanism to detect external 1Password changes. If your plan supports the 1Password Events API, you can configure a webhook from 1Password → `POST /v1/rotate/{itemID}` to trigger automatic invalidation on secret updates.

### Rate limit protection

1Password service accounts have a rate limit (1,000 reads/hour on most plans). Without a cache, every deploy hits the API for every secret — 10 secrets × 5 deploys/hour = 50 calls. With caching, deploys within the TTL window use the cached values and make **zero** 1Password API calls.

The cache key format is `vault/item/field`, matching the `op://vault/item/field` URI structure.

## Architecture

```
┌──────────────────────────────────────────────────────┐
│  Komodo Periphery (container, host docker socket)    │
│                                                      │
│  pre_deploy:                                         │
│  cat extra.env | docker exec -i herald               │
│    /herald-agent sync --stack mystack --env-file -   │
│    > .env.resolved                                   │
│                                                      │
│  docker compose up -d  (reads .env.resolved)         │
│                                                      │
│  post_deploy: rm -f .env.resolved                    │
└──────────────┬───────────────────────────────────────┘
               │ docker exec (via host daemon)
               ▼
┌──────────────────────────────┐
│  Herald container            │
│  (ghcr.io/elabx-org/herald)  │
│                              │
│  herald-agent (CLI)          │
│    → POST /v1/materialize/env│
│    ← resolved env content    │
│                              │
│  Herald API (Go HTTP server) │
│    → 1Password SDK           │
│    ← resolved secret values  │
└──────────────────────────────┘
```

Herald runs as a long-lived service. `herald-agent` is a sidecar binary in the same container used by Komodo's `docker exec` to communicate with the Herald API.

## Setup

### 1. Configure 1Password service accounts

In 1Password, create two service accounts:

**herald-read** (used at deploy time):
- Access: Read items in the vaults containing your secrets (e.g. `HomeLab`)
- Token → Komodo variable `HERALD_SA_READ_TOKEN`

**herald-provision** (used for AI-assisted secret creation):
- Access: Read + Write items in target vaults
- Token → Komodo variable `HERALD_SA_PROVISION_TOKEN`

### 2. Create Komodo variables

```
HERALD_API_TOKEN     → random token for API auth (openssl rand -hex 32)
HERALD_SA_READ_TOKEN → 1Password read-only service account token
HERALD_SA_PROVISION_TOKEN → 1Password write service account token
HERALD_CACHE_KEY     → 32-char encryption key (openssl rand -hex 16)
```

### 3. Create the herald Docker network (if not already created)

```bash
docker network create herald-internal
```

### 4. Deploy the Herald stack

```
mcp__komodo__deploy_stack(stack="herald")
```

Verify it's healthy:
```bash
curl -H "Authorization: Bearer $HERALD_API_TOKEN" http://10.0.0.9:8765/v1/health
```

## Migrating a stack to use Herald

### Step 1: Create secrets in 1Password

Use the Herald MCP tool to create a secret item for the stack:

```
herald_provision_secret(
  vault="HomeLab",
  item="myapp",
  fields={
    "db_password": {"concealed": true},
    "smtp_key": {"concealed": true},
    "api_key": {"concealed": true}
  }
)
```

Note the `op://` references returned (e.g. `op://HomeLab/myapp/db_password`).

### Step 2: Update `extra.env`

Replace plaintext secret values with `op://` references:

```bash
# Before
DB_PASSWORD=plaintext-password-123
DATABASE_URL=postgresql+asyncpg://myuser:plaintext-password-123@postgres:5432/mydb

# After — non-secret config stays as-is
APP_URL=https://myapp.example.com

# Standalone: whole value is an op:// ref
DB_PASSWORD=op://HomeLab/myapp/db_password
SMTP_KEY=op://HomeLab/myapp/smtp_key

# Inline: op:// ref embedded inside a larger string
DATABASE_URL=postgresql+asyncpg://myuser:op://HomeLab/myapp/db_password@postgres:5432/mydb
```

Non-secret values, comments, and blank lines are preserved unchanged in the resolved output. The same `op://` URI can appear in multiple variables (standalone or inline) and is fetched from 1Password only once.

### Step 3: Update the compose file

Change services from `env_file: - extra.env` to `env_file: - .env.resolved`:

```yaml
services:
  myapp:
    env_file:
      - .env.resolved   # was: extra.env
```

### Step 4: Configure Komodo stack hooks

In Komodo, update the stack's pre_deploy and post_deploy:

**pre_deploy:**
```bash
cat extra.env | docker exec -i herald /herald-agent sync --stack mystack --env-file - > .env.resolved
```

**post_deploy:**
```bash
rm -f .env.resolved
```

### Step 5: Deploy

```
mcp__komodo__deploy_stack(stack="mystack")
```

The deployment will:
1. `pre_deploy`: Pipe `extra.env` into herald-agent → Herald resolves all `op://` refs → prints complete resolved env to stdout → captured as `.env.resolved`
2. `docker compose up`: Services read from `.env.resolved`, receive actual secret values
3. `post_deploy`: `.env.resolved` is deleted

## How the `op://` URI format works

```
op://[vault]/[item]/[field]
op://HomeLab/myapp/db_password
      │       │      └── Field name in the item
      │       └────────── Item name (or UUID)
      └────────────────── Vault name (or UUID)
```

Herald resolves the field named `db_password` from the item named `myapp` in the `HomeLab` vault.

## Inline `op://` references

Herald supports embedding `op://` URIs inside larger values — useful for connection strings, DSNs, or any URL where a secret appears as a substring:

```bash
# Standalone (whole value is a secret) — both work:
DB_PASSWORD=op://HomeLab/myapp/db_password
DATABASE_URL=postgresql+asyncpg://myuser:op://HomeLab/myapp/db_password@postgres:5432/mydb
```

After resolution:
```bash
DB_PASSWORD=xK9mP2qR7vNsLd
DATABASE_URL=postgresql+asyncpg://myuser:xK9mP2qR7vNsLd@postgres:5432/mydb
```

The resolver regex safely terminates an inline URI at characters like `@`, `:`, whitespace, and quotes that naturally appear as delimiters in surrounding strings.

**Deduplication:** if the same `op://` URI appears multiple times (e.g. both `DB_PASSWORD` and `DATABASE_URL` embed `op://HomeLab/myapp/db_password`), Herald fetches it from 1Password only once.

**Character constraint:** vault, item, and field names in inline refs must match `[A-Za-z0-9_-]`. Names with spaces or dots are not supported in inline position (use standalone form instead, or encode them).

## API reference

Herald exposes a JSON HTTP API on port 8765. All endpoints except `/ping` and `/v1/health` require `Authorization: Bearer <token>`.

### `GET /ping`

Liveness check. Returns `{"ok":true}` immediately with no external calls — only confirms the HTTP server is running. Used by the Docker healthcheck.

```json
{"ok": true}
```

### `GET /v1/health`

Provider health check. Returns the status of the 1Password provider. No authentication required, but result is **cached for 60 seconds** to avoid unnecessary API calls.

```json
{
  "status": "ok",
  "providers": [{"name": "1password", "status": "ok", "latency_ms": 45}],
  "uptime_seconds": 3600
}
```

Status is `"degraded"` (HTTP 503) if any provider is unreachable or rate-limited.

### `POST /v1/materialize/env`

Resolve `op://` references in env file content and return the complete resolved env content.

**Request:**
```json
{
  "stack": "myapp",
  "env_content": "APP_URL=https://example.com\nDB_PASSWORD=op://HomeLab/myapp/db_password\n",
  "out_path": ""
}
```

- `stack`: Stack name (used for logging/audit)
- `env_content`: Raw env file content, may contain `op://` references
- `out_path`: If non-empty, also write resolved content to this file path inside the Herald container

**Response:**
```json
{
  "resolved": 1,
  "cache_hits": 0,
  "failed": 0,
  "duration_ms": 120,
  "content": "APP_URL=https://example.com\nDB_PASSWORD=xK9mP2qR7vNsLd\n"
}
```

- `content`: Complete resolved env file content (all lines, `op://` refs substituted, non-secrets preserved)

### `POST /v1/provision`

Create a new item in 1Password. Requires `OP_PROVISION_TOKEN` configured in the Herald container.

**Request:**
```json
{
  "vault": "HomeLab",
  "item": "myapp",
  "category": "Login",
  "fields": {
    "db_password": {"concealed": true},
    "api_key": {"value": "known-value", "concealed": true},
    "username": {"value": "myapp-user"}
  }
}
```

**Response:**
```json
{
  "vault_id": "abc123...",
  "item_id": "xyz456...",
  "refs": {
    "db_password": "op://HomeLab/myapp/db_password",
    "api_key": "op://HomeLab/myapp/api_key",
    "username": "op://HomeLab/myapp/username"
  }
}
```

## MCP tools (AI-assisted secret management)

Herald ships with an MCP server (`mcp-herald`) that exposes these tools to Claude:

### `herald_provision_secret`

Create a new secret item in 1Password. Herald auto-generates values for empty concealed fields.

### `herald_sync_stack`

Trigger secret synchronisation for a stack (resolve + write env file).

### `herald_health`

Check if the Herald service is reachable.

## Environment variables

| Variable | Purpose |
|----------|---------|
| `HERALD_API_TOKEN` | Bearer token for Herald API authentication |
| `OP_SERVICE_ACCOUNT_TOKEN` | 1Password read-only service account token |
| `OP_PROVISION_TOKEN` | 1Password write service account token (for provisioning) |
| `HERALD_CACHE_KEY` | Passphrase for on-disk cache encryption (PBKDF2 → AES-256-GCM). If unset, cache is disabled and every deploy hits 1Password. |
| `HERALD_CACHE_DATA_PATH` | Path for the BoltDB cache file (default: `/data/cache.db`) |
| `HERALD_URL` | URL of the Herald service (default: `http://herald:8765`, used by herald-agent) |

## Troubleshooting

### `herald returned HTTP 500` in pre_deploy

Check Herald logs:
```
mcp__komodo__get_stack_logs(stack="herald", tail=50)
```

Common causes:
- 1Password service account token expired or revoked
- Item or vault name in `op://` ref doesn't match 1Password
- Herald container not running

### `.env.resolved` not found

The `pre_deploy` script failed silently. Check the Komodo deploy operation logs:
```
mcp__komodo__list_updates(limit=5)
```

If `cat extra.env | docker exec -i herald ...` fails, Herald couldn't be reached. Verify Herald is running:
```
mcp__komodo__get_stack_logs(stack="herald", tail=20)
```

### Secrets still showing `op://` in container

The `pre_deploy` ran but `docker compose` is still using the old `extra.env` instead of `.env.resolved`. Verify the compose file references `.env.resolved` (not `extra.env`) in `env_file:`.

### `herald-agent: failed after N retries`

Herald may be starting up or overloaded. The agent retries 3 times with backoff by default. If this persists, check Herald container health:
```
mcp__komodo__get_stack_logs(stack="herald", tail=20)
```

### `rate limit exceeded` in Herald logs or `/v1/health`

The 1Password service account has exhausted its API quota (1,000 reads/hour on most plans). The limit uses a **60-minute rolling window** — you must wait for the window to pass, not just 60 minutes from now.

**Causes:**
- Cache not configured: `HERALD_CACHE_KEY` not set, so every deploy fetches all secrets fresh
- Duplicate item in 1Password: `op://vault/item/...` matched multiple items, causing repeated retries before failing
- Health endpoint called excessively: each call to `/v1/health` triggers a `Vaults().List()` check

**Resolution:**
1. Ensure `HERALD_CACHE_KEY` is set in Komodo variables — this enables the 60-minute TTL cache and eliminates repeat API calls for cached secrets
2. Check for duplicate item names in 1Password: vault/item names in `op://` refs must uniquely identify one item
3. Use `/ping` for liveness checks; only call `/v1/health` when you need to verify 1Password connectivity
