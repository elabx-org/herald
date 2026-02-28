# Herald Secret Delivery v2 — Design

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make Herald the industry-standard secret delivery layer for all Komodo-managed Docker stacks, eliminating plaintext secrets from git and delivering resolved secrets as env vars at deploy time.

**Architecture:** Herald adopts the Doppler/Infisical/`op run` pattern — `extra.env` in git holds `op://` refs, Herald resolves them at deploy time and outputs a complete resolved env file, which is deleted after containers start. The `/run/herald` bind-mount architecture is retired in favour of stdout output.

**Tech Stack:** Go (herald service + herald-agent), 1Password SDK, Komodo pre/post_deploy hooks

---

## Problem

Secrets currently live in plaintext in `extra.env` files committed to the `elmerfds/stacks` git repository. This means anyone with repo access sees database passwords, OIDC client secrets, and API keys. Herald exists to fix this.

The v1 integration test worked but used an insecure pattern (`/run/herald` with chmod 777, volume mount + shell source) that is not suitable for production stacks.

## Target Pattern

```
extra.env (committed to git)              .env.resolved (ephemeral, lives ~seconds)
─────────────────────────────────────     ──────────────────────────────────────────
APP_URL=https://skillsforge.elabx.app ──▶ APP_URL=https://skillsforge.elabx.app
DB_PASSWORD=op://HomeLab/sf/db_pass   ──▶ DB_PASSWORD=xK9mP2qR...
OIDC_SECRET=op://HomeLab/sf/oidc      ──▶ OIDC_SECRET=abc123...
```

Every stack gets:
- `pre_deploy`: resolves `extra.env` → `.env.resolved`
- `post_deploy`: `rm -f .env.resolved`
- Compose services: `env_file: - .env.resolved`

## Required Changes

### 1. Herald API: Complete pass-through output

`POST /v1/materialize/env` currently writes only the resolved secret key=value pairs to the output file. It must write the **complete env file** — non-secret lines pass through unchanged, `op://` refs are substituted with resolved values.

Input `env_content`:
```
APP_URL=https://example.com
DB_PASSWORD=op://HomeLab/myapp/db_password
DEBUG=false
```

Output file (and response body):
```
APP_URL=https://example.com
DB_PASSWORD=xK9mP2qR7vNsLd
DEBUG=false
```

### 2. Herald API: Return resolved content in response body

Add `content` field to `materializeEnvResponse` containing the complete resolved env content as a string. This allows herald-agent to receive and print it without needing to read a separate file.

### 3. Herald-agent: `--out -` stdout support

When `--out -` is passed, herald-agent:
- Sends the API request with `out_path: ""` (no file write) or `out_path: "-"` (sentinel)
- Receives resolved content from response `content` field
- Prints to stdout

Usage in Komodo pre_deploy:
```bash
cat extra.env | docker exec -i herald /herald-agent sync \
  --stack mystack --env-file - --out - > .env.resolved
```

### 4. Herald: Remove mandatory `env_content` requirement

Currently the API returns 400 if `env_content` is empty. Should allow empty content (stacks with no secrets return an empty resolved file).

### 5. README

Comprehensive documentation covering:
- What Herald is and why it exists
- Setup: deploying Herald, configuring 1Password service accounts
- Usage: migrating a stack from plaintext to op:// refs
- MCP tools: provisioning secrets via AI assistant
- Security model and trade-offs
- Architecture diagram

## Security Posture

| Risk | Status | Notes |
|------|--------|-------|
| Secrets in git | ✅ Eliminated | `extra.env` has only `op://` refs |
| Secrets on disk (persistent) | ✅ Eliminated | `.env.resolved` deleted post-deploy |
| Secrets on disk (transient) | ⚠️ Brief window | ~seconds during `docker compose up` |
| Secrets in `docker inspect` | ⚠️ Accepted | Env vars inherently visible. Industry standard. |
| Unauthorised access | ✅ Controlled | 1Password SA with read-only, vault-scoped access |
| Audit trail | ✅ Full | 1Password logs every SA access |

The transient disk and `docker inspect` risks are universally accepted in the industry (Doppler, Infisical, Railway, Render all operate this way).

## Stack Migration Pattern

### `extra.env` (after migration)
```bash
# Non-secret config — plaintext is fine
DATA_DIR=/mnt/cache/appdata
APP_URL=https://myapp.elabx.app
ENVIRONMENT=production

# Secrets — op:// refs only, never plaintext in git
SECRET_KEY=op://HomeLab/myapp/secret_key
DB_PASSWORD=op://HomeLab/myapp/db_password
API_KEY=op://HomeLab/myapp/api_key
```

### `compose.yaml` (after migration)
```yaml
services:
  myapp:
    env_file:
      - .env.resolved   # replaces extra.env
```

### Komodo stack config
```
pre_deploy:  cat extra.env | docker exec -i herald /herald-agent sync --stack mystack --env-file - --out - > .env.resolved
post_deploy: rm -f .env.resolved
```

## Out of Scope

- Secret rotation / hot-reload (secrets take effect on next deploy)
- File-based secrets (`/run/secrets/`) — requires app code changes, not needed for current stacks
- Docker Swarm native secrets — not using Swarm
