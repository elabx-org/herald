# Migrating a Stack to Herald

## Step 1: Provision secrets in 1Password

Use the `herald_provision_secret` MCP tool to create a 1Password item for the stack:

```
herald_provision_secret(
  vault="HomeLab",
  item="myapp",
  fields={
    "db_password": {},          # empty → auto-generated concealed field
    "smtp_key": {},
    "api_key": {"value": "known-value"}
  }
)
```

The tool returns `op://` references for each field:
```
op://HomeLab/myapp/db_password
op://HomeLab/myapp/smtp_key
op://HomeLab/myapp/api_key
```

## Step 2: Update `extra.env`

Replace plaintext secret values with `op://` references. Non-secret config passes through unchanged.

```bash
# Before
APP_URL=https://myapp.example.com
DB_PASSWORD=plaintext-password-123
DATABASE_URL=postgresql+asyncpg://myuser:plaintext-password-123@postgres:5432/mydb

# After
APP_URL=https://myapp.example.com

# Standalone: whole value is a secret
DB_PASSWORD=op://HomeLab/myapp/db_password

# Inline: secret embedded in connection string
DATABASE_URL=postgresql+asyncpg://myuser:op://HomeLab/myapp/db_password@postgres:5432/mydb
```

The same `op://` URI in multiple variables is fetched from 1Password only once (deduplicated).

## Step 3: Update `compose.yaml`

Change `env_file:` to reference `.env.resolved` instead of `extra.env`:

```yaml
services:
  myapp:
    env_file:
      - .env.resolved   # was: extra.env
```

If the service had plaintext secrets directly in its `environment:` block, move those to `extra.env` as `op://` refs and remove them from `environment:`.

## Step 4: Set Komodo hooks

In the Komodo stack config, set:

**pre_deploy:**
```bash
cat extra.env | docker exec -i herald /herald-agent sync --stack myapp --env-file - > .env.resolved
```

**post_deploy:**
```bash
rm -f .env.resolved
```

## Step 5: Deploy

```
mcp__komodo__deploy_stack(stack="myapp")
```

The deployment will:
1. `pre_deploy`: Herald resolves all `op://` refs → `.env.resolved` created
2. `docker compose up`: Services receive actual secret values via `env_file`
3. `post_deploy`: `.env.resolved` deleted

---

## Common pitfalls

### Variable name mapping loss

If the compose `environment:` block had:
```yaml
environment:
  - MYSQL_PASSWORD=${DB_PASSWORD}
```

Removing that line means the container loses `MYSQL_PASSWORD` — it only receives `DB_PASSWORD` from `.env.resolved`. Fix: add both names to `extra.env` pointing to the same 1Password field:

```bash
DB_PASSWORD=op://HomeLab/myapp/db_password
MYSQL_PASSWORD=op://HomeLab/myapp/db_password   # same field, fetched once
```

### Komodo hook line joining

Komodo strips blank lines and `#` comments from hooks, then joins remaining lines with ` && `. Use actual newlines (not `;`) between commands. If prepending to an existing hook, use `\n` as separator.

### Early-exit post_deploy

If `post_deploy` has paths that `exit 0` early, put `rm -f .env.resolved` at the top so cleanup always runs.

### Authelia `AUTHELIA_*` variable names

Authelia strips `AUTHELIA_*` env vars before template processing. Use `SECRET_*` prefixed names instead when delivering Authelia secrets via Herald — see the Authelia stack docs.
