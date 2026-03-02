package db

const schema = `
CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    username      TEXT UNIQUE NOT NULL,
    password_hash TEXT,
    oidc_sub      TEXT UNIQUE,
    role          TEXT NOT NULL DEFAULT 'viewer',
    created_at    DATETIME NOT NULL,
    last_login_at DATETIME
);
CREATE TABLE IF NOT EXISTS api_keys (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    key_hash     TEXT NOT NULL,
    scope        TEXT NOT NULL,
    created_at   DATETIME NOT NULL,
    last_used_at DATETIME,
    expires_at   DATETIME
);
CREATE TABLE IF NOT EXISTS providers (
    id         TEXT PRIMARY KEY,
    name       TEXT UNIQUE NOT NULL,
    type       TEXT NOT NULL,
    priority   INTEGER NOT NULL DEFAULT 10,
    config     TEXT NOT NULL,
    config_ver INTEGER NOT NULL DEFAULT 1,
    enabled    BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE TABLE IF NOT EXISTS integrations (
    id         TEXT PRIMARY KEY,
    name       TEXT UNIQUE NOT NULL,
    type       TEXT NOT NULL,
    config     TEXT NOT NULL,
    enabled    BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL
);
CREATE TABLE IF NOT EXISTS aliases (
    id              TEXT PRIMARY KEY,
    provider_id     TEXT NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    herald_vault    TEXT NOT NULL,
    herald_item     TEXT NOT NULL,
    native_vault_id TEXT NOT NULL,
    native_item_id  TEXT NOT NULL,
    created_at      DATETIME NOT NULL,
    UNIQUE(provider_id, herald_vault, herald_item)
);
CREATE TABLE IF NOT EXISTS stacks (
    name          TEXT PRIMARY KEY,
    last_synced   DATETIME,
    secrets_count INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS stack_secrets (
    stack_name  TEXT NOT NULL REFERENCES stacks(name) ON DELETE CASCADE,
    ref         TEXT NOT NULL,
    ttl_seconds INTEGER,
    PRIMARY KEY (stack_name, ref)
);
CREATE TABLE IF NOT EXISTS audit (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ts          DATETIME NOT NULL,
    action      TEXT NOT NULL,
    stack       TEXT,
    secret_ref  TEXT,
    provider    TEXT,
    user_id     TEXT,
    api_key_id  TEXT,
    cache_hit   BOOLEAN,
    duration_ms INTEGER,
    error       TEXT,
    request_id  TEXT
);
CREATE INDEX IF NOT EXISTS audit_ts    ON audit(ts);
CREATE INDEX IF NOT EXISTS audit_stack ON audit(stack, ts);
CREATE TABLE IF NOT EXISTS migrations (
    version    INTEGER PRIMARY KEY,
    applied_at DATETIME NOT NULL
);
`
