# Architecture

## Deploy Flow

![Herald Deploy Flow](herald-deploy-flow.svg)

## Secret Rotation

![Herald Secret Rotation](herald-secret-rotation.svg)

> Source: [herald-architecture.drawio](herald-architecture.drawio)

## How it works

At deploy time, Komodo's `pre_deploy` hook pipes `extra.env` through Herald, which resolves every `op://` reference and outputs a complete resolved env file. Docker Compose reads this and passes values to containers as standard environment variables. The resolved file is deleted by `post_deploy`.

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
┌──────────────────────────────────┐
│  Herald container                │
│  (ghcr.io/elabx-org/herald)      │
│                                  │
│  herald-agent (CLI)              │
│    → POST /v1/materialize/env    │
│    ← resolved env content        │
│                                  │
│  Herald API (Go HTTP server)     │
│    → 1Password provider(s)       │
│    ← resolved secret values      │
└──────────────────────────────────┘
```

Herald runs as a long-lived service. `herald-agent` is a sidecar CLI binary in the same container, used by Komodo's `docker exec` to talk to the Herald API.

## 1Password providers

Herald supports two provider types, used in priority order:

| Type | Key | Rate limits | Notes |
|------|-----|-------------|-------|
| **Connect server** | `connect_server` | None | Self-hosted containers, local REST API |
| **Service account** | `service_account` | 1,000 reads/hr | Cloud API, fallback |

When a Connect server is configured, it is tried first. The service account is used as fallback if Connect is unavailable. Both providers can be active simultaneously for resilience.

Check active providers via the health endpoint or the `herald_health` MCP tool.

## Security model

| Risk | Status | Notes |
|------|--------|-------|
| Secrets in git | ✅ Eliminated | Only `op://` refs in git |
| Secrets on disk (persistent) | ✅ Eliminated | `.env.resolved` deleted post-deploy |
| Secrets on disk (transient) | ⚠️ ~seconds | During `docker compose up` window |
| Secrets in `docker inspect env` | ⚠️ Accepted | Same as Doppler, Infisical, `op run` |
| Unauthorised secret access | ✅ Controlled | 1Password SA with read-only, vault-scoped token |
| Audit trail | ✅ Full | 1Password logs every service account access |
| Cache at rest | ✅ Encrypted | AES-256-GCM — see [Cache](cache.md) |

The `docker inspect` risk requires an attacker to already have Docker socket access, at which point full host access is already compromised — this is the accepted industry baseline.

## `op://` URI format

```
op://[vault]/[item]/[field]
op://HomeLab/myapp/db_password
      │       │      └── Field name
      │       └────────── Item title (or UUID)
      └────────────────── Vault name (or UUID)
```

### Inline references

`op://` URIs can be embedded inside larger values — useful for connection strings and DSNs:

```bash
# Standalone — whole value is a secret
DB_PASSWORD=op://HomeLab/myapp/db_password

# Inline — op:// embedded inside a connection string
DATABASE_URL=postgresql+asyncpg://myuser:op://HomeLab/myapp/db_password@postgres:5432/mydb
```

After resolution:
```bash
DB_PASSWORD=xK9mP2qR7vNsLd
DATABASE_URL=postgresql+asyncpg://myuser:xK9mP2qR7vNsLd@postgres:5432/mydb
```

**Deduplication:** if the same URI appears multiple times across variables, Herald fetches it from 1Password only once.

**Character constraint:** vault, item, and field names in inline refs must match `[A-Za-z0-9_-]`. Names with spaces or dots aren't supported in inline position — use standalone form or store the full constructed value as a 1Password field.
