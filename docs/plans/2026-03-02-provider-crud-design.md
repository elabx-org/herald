# Provider CRUD Design

**Date:** 2026-03-02
**Status:** Approved

## Problem

Providers (1Password Connect, Service Account) are configured only via environment variables. There is no way to add, edit, or remove providers through the Herald UI. Users must know the env var names, update Komodo stack config, and redeploy — all outside Herald.

## Goal

Full provider management through the Herald UI: add, edit, delete providers live without a server restart. Env-var-defined providers remain visible and safe as a read-only baseline. A CLI/env-var reference is shown on each card for ops tooling.

## Approach: Two-tier (env baseline + bbolt CRUD)

- **Env-var providers** appear with `source: "env"` — read-only, cannot be deleted. These represent infrastructure-managed config.
- **DB providers** have `source: "db"` — fully editable via the UI, stored encrypted in bbolt.
- Editing an env provider creates a bbolt override with the same name. The bbolt version wins.
- Changes take effect immediately (hot-reload); no server restart required.

## Data Model

New `providers` bbolt bucket in the shared `/data/cache.db` file (alongside `secrets` and `index`).

```
ProviderRecord {
  Name     string   // unique identifier
  Type     string   // "1password-connect" | "1password-sdk" | "mock"
  Priority int      // 0 = highest priority, tried first
  URL      string   // plaintext: Connect server URL or mock file path
  Token    []byte   // AES-GCM encrypted using HERALD_CACHE_KEY
}
```

- URL is not sensitive; stored and returned in plaintext.
- Token is encrypted at rest using `HERALD_CACHE_KEY`. Storing a provider with a token is refused if no cache key is set.
- Tokens are **never returned** by any API endpoint — not even masked.
- On PUT, omitting the token field keeps the existing encrypted token unchanged.

## API

### Enhanced GET /v2/providers
Existing endpoint, extended response:
```json
{
  "name": "1password-connect",
  "type": "1password-connect",
  "priority": 0,
  "healthy": true,
  "latency_ms": 45,
  "checked_at": "2026-03-02T15:15:06Z",
  "url": "http://op-connect-api:8080",
  "source": "env"
}
```

### POST /v2/providers
Create a new provider. Persists to bbolt and activates immediately.
```json
{
  "name": "my-connect",
  "type": "1password-connect",
  "priority": 1,
  "url": "http://op-connect-api:8080",
  "token": "eyJ..."
}
```
Returns the new provider status (without token). Returns 400 if name already exists as a DB provider.

### PUT /v2/providers/{name}
Update an existing provider. Works on both `env` and `db` source providers (env providers get a bbolt override). Omit `token` to keep the existing encrypted value.

### DELETE /v2/providers/{name}
- `source: "db"`: removes from bbolt and deactivates immediately.
- `source: "env"` with no bbolt override: returns 403 (cannot delete env-managed providers).
- `source: "env"` with a bbolt override: deletes the override, reverts to env config.

## Hot-reload

`core.Manager` gains a `sync.RWMutex` around its provider slice. New methods:
- `AddProvider(p Provider) error`
- `UpdateProvider(p Provider) error`
- `RemoveProvider(name string) error`

API handlers call these after persisting to bbolt. The updated provider list is used for the next resolve attempt and health check cycle.

Provider types are constructed from their configs using a factory registry:
- `"1password-connect"` → `opprovider.NewConnect(name, url, token, priority)`
- `"1password-sdk"` → `opprovider.NewSDK(name, token, priority)` (calls registerSDKProvider logic)
- `"mock"` → `mockprovider.New(name, path, priority)`

## UI

**Provider cards** — each card extended with:
- Connection URL (Connect providers only)
- `ENV` / `DB` source badge
- Edit button (pencil icon, always visible)
- Delete button (trash icon, only `source: "db"`, with confirm dialog)
- Collapsible "Env vars" section showing the CLI/Komodo equivalent config

**Add Provider form** (slide-in panel from "Add Provider" button):
- Type selector: 1Password Connect | 1Password Service Account | Mock
- Name field (auto-suggested from type)
- Priority field (number input)
- Type-specific fields:
  - Connect: URL + Token (password with show/hide)
  - Service Account: Token only
  - Mock: File path only
- Success: new card appears immediately

**Edit form** — same panel, pre-filled with non-sensitive values. Token field shows placeholder: *"Leave blank to keep existing token"*

**Env var reference** (per-card collapsible):
```
# 1Password Connect
OP_CONNECT_SERVER_URL=http://op-connect-api:8080
OP_CONNECT_TOKEN=<your-token>

# 1Password Service Account
OP_SERVICE_ACCOUNT_TOKEN=<your-token>
```

## Security Notes

- Tokens encrypted with AES-GCM; same key derivation as cache store (HERALD_CACHE_KEY).
- POST/PUT with a token and no HERALD_CACHE_KEY set → 400 error (refuse unencrypted storage).
- Token never appears in API responses, logs, or audit entries.
- Audit log records provider add/edit/delete events (name, type, source — no token).

## Out of Scope

- Provider reordering via drag-and-drop (use priority field instead)
- Vault/item browsing within a provider (separate feature, ListVaults/ListItems already in Provider interface)
- Non-1Password provider types (design is extensible via factory registry, but only the three existing types ship now)
