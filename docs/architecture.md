# Architecture

## Deploy Flow

```mermaid
flowchart LR
    git(["Git Push"]) -->|webhook| komodo[Komodo]

    subgraph hooks[" "]
        direction TB
        komodo -->|pre_deploy| agent["Herald Agent\n(docker exec)"]
        komodo -->|deploy| compose[Docker Compose]
        komodo -->|post_deploy| cleanup[/"rm .env.resolved"/]
    end

    subgraph herald[Herald]
        agent -->|"POST /v1/materialize/env"| api[Herald API]
    end

    subgraph op[1Password]
        connect[(Connect\nServer)]
        sa[(Service\nAccount)]
    end

    api -->|primary| connect
    api -.->|fallback| sa
    api -->|write| env[".env.resolved"]
    compose -->|read| env
    compose -->|inject env| app[App Container]
    cleanup -->|delete| env

    style env fill:#fff2cc,stroke:#d6b656
    style connect fill:#d5e8d4,stroke:#82b366
    style sa fill:#f5f5f5,stroke:#666666
```

## Secret Rotation

```mermaid
flowchart LR
    op(["1Password UI"]) -->|user edits secret| updated[Secret Updated]
    updated -->|MCP tool call| rotate["herald_rotate\n(MCP)"]
    rotate -->|"POST /v1/rotate/{item}"| api[Herald API]
    api -->|clears stale entries| cache[Cache Invalidated]
    api -->|redeploy affected stacks| komodo[Komodo API]
    komodo -->|triggers| redeploy[Stack Redeploy]
    redeploy -->|pre_deploy hook| agent["Herald Agent\n(docker exec)"]
    agent -->|fetches fresh secrets| fresh[Fresh Secrets]

    style cache fill:#d5e8d4,stroke:#82b366
    style fresh fill:#d5e8d4,stroke:#82b366
    style rotate fill:#dae8fc,stroke:#6c8ebf
```

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

## Background subsystems

Herald runs several background goroutines alongside the HTTP server:

| Subsystem | Interval | Purpose |
|-----------|----------|---------|
| **Health watcher** | 5 min | Polls `/v1/health`; sends Komodo alert on `ok→degraded` and `degraded→ok` transitions |
| **Token expiry monitor** | 5 min | Decodes `exp` claim from provider JWT tokens; sends warning alert N days before expiry (configurable), critical alert on expiry |
| **Audit pruner** | 24 h (+ startup) | Rewrites audit log keeping only entries within `retention_days` |

All goroutines respect the server's context and shut down cleanly on SIGTERM.

---

## Security model

| Risk | Status | Notes |
|------|--------|-------|
| Secrets in git | ✅ Eliminated | Only `op://` refs in git |
| Secrets on disk (persistent) | ✅ Eliminated | `.env.resolved` deleted post-deploy |
| Secrets on disk (transient) | ⚠️ ~seconds | During `docker compose up` window |
| Secrets in `docker inspect env` | ⚠️ Accepted | Same as Doppler, Infisical, `op run` |
| Unauthorised secret access | ✅ Controlled | 1Password SA with read-only, vault-scoped token |
| Audit trail | ✅ Full | Herald logs every sync and rotation; 1Password logs every SA access |
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
