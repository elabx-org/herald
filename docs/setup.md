# Setup

## 1. Configure 1Password access

Herald supports two methods of accessing 1Password. Connect is recommended where available.

### Option A: 1Password Connect (recommended)

Self-hosted Connect containers run alongside Herald with no rate limits.

1. Generate a `1password-credentials.json` file from your 1Password account
2. Place it at `/mnt/cache/appdata/herald/1password-connect/1password-credentials.json` on the host
3. Generate a Connect access token scoped to your secrets vault (e.g. `HomeLab`)
4. Store the token as the Komodo variable `OP_CONNECT_TOKEN`

The Herald compose stack (`titan/herald/compose.yaml`) includes the Connect containers (`op-connect-api`, `op-connect-sync`) alongside Herald.

### Option B: Service account

1. In 1Password, create a service account with read access to your secrets vault
2. Store the token as the Komodo variable `HERALD_SA_READ_TOKEN`

For secret provisioning (creating new 1Password items via MCP), create a second service account with write access and store as `HERALD_SA_PROVISION_TOKEN`.

---

## 2. Create Komodo variables

| Variable | Purpose | Secret? |
|----------|---------|---------|
| `HERALD_API_TOKEN` | Bearer token for Herald API auth (`openssl rand -hex 32`) | Yes |
| `HERALD_CACHE_KEY` | Cache encryption passphrase (`openssl rand -hex 16`) | Yes |
| `HERALD_SA_READ_TOKEN` | 1Password service account token (read-only) | Yes |
| `HERALD_SA_PROVISION_TOKEN` | 1Password service account token (read+write) | Yes |
| `OP_CONNECT_TOKEN` | 1Password Connect access token | Yes |

---

## 3. Create the Docker network

```bash
docker network create herald-internal
```

---

## 4. Deploy

```
mcp__komodo__deploy_stack(stack="herald")
```

Verify it's healthy:
```bash
curl http://10.0.0.9:8765/v1/health
```

Expected response:
```json
{
  "status": "ok",
  "provisioner": "connect",
  "providers": [
    {"name": "1password-connect", "type": "connect_server", "status": "ok", "latency_ms": 10},
    {"name": "1password", "type": "service_account", "status": "ok"}
  ],
  "uptime_seconds": 7
}
```

---

## Environment variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `HERALD_API_TOKEN` | — | Bearer token for API authentication |
| `HERALD_CACHE_KEY` | — | Passphrase for on-disk cache encryption. If unset, cache is disabled. |
| `HERALD_CACHE_DATA_PATH` | `/data/cache.db` | Path for the BoltDB cache file |
| `OP_SERVICE_ACCOUNT_TOKEN` | — | 1Password service account token (read-only) |
| `OP_PROVISION_TOKEN` | — | 1Password service account token (provisioning) |
| `OP_CONNECT_TOKEN` | — | 1Password Connect access token |
| `OP_CONNECT_SERVER_URL` | — | Connect API URL (e.g. `http://op-connect-api:8080`) |
| `KOMODO_URL` | — | Komodo API URL for redeployment on rotation |
| `KOMODO_API_KEY` | — | Komodo API key |
| `KOMODO_API_SECRET` | — | Komodo API secret |
| `HERALD_URL` | `http://herald:8765` | Herald URL used by `herald-agent` CLI |
