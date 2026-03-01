# Troubleshooting

## Herald returned HTTP 500 in pre_deploy

Check Herald logs:
```
mcp__komodo__get_stack_logs(stack="herald", tail=50)
```

Common causes:
- 1Password service account token expired or revoked
- Item or vault name in `op://` ref doesn't match 1Password (case-sensitive on some providers)
- Herald container not running

## `.env.resolved` not found

The `pre_deploy` script failed silently. Check the Komodo deploy operation logs:
```
mcp__komodo__list_updates(limit=5)
```

If `cat extra.env | docker exec -i herald ...` fails, Herald wasn't reachable. Verify Herald is running:
```
mcp__komodo__get_stack_logs(stack="herald", tail=20)
```

## Secrets still showing `op://` in container

The `pre_deploy` ran but `docker compose` is reading `extra.env` instead of `.env.resolved`. Check that the compose file references `.env.resolved` (not `extra.env`) in `env_file:`.

## `herald-agent: failed after N retries`

Herald may be starting up or temporarily overloaded. The agent retries 3 times with backoff. If persistent, check Herald container health:
```
mcp__komodo__get_stack_logs(stack="herald", tail=20)
```

## Rate limit exceeded

The 1Password service account has exhausted its quota (1,000 reads/hour, 60-minute rolling window). You must wait for the window to pass — not just 60 minutes from now.

**Common causes:**
- `HERALD_CACHE_KEY` not set — every deploy hits 1Password for every secret
- Duplicate item names in 1Password — Herald retries before failing, multiplying API calls
- Health endpoint called excessively — each `/v1/health` call triggers a provider check

**Resolution:**
1. Ensure `HERALD_CACHE_KEY` is set in Komodo variables (enables the cache)
2. Check for duplicate item titles in 1Password
3. Use `/ping` for container liveness checks; reserve `/v1/health` for provider diagnostics
4. If available, configure 1Password Connect — no rate limits

## Provider shows `degraded` in health check

```
mcp__herald__herald_health()
```

- If `type: connect_server` is degraded: check `op-connect-api` and `op-connect-sync` container logs
- If `type: service_account` is degraded and `rate_limited_since` is set: wait for the rate limit window to reset
- If all providers are degraded: Herald cannot resolve secrets — deploys will fail

## Rotation not triggering redeployment

`herald_rotate` invalidates the cache but only redeploys stacks that appear in Herald's **in-memory index**. The index is populated on each successful `POST /v1/materialize/env` call and **resets on Herald restart**.

If no stacks are redeployed after rotation:
1. Herald may have restarted recently — redeploy affected stacks manually to repopulate the index
2. Check that `KOMODO_URL`, `KOMODO_API_KEY`, `KOMODO_API_SECRET` are configured in the Herald container
