# Herald

Herald is a secret middleware service that bridges [1Password](https://1password.com) to [Komodo](https://komo.do)-managed Docker Compose stacks. It resolves `op://` references in `extra.env` files at deploy time — keeping secrets out of git while delivering them as standard environment variables.

```
extra.env (in git)                  .env.resolved (ephemeral, ~seconds)
────────────────────────────────    ──────────────────────────────────
APP_URL=https://myapp.example.com → APP_URL=https://myapp.example.com
DB_PASSWORD=op://HomeLab/myapp/pw → DB_PASSWORD=xK9mP2qR7vNsLd
```

## Documentation

| Doc | Description |
|-----|-------------|
| [Architecture](docs/architecture.md) | How Herald works, components, security model |
| [Setup](docs/setup.md) | 1Password, Komodo variables, deployment |
| [Stack Migration](docs/stack-migration.md) | Migrating a stack to use Herald |
| [API Reference](docs/api.md) | HTTP API endpoints |
| [Cache](docs/cache.md) | Cache internals, encryption, invalidation |
| [Troubleshooting](docs/troubleshooting.md) | Common issues and fixes |

## Quick start

1. [Set up 1Password access](docs/setup.md#1-configure-1password-access) — Connect server (recommended) or service account
2. [Create Komodo variables](docs/setup.md#2-create-komodo-variables)
3. [Deploy the Herald stack](docs/setup.md#4-deploy)
4. [Migrate your first stack](docs/stack-migration.md)

## MCP tools

The [`mcp-herald`](https://github.com/elabx-org/mcp-herald) companion MCP server exposes Herald's capabilities to Claude for AI-assisted secret management.
