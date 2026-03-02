# Herald v2 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rewrite Herald as a provider-agnostic secret management microservice with an embedded web UI, supporting multiple vault providers and deployment integrations through a unified plugin interface.

**Architecture:** Single Go binary embedding a React frontend; providers and integrations are Go packages implementing shared interfaces (no runtime loading); SQLite for config/audit/index, BoltDB for encrypted secret cache; `herald://` URI scheme with `op://` backward-compatibility alias.

**Tech Stack:** Go 1.24, Chi router, SQLite (`modernc.org/sqlite`), BoltDB (bbolt), React 18, Vite, Tailwind CSS, Radix UI, Framer Motion, Argon2id (passwords), AES-256-GCM + HKDF (encryption), SSE (live events), Prometheus metrics.

**Design doc:** `docs/plans/2026-03-02-herald-v2-design.md`

---

## Phase 1: Foundation

### Task 1: Initialize module + directory skeleton

**Files:**
- Create: `go.mod` (module `github.com/elabx-org/herald`, Go 1.24)
- Create: `internal/db/` directory
- Create: `internal/core/` directory
- Create: `internal/providers/` directory
- Create: `internal/integrations/` directory
- Create: `internal/api/` directory
- Create: `internal/auth/` directory
- Create: `internal/config/` directory
- Create: `internal/resolver/` directory
- Create: `cmd/herald/` directory
- Create: `cmd/herald-agent/` directory
- Create: `ui/` directory

**Step 1: Create go.mod and fetch core dependencies**

```bash
cd /config/workspace/gh/herald
go mod init github.com/elabx-org/herald
go get github.com/go-chi/chi/v5@latest
go get modernc.org/sqlite@latest
go get go.etcd.io/bbolt@latest
go get github.com/rs/zerolog@latest
go get golang.org/x/crypto@latest         # argon2id + hkdf
go get golang.org/x/sync@latest           # singleflight + errgroup
go get github.com/spf13/cobra@latest
go get github.com/google/uuid@latest
go get gopkg.in/yaml.v3@latest
go get github.com/prometheus/client_golang@latest
```

**Step 2: Create placeholder main.go to verify compilation**

```go
// cmd/herald/main.go
package main

func main() {}
```

**Step 3: Verify compilation**

```bash
go build ./...
```
Expected: no output (success)

**Step 4: Commit**

```bash
git add go.mod go.sum cmd/herald/main.go
git commit -m "chore: initialize herald v2 module"
```

---

### Task 2: SQLite schema + migrations

**Files:**
- Create: `internal/db/db.go`
- Create: `internal/db/schema.go`
- Create: `internal/db/migrate.go`
- Create: `internal/db/db_test.go`

**Step 1: Write the failing test**

```go
// internal/db/db_test.go
package db_test

import (
    "testing"
    "github.com/elabx-org/herald/internal/db"
)

func TestOpen_CreatesSchema(t *testing.T) {
    store, err := db.Open(":memory:")
    if err != nil {
        t.Fatalf("Open: %v", err)
    }
    defer store.Close()

    // All tables must exist
    tables := []string{"settings", "users", "api_keys", "providers",
        "integrations", "aliases", "stacks", "stack_secrets", "audit", "migrations"}
    for _, tbl := range tables {
        row := store.DB().QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tbl)
        var name string
        if err := row.Scan(&name); err != nil {
            t.Errorf("table %q not found: %v", tbl, err)
        }
    }
}

func TestOpen_WALMode(t *testing.T) {
    store, err := db.Open(":memory:")
    if err != nil {
        t.Fatalf("Open: %v", err)
    }
    defer store.Close()

    var mode string
    store.DB().QueryRow("PRAGMA journal_mode").Scan(&mode)
    if mode != "wal" {
        t.Errorf("expected WAL mode, got %q", mode)
    }
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/db/... -v
```
Expected: FAIL — package not found

**Step 3: Implement db.go**

```go
// internal/db/db.go
package db

import (
    "database/sql"
    _ "modernc.org/sqlite"
)

type Store struct {
    db *sql.DB
}

func Open(path string) (*Store, error) {
    db, err := sql.Open("sqlite", path)
    if err != nil {
        return nil, err
    }
    pragmas := []string{
        "PRAGMA journal_mode=WAL",
        "PRAGMA foreign_keys=ON",
        "PRAGMA busy_timeout=5000",
    }
    for _, p := range pragmas {
        if _, err := db.Exec(p); err != nil {
            db.Close()
            return nil, err
        }
    }
    s := &Store{db: db}
    if err := s.migrate(); err != nil {
        db.Close()
        return nil, err
    }
    return s, nil
}

func (s *Store) DB() *sql.DB { return s.db }
func (s *Store) Close() error { return s.db.Close() }
```

**Step 4: Implement schema.go**

```go
// internal/db/schema.go
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
```

**Step 5: Implement migrate.go**

```go
// internal/db/migrate.go
package db

import "time"

func (s *Store) migrate() error {
    if _, err := s.db.Exec(schema); err != nil {
        return err
    }
    _, err := s.db.Exec(
        `INSERT OR IGNORE INTO migrations(version, applied_at) VALUES (1, ?)`,
        time.Now().UTC(),
    )
    return err
}
```

**Step 6: Run tests**

```bash
go test ./internal/db/... -v
```
Expected: PASS

**Step 7: Commit**

```bash
git add internal/db/
git commit -m "feat(db): SQLite schema with WAL mode and migrations"
```

---

### Task 3: Config package

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write failing test**

```go
// internal/config/config_test.go
package config_test

import (
    "os"
    "testing"
    "github.com/elabx-org/herald/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
    cfg, err := config.Load("")
    if err != nil {
        t.Fatalf("Load: %v", err)
    }
    if cfg.Port != 8765 {
        t.Errorf("expected port 8765, got %d", cfg.Port)
    }
    if cfg.Cache.DataPath != "/data/cache.db" {
        t.Errorf("unexpected cache path: %s", cfg.Cache.DataPath)
    }
}

func TestLoad_EnvOverrides(t *testing.T) {
    os.Setenv("HERALD_PORT", "9000")
    defer os.Unsetenv("HERALD_PORT")

    cfg, err := config.Load("")
    if err != nil {
        t.Fatalf("Load: %v", err)
    }
    if cfg.Port != 9000 {
        t.Errorf("expected port 9000, got %d", cfg.Port)
    }
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/config/... -v
```
Expected: FAIL

**Step 3: Implement config.go**

```go
// internal/config/config.go
package config

import (
    "os"
    "strconv"

    "gopkg.in/yaml.v3"
)

type Config struct {
    Port     int      `yaml:"port"`
    DBPath   string   `yaml:"db_path"`
    LogLevel string   `yaml:"log_level"`
    Cache    Cache    `yaml:"cache"`
    Audit    Audit    `yaml:"audit"`
    Auth     Auth     `yaml:"auth"`
    Komodo   Komodo   `yaml:"komodo"`
}

type Cache struct {
    Key            string `yaml:"key"`
    DataPath       string `yaml:"data_path"`
    DefaultTTLSecs int    `yaml:"default_ttl_seconds"`
    PollIntervalSecs int  `yaml:"poll_interval_seconds"`
    PollThresholdSecs int `yaml:"poll_threshold_seconds"`
    OutPathPrefix  string `yaml:"out_path_prefix"`
}

type Audit struct {
    Enabled       bool   `yaml:"enabled"`
    RetentionDays int    `yaml:"retention_days"`
}

type Auth struct {
    JWTSecret      string `yaml:"jwt_secret"`
    JWTExpiryHours int    `yaml:"jwt_expiry_hours"`
    OIDCIssuer     string `yaml:"oidc_issuer"`
    OIDCClientID   string `yaml:"oidc_client_id"`
    OIDCClientSecret string `yaml:"oidc_client_secret"`
}

type Komodo struct {
    URL       string `yaml:"url"`
    APIKey    string `yaml:"api_key"`
    APISecret string `yaml:"api_secret"`
}

func Load(path string) (*Config, error) {
    cfg := defaults()
    if path != "" {
        data, err := os.ReadFile(path)
        if err != nil {
            return nil, err
        }
        if err := yaml.Unmarshal(data, cfg); err != nil {
            return nil, err
        }
    }
    applyEnv(cfg)
    return cfg, nil
}

func defaults() *Config {
    return &Config{
        Port:     8765,
        DBPath:   "/data/herald.db",
        LogLevel: "info",
        Cache: Cache{
            DataPath:          "/data/cache.db",
            DefaultTTLSecs:    3600,
            PollIntervalSecs:  600,
            PollThresholdSecs: 300,
        },
        Audit: Audit{RetentionDays: 30},
        Auth:  Auth{JWTExpiryHours: 24},
    }
}

func applyEnv(cfg *Config) {
    if v := os.Getenv("HERALD_PORT"); v != "" {
        if n, err := strconv.Atoi(v); err == nil {
            cfg.Port = n
        }
    }
    if v := os.Getenv("HERALD_DB_PATH"); v != "" { cfg.DBPath = v }
    if v := os.Getenv("HERALD_LOG_LEVEL"); v != "" { cfg.LogLevel = v }
    if v := os.Getenv("HERALD_CACHE_KEY"); v != "" { cfg.Cache.Key = v }
    if v := os.Getenv("HERALD_CACHE_DATA_PATH"); v != "" { cfg.Cache.DataPath = v }
    if v := os.Getenv("HERALD_JWT_SECRET"); v != "" { cfg.Auth.JWTSecret = v }
    if v := os.Getenv("HERALD_AUDIT_ENABLED"); v == "true" || v == "1" { cfg.Audit.Enabled = true }
    if v := os.Getenv("KOMODO_URL"); v != "" { cfg.Komodo.URL = v }
    if v := os.Getenv("KOMODO_API_KEY"); v != "" { cfg.Komodo.APIKey = v }
    if v := os.Getenv("KOMODO_API_SECRET"); v != "" { cfg.Komodo.APISecret = v }
}
```

**Step 4: Run tests**

```bash
go test ./internal/config/... -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): env + YAML config with defaults"
```

---

## Phase 2: Provider System

### Task 4: Provider interface + registry

**Files:**
- Create: `internal/providers/interface.go`
- Create: `internal/providers/registry.go`
- Create: `internal/providers/registry_test.go`

**Step 1: Write failing test**

```go
// internal/providers/registry_test.go
package providers_test

import (
    "context"
    "testing"
    "github.com/elabx-org/herald/internal/providers"
)

type stubProvider struct{ name string; priority int }
func (s *stubProvider) Name() string  { return s.name }
func (s *stubProvider) Type() string  { return "stub" }
func (s *stubProvider) Priority() int { return s.priority }
func (s *stubProvider) Resolve(ctx context.Context, vault, item, field string) (string, error) { return "val", nil }
func (s *stubProvider) Healthy(ctx context.Context) (bool, int64, error) { return true, 1, nil }
func (s *stubProvider) ListVaults(ctx context.Context) ([]providers.Vault, error) { return nil, nil }
func (s *stubProvider) ListItems(ctx context.Context, vault string) ([]providers.Item, error) { return nil, nil }
func (s *stubProvider) ListFields(ctx context.Context, vault, item string) ([]providers.Field, error) { return nil, nil }
func (s *stubProvider) Close() error { return nil }

func TestRegistry_PriorityOrdering(t *testing.T) {
    r := providers.NewRegistry()
    r.Register(&stubProvider{"b", 5})
    r.Register(&stubProvider{"a", 1})
    r.Register(&stubProvider{"c", 10})

    ordered := r.Ordered()
    if ordered[0].Name() != "a" || ordered[1].Name() != "b" || ordered[2].Name() != "c" {
        t.Errorf("unexpected order: %v", ordered)
    }
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/providers/... -v
```

**Step 3: Implement interface.go**

```go
// internal/providers/interface.go
package providers

import "context"

type Vault struct{ ID, Name string }
type Item  struct{ ID, Name, Category string }
type Field struct{ ID, Label string; Concealed bool }

type Provider interface {
    Name() string
    Type() string
    Priority() int
    Resolve(ctx context.Context, vault, item, field string) (string, error)
    Healthy(ctx context.Context) (ok bool, latencyMs int64, err error)
    ListVaults(ctx context.Context) ([]Vault, error)
    ListItems(ctx context.Context, vault string) ([]Item, error)
    ListFields(ctx context.Context, vault, item string) ([]Field, error)
    Close() error
}
```

**Step 4: Implement registry.go**

```go
// internal/providers/registry.go
package providers

import "sort"

type Registry struct {
    providers []Provider
}

func NewRegistry() *Registry { return &Registry{} }

func (r *Registry) Register(p Provider) {
    r.providers = append(r.providers, p)
}

func (r *Registry) Ordered() []Provider {
    sorted := make([]Provider, len(r.providers))
    copy(sorted, r.providers)
    sort.Slice(sorted, func(i, j int) bool {
        return sorted[i].Priority() < sorted[j].Priority()
    })
    return sorted
}
```

**Step 5: Run tests**

```bash
go test ./internal/providers/... -v
```
Expected: PASS

**Step 6: Commit**

```bash
git add internal/providers/
git commit -m "feat(providers): provider interface and priority-ordered registry"
```

---

### Task 5: Mock provider (local YAML file)

**Files:**
- Create: `internal/providers/mock/mock.go`
- Create: `internal/providers/mock/mock_test.go`
- Create: `testdata/mock-secrets.yaml`

**Step 1: Write failing test**

```go
// internal/providers/mock/mock_test.go
package mock_test

import (
    "context"
    "os"
    "testing"
    "github.com/elabx-org/herald/internal/providers/mock"
)

func TestMock_Resolve(t *testing.T) {
    yaml := `
secrets:
  HomeLab:
    myapp:
      db_password: "s3cr3t"
      api_key: "abc123"
`
    f, _ := os.CreateTemp("", "secrets*.yaml")
    f.WriteString(yaml)
    f.Close()
    defer os.Remove(f.Name())

    p, err := mock.New("test-mock", f.Name(), 99)
    if err != nil {
        t.Fatalf("New: %v", err)
    }

    val, err := p.Resolve(context.Background(), "HomeLab", "myapp", "db_password")
    if err != nil {
        t.Fatalf("Resolve: %v", err)
    }
    if val != "s3cr3t" {
        t.Errorf("expected s3cr3t, got %q", val)
    }
}

func TestMock_ResolveNotFound(t *testing.T) {
    f, _ := os.CreateTemp("", "secrets*.yaml")
    f.WriteString("secrets: {}")
    f.Close()
    defer os.Remove(f.Name())

    p, _ := mock.New("test-mock", f.Name(), 99)
    _, err := p.Resolve(context.Background(), "Vault", "item", "field")
    if err == nil {
        t.Error("expected error for missing secret")
    }
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/providers/mock/... -v
```

**Step 3: Implement mock.go**

```go
// internal/providers/mock/mock.go
package mock

import (
    "context"
    "fmt"
    "os"

    "github.com/elabx-org/herald/internal/providers"
    "gopkg.in/yaml.v3"
)

type secretsFile struct {
    Secrets map[string]map[string]map[string]string `yaml:"secrets"`
}

type Provider struct {
    name     string
    path     string
    priority int
    data     secretsFile
}

func New(name, path string, priority int) (*Provider, error) {
    p := &Provider{name: name, path: path, priority: priority}
    return p, p.reload()
}

func (p *Provider) reload() error {
    raw, err := os.ReadFile(p.path)
    if err != nil {
        return err
    }
    return yaml.Unmarshal(raw, &p.data)
}

func (p *Provider) Name() string  { return p.name }
func (p *Provider) Type() string  { return "mock" }
func (p *Provider) Priority() int { return p.priority }
func (p *Provider) Close() error  { return nil }

func (p *Provider) Resolve(_ context.Context, vault, item, field string) (string, error) {
    v, ok := p.data.Secrets[vault]
    if !ok {
        return "", fmt.Errorf("mock: vault %q not found", vault)
    }
    i, ok := v[item]
    if !ok {
        return "", fmt.Errorf("mock: item %q not found in vault %q", item, vault)
    }
    f, ok := i[field]
    if !ok {
        return "", fmt.Errorf("mock: field %q not found in %q/%q", field, vault, item)
    }
    return f, nil
}

func (p *Provider) Healthy(_ context.Context) (bool, int64, error) { return true, 0, nil }
func (p *Provider) ListVaults(_ context.Context) ([]providers.Vault, error) {
    var vaults []providers.Vault
    for name := range p.data.Secrets {
        vaults = append(vaults, providers.Vault{ID: name, Name: name})
    }
    return vaults, nil
}
func (p *Provider) ListItems(_ context.Context, vault string) ([]providers.Item, error) {
    var items []providers.Item
    for name := range p.data.Secrets[vault] {
        items = append(items, providers.Item{ID: name, Name: name})
    }
    return items, nil
}
func (p *Provider) ListFields(_ context.Context, vault, item string) ([]providers.Field, error) {
    var fields []providers.Field
    for name := range p.data.Secrets[vault][item] {
        fields = append(fields, providers.Field{ID: name, Label: name})
    }
    return fields, nil
}
```

**Step 4: Create testdata/mock-secrets.yaml**

```yaml
secrets:
  HomeLab:
    myapp:
      db_password: "changeme"
      api_key: "test-api-key"
```

**Step 5: Run tests**

```bash
go test ./internal/providers/mock/... -v
```
Expected: PASS

**Step 6: Commit**

```bash
git add internal/providers/mock/ testdata/
git commit -m "feat(providers): mock YAML file provider for dev/test"
```

---

## Phase 3: Resolver & Cache

### Task 6: herald:// + op:// URI resolver

**Files:**
- Create: `internal/resolver/resolver.go`
- Create: `internal/resolver/resolver_test.go`

**Step 1: Write failing tests**

```go
// internal/resolver/resolver_test.go
package resolver_test

import (
    "testing"
    "github.com/elabx-org/herald/internal/resolver"
)

func TestParseRef_Herald(t *testing.T) {
    ref, err := resolver.ParseRef("herald://HomeLab/myapp/db_password")
    if err != nil { t.Fatal(err) }
    if ref.Vault != "HomeLab" || ref.Item != "myapp" || ref.Field != "db_password" {
        t.Errorf("unexpected: %+v", ref)
    }
}

func TestParseRef_Op(t *testing.T) {
    ref, err := resolver.ParseRef("op://HomeLab/myapp/db_password")
    if err != nil { t.Fatal(err) }
    if ref.Vault != "HomeLab" || ref.Item != "myapp" || ref.Field != "db_password" {
        t.Errorf("unexpected: %+v", ref)
    }
}

func TestParseRef_Invalid(t *testing.T) {
    _, err := resolver.ParseRef("not-a-ref")
    if err == nil {
        t.Error("expected error for invalid ref")
    }
}

func TestScanRefs_Inline(t *testing.T) {
    content := `DB_URL=postgres://admin:herald://Vault/item/field@host:5432/db
PLAIN=hello`
    refs := resolver.ScanRefs(content)
    if len(refs) != 1 {
        t.Fatalf("expected 1 ref, got %d: %v", len(refs), refs)
    }
    if refs[0].Raw != "herald://Vault/item/field" {
        t.Errorf("unexpected raw: %q", refs[0].Raw)
    }
}

func TestSubstituteRefs(t *testing.T) {
    content := `URL=https://example.com
SECRET=herald://V/I/F`
    resolved := map[string]string{"herald://V/I/F": "mysecret"}
    result := resolver.SubstituteRefs(content, resolved)
    expected := "URL=https://example.com\nSECRET=mysecret"
    if result != expected {
        t.Errorf("got %q, want %q", result, expected)
    }
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/resolver/... -v
```

**Step 3: Implement resolver.go**

```go
// internal/resolver/resolver.go
package resolver

import (
    "fmt"
    "regexp"
    "strings"
)

var refPattern = regexp.MustCompile(`(?:herald|op)://([^/\s]+)/([^/\s]+)/([^\s"']+)`)

type Ref struct {
    Raw   string
    Vault string
    Item  string
    Field string
}

func ParseRef(s string) (Ref, error) {
    m := refPattern.FindStringSubmatch(s)
    if m == nil {
        return Ref{}, fmt.Errorf("resolver: %q is not a valid herald:// or op:// reference", s)
    }
    return Ref{Raw: m[0], Vault: m[1], Item: m[2], Field: m[3]}, nil
}

func ScanRefs(content string) []Ref {
    matches := refPattern.FindAllStringSubmatch(content, -1)
    seen := map[string]bool{}
    var refs []Ref
    for _, m := range matches {
        if !seen[m[0]] {
            seen[m[0]] = true
            refs = append(refs, Ref{Raw: m[0], Vault: m[1], Item: m[2], Field: m[3]})
        }
    }
    return refs
}

func SubstituteRefs(content string, resolved map[string]string) string {
    result := content
    for raw, val := range resolved {
        result = strings.ReplaceAll(result, raw, val)
    }
    return result
}
```

**Step 4: Run tests**

```bash
go test ./internal/resolver/... -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/resolver/
git commit -m "feat(resolver): herald:// and op:// URI parsing with inline substitution"
```

---

### Task 7: BoltDB cache with encryption

**Files:**
- Create: `internal/core/cache/store.go`
- Create: `internal/core/cache/store_test.go`

**Step 1: Write failing tests**

```go
// internal/core/cache/store_test.go
package cache_test

import (
    "context"
    "testing"
    "time"
    "github.com/elabx-org/herald/internal/core/cache"
)

func TestCache_SetAndGet(t *testing.T) {
    s, err := cache.Open(t.TempDir()+"/cache.db", "test-passphrase")
    if err != nil { t.Fatal(err) }
    defer s.Close()

    entry := cache.Entry{
        Value:     "supersecret",
        Provider:  "mock",
        ExpiresAt: time.Now().Add(time.Hour),
    }
    if err := s.Set(context.Background(), "HomeLab/myapp/db_password", entry); err != nil {
        t.Fatal(err)
    }
    got, found, err := s.Get(context.Background(), "HomeLab/myapp/db_password")
    if err != nil { t.Fatal(err) }
    if !found { t.Fatal("expected to find entry") }
    if got.Value != "supersecret" {
        t.Errorf("got %q, want supersecret", got.Value)
    }
}

func TestCache_StaleGet(t *testing.T) {
    s, err := cache.Open(t.TempDir()+"/cache.db", "passphrase")
    if err != nil { t.Fatal(err) }
    defer s.Close()

    expired := cache.Entry{
        Value:     "old-secret",
        Provider:  "mock",
        ExpiresAt: time.Now().Add(-time.Second), // already expired
    }
    s.Set(context.Background(), "V/I/F", expired)

    _, found, _ := s.Get(context.Background(), "V/I/F")
    if found { t.Error("Get should not return expired entry") }

    stale, found, _ := s.GetStale(context.Background(), "V/I/F")
    if !found { t.Error("GetStale should return expired entry") }
    if stale.Value != "old-secret" {
        t.Errorf("unexpected stale value: %q", stale.Value)
    }
}

func TestCache_Flush(t *testing.T) {
    s, err := cache.Open(t.TempDir()+"/cache.db", "passphrase")
    if err != nil { t.Fatal(err) }
    defer s.Close()

    s.Set(context.Background(), "V/I/F", cache.Entry{Value: "x", ExpiresAt: time.Now().Add(time.Hour)})
    if err := s.Flush(); err != nil { t.Fatal(err) }
    _, found, _ := s.Get(context.Background(), "V/I/F")
    if found { t.Error("entry should be gone after flush") }
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/core/cache/... -v
```

**Step 3: Implement store.go** (AES-256-GCM encryption, HKDF key derivation)

```go
// internal/core/cache/store.go
package cache

import (
    "context"
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "crypto/sha256"
    "encoding/json"
    "fmt"
    "io"
    "time"

    "go.etcd.io/bbolt"
    "golang.org/x/crypto/hkdf"
)

var bucket = []byte("secrets")

type Entry struct {
    Value     string    `json:"v"`
    Provider  string    `json:"p"`
    ExpiresAt time.Time `json:"e"`
    FetchedAt time.Time `json:"f"`
}

type Store struct {
    db  *bbolt.DB
    aes cipher.AEAD
}

func Open(path, passphrase string) (*Store, error) {
    key := deriveKey(passphrase)
    block, err := aes.NewCipher(key)
    if err != nil { return nil, err }
    gcm, err := cipher.NewGCM(block)
    if err != nil { return nil, err }

    db, err := bbolt.Open(path, 0600, &bbolt.Options{Timeout: 5 * time.Second})
    if err != nil { return nil, err }
    err = db.Update(func(tx *bbolt.Tx) error {
        _, err := tx.CreateBucketIfNotExists(bucket)
        return err
    })
    if err != nil { db.Close(); return nil, err }
    return &Store{db: db, aes: gcm}, nil
}

func deriveKey(passphrase string) []byte {
    // HKDF with SHA-256; salt is fixed per application class
    // TODO(v2): use per-installation random salt from SQLite settings
    h := hkdf.New(sha256.New, []byte(passphrase), []byte("herald-cache-v2"), nil)
    key := make([]byte, 32)
    io.ReadFull(h, key)
    return key
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Set(_ context.Context, key string, e Entry) error {
    e.FetchedAt = time.Now().UTC()
    raw, err := json.Marshal(e)
    if err != nil { return err }
    nonce := make([]byte, s.aes.NonceSize())
    if _, err := rand.Read(nonce); err != nil { return err }
    sealed := s.aes.Seal(nonce, nonce, raw, nil)
    return s.db.Update(func(tx *bbolt.Tx) error {
        return tx.Bucket(bucket).Put([]byte(key), sealed)
    })
}

func (s *Store) get(key string) (Entry, bool, error) {
    var e Entry
    var found bool
    err := s.db.View(func(tx *bbolt.Tx) error {
        v := tx.Bucket(bucket).Get([]byte(key))
        if v == nil { return nil }
        found = true
        ns := s.aes.NonceSize()
        if len(v) < ns { return fmt.Errorf("cache: corrupt entry for %q", key) }
        plain, err := s.aes.Open(nil, v[:ns], v[ns:], nil)
        if err != nil { return err }
        return json.Unmarshal(plain, &e)
    })
    return e, found, err
}

func (s *Store) Get(_ context.Context, key string) (Entry, bool, error) {
    e, found, err := s.get(key)
    if err != nil || !found { return e, found, err }
    if time.Now().After(e.ExpiresAt) { return Entry{}, false, nil }
    return e, true, nil
}

func (s *Store) GetStale(_ context.Context, key string) (Entry, bool, error) {
    return s.get(key)
}

func (s *Store) Delete(_ context.Context, key string) error {
    return s.db.Update(func(tx *bbolt.Tx) error {
        return tx.Bucket(bucket).Delete([]byte(key))
    })
}

func (s *Store) Flush() error {
    return s.db.Update(func(tx *bbolt.Tx) error {
        return tx.DeleteBucket(bucket)
    })
}
```

**Step 4: Run tests**

```bash
go test ./internal/core/cache/... -v
```
Expected: PASS

**Step 5: Fix Flush — bucket must be re-created after delete**

Update `Flush()`:
```go
func (s *Store) Flush() error {
    return s.db.Update(func(tx *bbolt.Tx) error {
        if err := tx.DeleteBucket(bucket); err != nil && err != bbolt.ErrBucketNotFound {
            return err
        }
        _, err := tx.CreateBucket(bucket)
        return err
    })
}
```

**Step 6: Re-run tests**

```bash
go test ./internal/core/cache/... -v
```
Expected: PASS

**Step 7: Commit**

```bash
git add internal/core/cache/
git commit -m "feat(cache): AES-256-GCM encrypted BoltDB cache with stale-get support"
```

---

### Task 8: Provider manager with singleflight + stale fallback

**Files:**
- Create: `internal/core/manager.go`
- Create: `internal/core/manager_test.go`

**Step 1: Write failing tests**

```go
// internal/core/manager_test.go
package core_test

import (
    "context"
    "errors"
    "sync"
    "sync/atomic"
    "testing"
    "time"

    "github.com/elabx-org/herald/internal/core"
    "github.com/elabx-org/herald/internal/core/cache"
    "github.com/elabx-org/herald/internal/providers"
)

type countingProvider struct {
    calls int64
    err   error
}
func (p *countingProvider) Name() string  { return "counting" }
func (p *countingProvider) Type() string  { return "test" }
func (p *countingProvider) Priority() int { return 0 }
func (p *countingProvider) Close() error  { return nil }
func (p *countingProvider) Healthy(_ context.Context) (bool, int64, error) { return p.err == nil, 0, nil }
func (p *countingProvider) ListVaults(_ context.Context) ([]providers.Vault, error) { return nil, nil }
func (p *countingProvider) ListItems(_ context.Context, _ string) ([]providers.Item, error) { return nil, nil }
func (p *countingProvider) ListFields(_ context.Context, _, _ string) ([]providers.Field, error) { return nil, nil }
func (p *countingProvider) Resolve(_ context.Context, _, _, _ string) (string, error) {
    atomic.AddInt64(&p.calls, 1)
    if p.err != nil { return "", p.err }
    return "secret-value", nil
}

func TestManager_Singleflight(t *testing.T) {
    store, _ := cache.Open(t.TempDir()+"/c.db", "pass")
    defer store.Close()
    cp := &countingProvider{}
    mgr := core.NewManager(store, []providers.Provider{cp}, time.Hour)

    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            mgr.Resolve(context.Background(), "V", "I", "F")
        }()
    }
    wg.Wait()

    if atomic.LoadInt64(&cp.calls) > 1 {
        t.Errorf("singleflight failed: provider called %d times", cp.calls)
    }
}

func TestManager_StaleFallback(t *testing.T) {
    store, _ := cache.Open(t.TempDir()+"/c.db", "pass")
    defer store.Close()
    // Pre-populate with expired entry
    store.Set(context.Background(), "counting/V/I/F", cache.Entry{
        Value: "stale-value", Provider: "counting",
        ExpiresAt: time.Now().Add(-time.Second),
    })
    cp := &countingProvider{err: errors.New("provider down")}
    mgr := core.NewManager(store, []providers.Provider{cp}, time.Hour)

    val, err := mgr.Resolve(context.Background(), "V", "I", "F")
    if err != nil { t.Fatalf("expected stale value, got error: %v", err) }
    if val != "stale-value" {
        t.Errorf("expected stale-value, got %q", val)
    }
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/core/... -v
```

**Step 3: Implement manager.go**

```go
// internal/core/manager.go
package core

import (
    "context"
    "fmt"
    "time"

    "golang.org/x/sync/singleflight"
    "github.com/elabx-org/herald/internal/core/cache"
    "github.com/elabx-org/herald/internal/providers"
)

type Manager struct {
    cache      *cache.Store
    providers  []providers.Provider
    defaultTTL time.Duration
    sf         singleflight.Group
}

func NewManager(c *cache.Store, ps []providers.Provider, defaultTTL time.Duration) *Manager {
    return &Manager{cache: c, providers: ps, defaultTTL: defaultTTL}
}

func (m *Manager) Resolve(ctx context.Context, vault, item, field string) (string, error) {
    key := m.cacheKey(vault, item, field)
    if e, found, err := m.cache.Get(ctx, key); err == nil && found {
        return e.Value, nil
    }

    val, err, _ := m.sf.Do(key, func() (interface{}, error) {
        return m.fetchFromProvider(ctx, vault, item, field)
    })
    if err != nil {
        // Stale fallback
        if e, found, serr := m.cache.GetStale(ctx, key); serr == nil && found {
            return e.Value, nil
        }
        return "", err
    }
    return val.(string), nil
}

func (m *Manager) fetchFromProvider(ctx context.Context, vault, item, field string) (string, error) {
    var lastErr error
    for _, p := range m.providers {
        val, err := p.Resolve(ctx, vault, item, field)
        if err != nil {
            lastErr = err
            continue
        }
        _ = m.cache.Set(ctx, m.providerKey(p, vault, item, field), cache.Entry{
            Value:     val,
            Provider:  p.Name(),
            ExpiresAt: time.Now().Add(m.defaultTTL),
        })
        return val, nil
    }
    if lastErr != nil {
        return "", lastErr
    }
    return "", fmt.Errorf("no providers available")
}

func (m *Manager) cacheKey(vault, item, field string) string {
    // Try first provider name for cache key; use generic prefix for multi-provider lookup
    if len(m.providers) > 0 {
        return m.providerKey(m.providers[0], vault, item, field)
    }
    return fmt.Sprintf("_/%s/%s/%s", vault, item, field)
}

func (m *Manager) providerKey(p providers.Provider, vault, item, field string) string {
    return fmt.Sprintf("%s/%s/%s/%s", p.Name(), vault, item, field)
}
```

**Step 4: Run tests**

```bash
go test ./internal/core/... -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/core/
git commit -m "feat(core): provider manager with singleflight dedup and stale fallback"
```

---

## Phase 4: API Server

### Task 9: HTTP server skeleton with middleware

**Files:**
- Create: `internal/api/server.go`
- Create: `internal/api/middleware.go`
- Create: `internal/api/server_test.go`

**Step 1: Write failing test**

```go
// internal/api/server_test.go
package api_test

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "github.com/elabx-org/herald/internal/api"
)

func TestPing(t *testing.T) {
    srv := api.NewServer(api.Options{})
    req := httptest.NewRequest(http.MethodGet, "/ping", nil)
    w := httptest.NewRecorder()
    srv.ServeHTTP(w, req)
    if w.Code != 200 {
        t.Errorf("expected 200, got %d", w.Code)
    }
}

func TestProtectedWithoutToken(t *testing.T) {
    srv := api.NewServer(api.Options{APIToken: "secret"})
    req := httptest.NewRequest(http.MethodGet, "/v2/inventory", nil)
    w := httptest.NewRecorder()
    srv.ServeHTTP(w, req)
    if w.Code != 401 {
        t.Errorf("expected 401, got %d", w.Code)
    }
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/api/... -v
```

**Step 3: Implement server.go**

```go
// internal/api/server.go
package api

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
)

type Options struct {
    APIToken string
}

type Server struct {
    router chi.Router
    opts   Options
}

func NewServer(opts Options) *Server {
    s := &Server{opts: opts, router: chi.NewRouter()}
    s.router.Use(middleware.Recoverer)
    s.router.Use(requestIDMiddleware)
    s.router.Get("/ping", s.handlePing)

    s.router.Group(func(r chi.Router) {
        r.Get("/v2/health", s.handleHealth)
        r.Get("/v2/stats", s.handleStats)
        r.Get("/metrics", s.handleMetrics)
    })

    s.router.Group(func(r chi.Router) {
        if opts.APIToken != "" {
            r.Use(s.bearerAuthMiddleware)
        }
        r.Use(bodySizeMiddleware(1 << 20)) // 1MB
        r.Post("/v2/materialize/env", s.handleMaterialize)
        r.Get("/v2/inventory", s.handleInventory)
        r.Get("/v2/inventory/{stack}", s.handleInventoryStack)
        r.Get("/v2/audit", s.handleAudit)
        r.Post("/v2/rotate/{item}", s.handleRotate)
        r.Post("/v2/rotate/{vault}/{item}", s.handleRotateVault)
        r.Delete("/v2/cache/{stack}", s.handleCacheDeleteStack)
        r.Delete("/v2/cache", s.handleCacheFlush)
        r.Post("/v2/provision", s.handleProvision)
        r.Get("/v2/events", s.handleSSE)
        // V1 compat aliases
        r.Post("/v1/materialize/env", s.handleMaterialize)
        r.Get("/v1/health", s.handleHealth)
        r.Get("/v1/stats", s.handleStats)
        r.Get("/v1/inventory", s.handleInventory)
    })
    return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    s.router.ServeHTTP(w, r)
}

func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, http.StatusOK, map[string]int{})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("# Prometheus metrics\n"))
}

func (s *Server) handleMaterialize(w http.ResponseWriter, r *http.Request) {
    http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleInventory(w http.ResponseWriter, r *http.Request) {
    http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleInventoryStack(w http.ResponseWriter, r *http.Request) {
    http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
    http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleRotate(w http.ResponseWriter, r *http.Request) {
    http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleRotateVault(w http.ResponseWriter, r *http.Request) {
    http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleCacheDeleteStack(w http.ResponseWriter, r *http.Request) {
    http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleCacheFlush(w http.ResponseWriter, r *http.Request) {
    http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleProvision(w http.ResponseWriter, r *http.Request) {
    http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
    http.Error(w, "not implemented", http.StatusNotImplemented)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, errCode, msg, requestID string) {
    writeJSON(w, code, map[string]string{
        "error":      errCode,
        "message":    msg,
        "request_id": requestID,
    })
}
```

**Step 4: Implement middleware.go**

```go
// internal/api/middleware.go
package api

import (
    "context"
    "net/http"
    "strings"

    "github.com/google/uuid"
)

type ctxKey string
const requestIDKey ctxKey = "request_id"

func requestIDMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id := r.Header.Get("X-Request-ID")
        if id == "" {
            id = uuid.New().String()
        }
        w.Header().Set("X-Request-ID", id)
        ctx := context.WithValue(r.Context(), requestIDKey, id)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func getRequestID(r *http.Request) string {
    if id, ok := r.Context().Value(requestIDKey).(string); ok {
        return id
    }
    return ""
}

func (s *Server) bearerAuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        auth := r.Header.Get("Authorization")
        if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != s.opts.APIToken {
            writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or missing bearer token", getRequestID(r))
            return
        }
        next.ServeHTTP(w, r)
    })
}

func bodySizeMiddleware(max int64) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            r.Body = http.MaxBytesReader(w, r.Body, max)
            next.ServeHTTP(w, r)
        })
    }
}
```

**Step 5: Run tests**

```bash
go test ./internal/api/... -v
```
Expected: PASS

**Step 6: Commit**

```bash
git add internal/api/
git commit -m "feat(api): HTTP server skeleton with bearer auth and middleware"
```

---

### Task 10: Materialize handler (wires resolver + manager)

**Files:**
- Modify: `internal/api/server.go` — add `Manager` field to Options, wire materialize handler
- Create: `internal/api/materialize.go`
- Create: `internal/api/materialize_test.go`

**Step 1: Write failing integration test**

```go
// internal/api/materialize_test.go
package api_test

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/elabx-org/herald/internal/api"
    "github.com/elabx-org/herald/internal/core"
    "github.com/elabx-org/herald/internal/core/cache"
    "github.com/elabx-org/herald/internal/providers"
)

type fixedProvider struct{ val string }
func (p *fixedProvider) Name() string  { return "fixed" }
func (p *fixedProvider) Type() string  { return "test" }
func (p *fixedProvider) Priority() int { return 0 }
func (p *fixedProvider) Close() error  { return nil }
func (p *fixedProvider) Healthy(_ context.Context) (bool, int64, error) { return true, 0, nil }
func (p *fixedProvider) ListVaults(_ context.Context) ([]providers.Vault, error) { return nil, nil }
func (p *fixedProvider) ListItems(_ context.Context, _ string) ([]providers.Item, error) { return nil, nil }
func (p *fixedProvider) ListFields(_ context.Context, _, _ string) ([]providers.Field, error) { return nil, nil }
func (p *fixedProvider) Resolve(_ context.Context, _, _, _ string) (string, error) {
    return p.val, nil
}

func TestMaterialize_ResolvesRefs(t *testing.T) {
    store, _ := cache.Open(t.TempDir()+"/c.db", "pass")
    defer store.Close()
    fp := &fixedProvider{val: "resolved-secret"}
    mgr := core.NewManager(store, []providers.Provider{fp}, time.Hour)

    srv := api.NewServer(api.Options{Manager: mgr})
    body, _ := json.Marshal(map[string]string{
        "stack":       "teststack",
        "env_content": "DB_PASS=herald://V/I/F\n",
    })
    req := httptest.NewRequest(http.MethodPost, "/v2/materialize/env", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    srv.ServeHTTP(w, req)

    if w.Code != 200 {
        t.Fatalf("expected 200, got %d: %s", w.Code, w.Body)
    }
    var resp map[string]interface{}
    json.NewDecoder(w.Body).Decode(&resp)
    if content, ok := resp["content"].(string); !ok || content != "DB_PASS=resolved-secret\n" {
        t.Errorf("unexpected content: %v", resp["content"])
    }
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/api/... -run TestMaterialize -v
```

**Step 3: Update Options struct and implement materialize handler**

Add to `server.go`:
```go
type Options struct {
    APIToken string
    Manager  *core.Manager
}
```

Create `internal/api/materialize.go`:
```go
package api

import (
    "context"
    "encoding/json"
    "net/http"
    "time"

    "github.com/elabx-org/herald/internal/resolver"
)

type materializeRequest struct {
    Stack      string `json:"stack"`
    EnvContent string `json:"env_content"`
    OutPath    string `json:"out_path"`
    BypassCache bool  `json:"bypass_cache"`
}

type materializeResponse struct {
    Resolved   int    `json:"resolved"`
    CacheHits  int    `json:"cache_hits"`
    StaleHits  int    `json:"stale_hits"`
    Failed     int    `json:"failed"`
    DurationMs int64  `json:"duration_ms"`
    Content    string `json:"content"`
}

func (s *Server) handleMaterialize(w http.ResponseWriter, r *http.Request) {
    if s.opts.Manager == nil {
        writeError(w, http.StatusServiceUnavailable, "no_providers", "no providers configured", getRequestID(r))
        return
    }
    var req materializeRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid_body", err.Error(), getRequestID(r))
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
    defer cancel()

    start := time.Now()
    refs := resolver.ScanRefs(req.EnvContent)
    resolved := make(map[string]string, len(refs))
    resp := materializeResponse{}

    for _, ref := range refs {
        val, err := s.opts.Manager.Resolve(ctx, ref.Vault, ref.Item, ref.Field)
        if err != nil {
            resp.Failed++
            continue
        }
        resolved[ref.Raw] = val
        resp.Resolved++
    }
    resp.Content = resolver.SubstituteRefs(req.EnvContent, resolved)
    resp.DurationMs = time.Since(start).Milliseconds()
    writeJSON(w, http.StatusOK, resp)
}
```

**Step 4: Run tests**

```bash
go test ./internal/api/... -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/api/
git commit -m "feat(api): materialize handler with resolver integration"
```

---

## Phase 5: Wiring + Runnable Server

### Task 11: main.go — wire everything into a runnable server

**Files:**
- Modify: `cmd/herald/main.go`

**Step 1: Implement main.go**

```go
// cmd/herald/main.go
package main

import (
    "context"
    "net/http"
    "os"
    "os/signal"
    "strconv"
    "syscall"
    "time"

    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"

    "github.com/elabx-org/herald/internal/api"
    "github.com/elabx-org/herald/internal/config"
    "github.com/elabx-org/herald/internal/core"
    "github.com/elabx-org/herald/internal/core/cache"
    "github.com/elabx-org/herald/internal/providers"
    mockprovider "github.com/elabx-org/herald/internal/providers/mock"
)

func main() {
    zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
    log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

    cfgPath := os.Getenv("HERALD_CONFIG")
    cfg, err := config.Load(cfgPath)
    if err != nil {
        log.Fatal().Err(err).Msg("failed to load config")
    }

    // Cache
    var cacheStore *cache.Store
    if cfg.Cache.Key != "" {
        cacheStore, err = cache.Open(cfg.Cache.DataPath, cfg.Cache.Key)
        if err != nil {
            log.Fatal().Err(err).Str("path", cfg.Cache.DataPath).Msg("failed to open cache")
        }
        defer cacheStore.Close()
        log.Info().Str("path", cfg.Cache.DataPath).Msg("cache initialized")
    }

    // Providers (mock provider for dev; real providers added in later phases)
    var ps []providers.Provider
    if mockPath := os.Getenv("HERALD_MOCK_SECRETS"); mockPath != "" {
        mp, err := mockprovider.New("mock", mockPath, 99)
        if err != nil {
            log.Fatal().Err(err).Msg("failed to load mock provider")
        }
        ps = append(ps, mp)
        log.Info().Str("path", mockPath).Msg("mock provider initialized")
    }

    var mgr *core.Manager
    if len(ps) > 0 && cacheStore != nil {
        mgr = core.NewManager(cacheStore, ps, time.Duration(cfg.Cache.DefaultTTLSecs)*time.Second)
    }

    // Server
    srv := api.NewServer(api.Options{
        APIToken: os.Getenv("HERALD_API_TOKEN"),
        Manager:  mgr,
    })

    httpSrv := &http.Server{
        Addr:         ":" + strconv.Itoa(cfg.Port),
        Handler:      srv,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 60 * time.Second,
        IdleTimeout:  120 * time.Second,
    }

    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
    defer stop()

    go func() {
        log.Info().Int("port", cfg.Port).Msg("herald starting")
        if err := httpSrv.ListenAndServe(); err != http.ErrServerClosed {
            log.Fatal().Err(err).Msg("server error")
        }
    }()

    <-ctx.Done()
    log.Info().Msg("shutting down")
    shutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    httpSrv.Shutdown(shutCtx)
    log.Info().Msg("shutdown complete")
}
```

**Step 2: Build and verify**

```bash
go build ./cmd/herald/
```
Expected: `./herald` binary created

**Step 3: Smoke test with mock provider**

```bash
# Create minimal secrets file
echo 'secrets:
  HomeLab:
    myapp:
      db_pass: "test123"' > /tmp/secrets.yaml

HERALD_MOCK_SECRETS=/tmp/secrets.yaml HERALD_CACHE_KEY=testkey \
    HERALD_CACHE_DATA_PATH=/tmp/herald-test.db ./herald &
sleep 1

curl -s http://localhost:8765/ping
# Expected: {"ok":true}

curl -s -X POST http://localhost:8765/v2/materialize/env \
  -H 'Content-Type: application/json' \
  -d '{"stack":"test","env_content":"PASS=herald://HomeLab/myapp/db_pass\n"}'
# Expected: {"content":"PASS=test123\n",...}

kill %1
```

**Step 4: Commit**

```bash
git add cmd/herald/main.go
git commit -m "feat: wire server with config, cache, mock provider, graceful shutdown"
```

---

## Phase 6: 1Password Providers

### Task 12: 1Password Connect provider

> Prerequisite: 1Password Connect running at `OP_CONNECT_SERVER_URL` with `OP_CONNECT_TOKEN`.

**Files:**
- Create: `internal/providers/onepassword/connect.go` (build tag: `onepassword`)
- Create: `internal/providers/onepassword/connect_test.go`

**Note:** This provider uses the 1Password Connect REST API (pure HTTP, no CGO required). The SDK provider (CGO) comes in Task 13.

**Step 1: Write failing test (requires Connect server — use integration tag)**

```go
//go:build integration
// internal/providers/onepassword/connect_test.go
package onepassword_test

import (
    "context"
    "os"
    "testing"
    "github.com/elabx-org/herald/internal/providers/onepassword"
)

func TestConnect_Resolve(t *testing.T) {
    url := os.Getenv("OP_CONNECT_SERVER_URL")
    token := os.Getenv("OP_CONNECT_TOKEN")
    if url == "" || token == "" {
        t.Skip("OP_CONNECT_SERVER_URL and OP_CONNECT_TOKEN required")
    }
    p, err := onepassword.NewConnect("test-connect", url, token, 0)
    if err != nil { t.Fatal(err) }
    defer p.Close()

    ok, _, err := p.Healthy(context.Background())
    if err != nil || !ok { t.Fatalf("unhealthy: %v", err) }
}
```

**Step 2: Implement connect.go** (pure REST, no CGO)

```go
// internal/providers/onepassword/connect.go
package onepassword

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/elabx-org/herald/internal/providers"
)

type ConnectProvider struct {
    name     string
    url      string
    token    string
    priority int
    client   *http.Client
}

func NewConnect(name, url, token string, priority int) (*ConnectProvider, error) {
    return &ConnectProvider{
        name:     name,
        url:      url,
        token:    token,
        priority: priority,
        client:   &http.Client{Timeout: 15 * time.Second},
    }, nil
}

func (p *ConnectProvider) Name() string  { return p.name }
func (p *ConnectProvider) Type() string  { return "connect_server" }
func (p *ConnectProvider) Priority() int { return p.priority }
func (p *ConnectProvider) Close() error  { return nil }

func (p *ConnectProvider) Resolve(ctx context.Context, vault, item, field string) (string, error) {
    vaultID, err := p.resolveVaultID(ctx, vault)
    if err != nil { return "", err }
    itemObj, err := p.resolveItem(ctx, vaultID, item)
    if err != nil { return "", err }
    for _, f := range itemObj.Fields {
        if f.Label == field {
            return f.Value, nil
        }
    }
    return "", fmt.Errorf("connect: field %q not found in %s/%s", field, vault, item)
}

func (p *ConnectProvider) Healthy(ctx context.Context) (bool, int64, error) {
    start := time.Now()
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.url+"/v1/vaults", nil)
    req.Header.Set("Authorization", "Bearer "+p.token)
    resp, err := p.client.Do(req)
    if err != nil { return false, 0, err }
    resp.Body.Close()
    return resp.StatusCode == 200, time.Since(start).Milliseconds(), nil
}

func (p *ConnectProvider) ListVaults(ctx context.Context) ([]providers.Vault, error) {
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.url+"/v1/vaults", nil)
    req.Header.Set("Authorization", "Bearer "+p.token)
    resp, err := p.client.Do(req)
    if err != nil { return nil, err }
    defer resp.Body.Close()
    var raw []struct{ ID, Name string }
    json.NewDecoder(resp.Body).Decode(&raw)
    var vaults []providers.Vault
    for _, v := range raw {
        vaults = append(vaults, providers.Vault{ID: v.ID, Name: v.Name})
    }
    return vaults, nil
}

func (p *ConnectProvider) ListItems(ctx context.Context, vault string) ([]providers.Item, error) {
    vaultID, err := p.resolveVaultID(ctx, vault)
    if err != nil { return nil, err }
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.url+"/v1/vaults/"+vaultID+"/items", nil)
    req.Header.Set("Authorization", "Bearer "+p.token)
    resp, err := p.client.Do(req)
    if err != nil { return nil, err }
    defer resp.Body.Close()
    var raw []struct{ ID, Title string; Category string }
    json.NewDecoder(resp.Body).Decode(&raw)
    var items []providers.Item
    for _, i := range raw {
        items = append(items, providers.Item{ID: i.ID, Name: i.Title, Category: i.Category})
    }
    return items, nil
}

func (p *ConnectProvider) ListFields(ctx context.Context, vault, item string) ([]providers.Field, error) {
    vaultID, _ := p.resolveVaultID(ctx, vault)
    obj, err := p.resolveItem(ctx, vaultID, item)
    if err != nil { return nil, err }
    var fields []providers.Field
    for _, f := range obj.Fields {
        fields = append(fields, providers.Field{ID: f.ID, Label: f.Label, Concealed: f.Type == "CONCEALED"})
    }
    return fields, nil
}

// Internal helpers
type connectItem struct {
    ID     string
    Fields []struct{ ID, Label, Value, Type string }
}

func (p *ConnectProvider) resolveVaultID(ctx context.Context, name string) (string, error) {
    vaults, err := p.ListVaults(ctx)
    if err != nil { return "", err }
    for _, v := range vaults {
        if v.Name == name { return v.ID, nil }
    }
    return "", fmt.Errorf("connect: vault %q not found", name)
}

func (p *ConnectProvider) resolveItem(ctx context.Context, vaultID, itemName string) (connectItem, error) {
    url := fmt.Sprintf("%s/v1/vaults/%s/items?filter=title eq %q", p.url, vaultID, itemName)
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    req.Header.Set("Authorization", "Bearer "+p.token)
    resp, err := p.client.Do(req)
    if err != nil { return connectItem{}, err }
    defer resp.Body.Close()
    var items []connectItem
    json.NewDecoder(resp.Body).Decode(&items)
    if len(items) == 0 {
        return connectItem{}, fmt.Errorf("connect: item %q not found", itemName)
    }
    // Fetch full item with fields
    req2, _ := http.NewRequestWithContext(ctx, http.MethodGet,
        fmt.Sprintf("%s/v1/vaults/%s/items/%s", p.url, vaultID, items[0].ID), nil)
    req2.Header.Set("Authorization", "Bearer "+p.token)
    resp2, err := p.client.Do(req2)
    if err != nil { return connectItem{}, err }
    defer resp2.Body.Close()
    var full connectItem
    json.NewDecoder(resp2.Body).Decode(&full)
    return full, nil
}
```

**Step 3: Build to verify compilation**

```bash
go build ./internal/providers/onepassword/
```

**Step 4: Register Connect provider in main.go** (add after mock provider block)

```go
if url := os.Getenv("OP_CONNECT_SERVER_URL"); url != "" {
    if token := os.Getenv("OP_CONNECT_TOKEN"); token != "" {
        cp, err := onepassword.NewConnect("1password-connect", url, token, 0)
        if err != nil {
            log.Fatal().Err(err).Msg("failed to initialize Connect provider")
        }
        ps = append(ps, cp)
        log.Info().Str("url", url).Msg("1Password Connect provider initialized")
    }
}
```

**Step 5: Commit**

```bash
git add internal/providers/onepassword/ cmd/herald/main.go
git commit -m "feat(providers): 1Password Connect provider (REST, no CGO)"
```

---

### Task 13: 1Password SDK provider (CGO)

**Files:**
- Create: `internal/providers/onepassword/sdk.go` (build tag: `onepassword_sdk`)

**Note:** This requires `CGO_ENABLED=1` and `github.com/1password/onepassword-sdk-go`. Add the dependency:

```bash
CGO_ENABLED=1 go get github.com/1password/onepassword-sdk-go@latest
```

**Implement sdk.go** following the same Provider interface pattern as connect.go but using the 1Password Go SDK client. Mark the file with `//go:build onepassword_sdk`. Register in main.go behind `OP_SERVICE_ACCOUNT_TOKEN`.

**Build test:**
```bash
CGO_ENABLED=1 go build -tags onepassword_sdk ./...
```

**Commit:**
```bash
git commit -m "feat(providers): 1Password SDK provider (CGO, service account)"
```

---

## Phase 7: Integrations

### Task 14: Integration interface + Komodo integration

**Files:**
- Create: `internal/integrations/interface.go`
- Create: `internal/integrations/komodo/komodo.go`
- Create: `internal/integrations/komodo/komodo_test.go`

**Step 1: Define interface**

```go
// internal/integrations/interface.go
package integrations

import "context"

type Event struct {
    Type  string // "rotation", "health_change"
    Stack string
    Data  map[string]string
}

type Integration interface {
    Name() string
    Type() string
    Deploy(ctx context.Context, stack string) error
    Notify(ctx context.Context, event Event) error
    Healthy(ctx context.Context) (bool, error)
}
```

**Step 2: Implement Komodo integration** (move existing `internal/komodo/` logic here, drain response bodies)

```go
// internal/integrations/komodo/komodo.go
package komodo

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"
    "time"

    "github.com/elabx-org/herald/internal/integrations"
)

type Integration struct {
    name      string
    url       string
    apiKey    string
    apiSecret string
    client    *http.Client
}

func New(name, url, apiKey, apiSecret string) *Integration {
    return &Integration{
        name: name, url: url, apiKey: apiKey, apiSecret: apiSecret,
        client: &http.Client{Timeout: 30 * time.Second},
    }
}

func (k *Integration) Name() string { return k.name }
func (k *Integration) Type() string { return "komodo" }

func (k *Integration) Deploy(ctx context.Context, stack string) error {
    body, _ := json.Marshal(map[string]string{"stack": stack})
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
        k.url+"/execute/DeployStack",
        strings.NewReader(string(body)))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Api-Key", k.apiKey)
    req.Header.Set("X-Api-Secret", k.apiSecret)
    resp, err := k.client.Do(req)
    if err != nil { return err }
    defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()
    if resp.StatusCode >= 400 {
        return fmt.Errorf("komodo: deploy %q returned %d", stack, resp.StatusCode)
    }
    return nil
}

func (k *Integration) Notify(_ context.Context, _ integrations.Event) error { return nil }

func (k *Integration) Healthy(ctx context.Context) (bool, error) {
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, k.url+"/auth/me", nil)
    req.Header.Set("X-Api-Key", k.apiKey)
    req.Header.Set("X-Api-Secret", k.apiSecret)
    resp, err := k.client.Do(req)
    if err != nil { return false, err }
    defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()
    return resp.StatusCode == 200, nil
}
```

**Step 3: Write integration test**

```go
//go:build integration
// internal/integrations/komodo/komodo_test.go
// ... (test against real Komodo instance, skip if env vars not set)
```

**Step 4: Commit**

```bash
git add internal/integrations/
git commit -m "feat(integrations): integration interface and Komodo deploy integration"
```

---

## Phase 8: Rotation Handler

### Task 15: Rotation with cache invalidation + fanout

**Files:**
- Create: `internal/api/rotate.go`
- Create: `internal/api/rotate_test.go`

The rotate handler should:
1. Parse item (and optionally vault) from URL
2. Identify stacks referencing that item (from index)
3. Delete matching cache entries from BoltDB
4. Fan out to all registered integrations in parallel goroutines
5. Use `context.WithoutCancel` for the deploy goroutines (survive disconnect)
6. Return summary: `{item_id, cache_invalidated, stacks_redeployed}`

See existing `internal/api/rotate.go` in v1 for reference — port to v2 with integration fanout.

```bash
git add internal/api/rotate.go
git commit -m "feat(api): rotation handler with cache invalidation and integration fanout"
```

---

## Phase 9: UI

### Task 16: React frontend scaffold

**Files:**
- Create: `ui/package.json`
- Create: `ui/vite.config.ts`
- Create: `ui/tailwind.config.js`
- Create: `ui/src/main.tsx`
- Create: `ui/src/App.tsx`

**Step 1: Scaffold Vite project**

```bash
cd ui
npm create vite@latest . -- --template react-ts
npm install
npm install -D tailwindcss @tailwindcss/vite
npm install @radix-ui/react-dialog @radix-ui/react-dropdown-menu @radix-ui/react-toast
npm install framer-motion
npm install react-virtual
```

**Step 2: Configure Vite proxy for local dev**

```typescript
// ui/vite.config.ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    proxy: {
      '/v1': 'http://localhost:8765',
      '/v2': 'http://localhost:8765',
      '/ping': 'http://localhost:8765',
    }
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  }
})
```

**Step 3: Implement Aurora design system** in `ui/src/styles/aurora.css`

```css
:root {
  --bg: #060b14;
  --cyan: #22d3ee;
  --violet: #818cf8;
  --emerald: #34d399;
  --amber: #fbbf24;
  --glass: rgba(255,255,255,0.04);
  --glass-border: rgba(255,255,255,0.08);
}
body { background: var(--bg); color: #e2e8f0; font-family: 'Inter', sans-serif; }
```

**Step 4: Implement book-split login page** with Canvas particle animation (left) and glass card form (right)

**Step 5: Implement sidebar layout** with collapsible navigation

**Step 6: Build and verify embed**

```bash
cd ui && npm run build
cd .. && go build -tags "embed_ui" ./cmd/herald/
```

**Step 7: Commit**

```bash
git add ui/
git commit -m "feat(ui): Aurora design system, book-split login, dashboard scaffold"
```

---

### Task 17: Embed UI in Go binary

**Files:**
- Create: `cmd/herald/embed.go`

```go
// cmd/herald/embed.go
//go:build embed_ui

package main

import "embed"

//go:embed ../../ui/dist
var uiFS embed.FS
```

In `server.go`, serve the embedded UI under `/`:
```go
// Serve UI for all non-API routes
r.Handle("/*", http.FileServer(http.FS(uiFS)))
```

Build:
```bash
cd ui && npm run build && cd ..
go build -tags embed_ui ./cmd/herald/
```

Commit:
```bash
git commit -m "feat: embed React UI in Go binary via embed.FS"
```

---

## Phase 10: Observability

### Task 18: Prometheus metrics

**Files:**
- Create: `internal/api/metrics.go`

Register key metrics at package init and update them in handler wrappers:

```go
var (
    materializeDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
        Name: "herald_materialize_duration_seconds",
        Buckets: prometheus.DefBuckets,
    }, []string{"provider", "stack"})
    cacheHits   = prometheus.NewCounterVec(...)
    cacheMisses = prometheus.NewCounterVec(...)
    providerHealthy = prometheus.NewGaugeVec(...)
)

func init() {
    prometheus.MustRegister(materializeDuration, cacheHits, cacheMisses, providerHealthy)
}
```

Update `handleMetrics` to use `promhttp.Handler()`.

```bash
go get github.com/prometheus/client_golang/prometheus/promhttp@latest
go test ./internal/api/... -v
git commit -m "feat(metrics): Prometheus metrics endpoint"
```

---

## Phase 11: Integration Testing & Deployment

### Task 19: End-to-end test with real Herald binary

**Files:**
- Create: `e2e/e2e_test.go` (build tag: `e2e`)

Start Herald with mock provider, run materialize request, verify response:

```go
//go:build e2e
package e2e_test

// Start herald binary, run materialize, verify response
// Use os/exec to start the binary; use httptest.NewRequest against real HTTP
```

```bash
make build
go test -tags e2e ./e2e/... -v
```

---

### Task 20: Build + Deploy via Komodo

**Prerequisite:** Commit + push all code first.

**Step 1: Push to GitHub**

```bash
git push origin main
```

**Step 2: Build Herald v2 via Komodo**

```
mcp__komodo__run_build(build="herald")
```

Monitor: `mcp__komodo__get_build_action_state(build="herald")`

**Step 3: Deploy Herald stack**

```
mcp__komodo__deploy_stack(stack="herald")
```

**Step 4: Verify health**

```bash
curl http://10.0.0.9:8765/ping
curl http://10.0.0.9:8765/v2/health
```

---

## Execution Order Summary

| Phase | Tasks | Prerequisite |
|-------|-------|-------------|
| 1: Foundation | 1–3 | None |
| 2: Providers | 4–5 | Phase 1 |
| 3: Resolver & Cache | 6–8 | Phase 2 |
| 4: API | 9–10 | Phase 3 |
| 5: Wiring | 11 | Phase 4 |
| 6: 1P Providers | 12–13 | Phase 5 |
| 7: Integrations | 14 | Phase 5 |
| 8: Rotation | 15 | Phases 7+3 |
| 9: UI | 16–17 | Phase 5 (can be parallel) |
| 10: Observability | 18 | Phase 4 |
| 11: E2E + Deploy | 19–20 | All phases |

**Tasks 6–8 and 4–5 can be parallelized across worktrees.**
**UI (Tasks 16–17) can start in parallel after Task 11.**
