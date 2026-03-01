# Cache

Herald maintains an internal cache backed by BoltDB at `/data/cache.db`.

## What is cached

| Data | TTL | Notes |
|------|-----|-------|
| 1Password secret values | 300s (default) | Only used when `bypass_cache: false` |
| Health check result | 60s | Avoids repeated provider checks |

**Important:** `herald-agent sync` (used in `pre_deploy`) always sets `bypass_cache: true` — deploys always fetch fresh values from 1Password. The cache is used when callers explicitly opt into it.

## Encryption

On-disk cache entries are encrypted with **AES-256-GCM**:

1. `HERALD_CACHE_KEY` is stretched to a 256-bit key via **PBKDF2** (100,000 iterations, SHA-256)
2. A fresh random **nonce** is generated per write — same secret stored twice produces different ciphertext
3. GCM provides authenticated encryption — any tampered ciphertext fails to decrypt

`/data/cache.db` contains only ciphertext. Without `HERALD_CACHE_KEY`, it cannot be read. The key never touches disk — it lives only in the container environment.

If `HERALD_CACHE_KEY` is unset, the cache is disabled entirely.

## Cache key format

```
vault/item/field
```

e.g. `HomeLab/myapp/db_password` — mirrors the `op://` URI structure.

## Cache invalidation

### By 1Password item (recommended)

Invalidates all cached entries for an item and redeploys affected stacks:

```
herald_rotate(item_id="myapp")
```

This uses the item **title** (same string as the middle segment of the `op://` URI), not the 1Password UUID.

Requires `KOMODO_URL`, `KOMODO_API_KEY`, and `KOMODO_API_SECRET` to be configured for automatic redeployment. Stacks must have been synced since Herald last restarted to appear in the in-memory index.

### By stack

Clears cache entries for all secrets a stack has loaded. Takes effect on the next deploy:

```
herald_rotate_cache(stack="myapp")
```

## Rate limit protection

1Password service accounts are limited to **1,000 reads/hour** on most plans (60-minute rolling window). Without caching, a stack with 10 secrets deploying 6 times/hour = 60 API calls. With caching, repeat deploys within the TTL make zero API calls.

1Password Connect has no rate limits — if Connect is configured, the rate limit concern applies only when Connect is unavailable and Herald falls back to the service account.

### TTL configuration

```
HERALD_CACHE_DEFAULT_TTL=300   # seconds (default: 300)
```
