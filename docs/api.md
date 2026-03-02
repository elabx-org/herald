# API Reference

Herald exposes a JSON HTTP API on port `8765`. All endpoints except `/ping` and `GET /v1/health` require:

```
Authorization: Bearer <HERALD_API_TOKEN>
```

---

## `GET /ping`

Liveness check. No authentication. Returns immediately with no external calls.

```json
{"ok": true}
```

---

## `GET /v1/health`

Provider health check. No authentication required. Result is **cached for 60 seconds**.

```json
{
  "status": "ok",
  "provisioner": "connect",
  "providers": [
    {
      "name": "1password-connect",
      "type": "connect_server",
      "status": "ok",
      "latency_ms": 10
    },
    {
      "name": "1password",
      "type": "service_account",
      "status": "ok"
    }
  ],
  "uptime_seconds": 3600
}
```

- `status`: `"ok"` or `"degraded"` (HTTP 503 when degraded)
- `provisioner`: `"connect"` or `"sdk"` — which backend handles `/v1/provision`
- `providers[].type`: `"connect_server"` or `"service_account"`
- `providers[].rate_limited_since`: RFC3339 timestamp if rate-limited

---

## `POST /v1/materialize/env`

Resolve `op://` references in env file content.

**Request:**
```json
{
  "stack": "myapp",
  "env_content": "APP_URL=https://example.com\nDB_PASSWORD=op://HomeLab/myapp/db_password\n",
  "out_path": "",
  "bypass_cache": false
}
```

- `stack`: Stack name (used for logging and the in-memory index)
- `env_content`: Raw env file content with `op://` refs
- `out_path`: If non-empty, also write resolved content to this path inside the Herald container
- `bypass_cache`: Force fresh fetch from 1Password (default: `false`)

**Response:**
```json
{
  "resolved": 1,
  "cache_hits": 0,
  "stale_hits": 0,
  "failed": 0,
  "duration_ms": 120,
  "content": "APP_URL=https://example.com\nDB_PASSWORD=xK9mP2qR7vNsLd\n"
}
```

- `content`: Complete resolved env file — all lines preserved, `op://` refs substituted
- `stale_hits`: Secrets served from an expired cache entry because the provider was rate-limited

---

## `POST /v1/provision`

Create or upsert a 1Password item. Requires `OP_PROVISION_TOKEN` configured in Herald.

Fields with empty values are auto-generated. If the item already exists, only **missing** fields are added — existing field values are never overwritten.

**Request:**
```json
{
  "vault": "HomeLab",
  "item": "myapp",
  "category": "login",
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

---

## `GET /v1/inventory`

Returns metadata about all stacks that have been synced. Persisted to disk via bbolt — survives Herald restarts.

```json
{
  "stacks": {
    "myapp": {
      "secrets": 3,
      "last_synced": "2026-02-28T22:00:00Z",
      "providers_used": ["1password-connect"],
      "policies": ["memory"]
    }
  }
}
```

---

## `GET /v1/audit`

Query the audit log for secret access history. Returns an empty list if auditing is not enabled.

**Query params:** `stack`, `secret`, `hours`

```json
{
  "entries": [
    {
      "ts": "2026-02-28T22:00:00Z",
      "action": "materialize",
      "stack": "myapp",
      "provider": "1password-connect",
      "cache_hit": false,
      "duration_ms": 45
    },
    {
      "ts": "2026-02-28T23:00:00Z",
      "action": "rotate",
      "stack": "myapp",
      "secret": "myapp",
      "triggered_by": "rotation-webhook",
      "duration_ms": 0
    }
  ],
  "count": 2
}
```

Actions:
- `materialize` — a stack synced its secrets via `/v1/materialize/env`
- `rotate` — cache was invalidated and Komodo redeployment was triggered

---

## `POST /v1/rotate/{itemID}`

Invalidate cache for a 1Password item and redeploy all stacks referencing it.

`itemID` is the item title from the `op://` URI (e.g. `myapp` from `op://HomeLab/myapp/field`).

Redeployment requires Komodo credentials. Affected stacks are discovered from the persistent index.

```json
{
  "item_id": "myapp",
  "cache_invalidated": 3,
  "stacks_redeployed": ["myapp-prod", "myapp-staging"]
}
```

---

## `DELETE /v1/cache/{stack}`

Purge all cache entries for a stack and remove it from the inventory index. Takes effect on the next deploy.

---

## `DELETE /v1/cache`

Flush the entire cache (all stacks, all entries). Does not affect the inventory index.
