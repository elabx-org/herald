# Herald Secret Delivery v2 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Upgrade Herald to output complete resolved env files (pass-through non-secrets, substitute op:// refs) and support stdout output, enabling the industry-standard Doppler/Infisical deploy pattern for all Komodo stacks.

**Architecture:** Four changes in sequence: (1) add `ResolveEnvContent` to the resolver package that does line-by-line substitution; (2) update `EnvMaterializer.Materialize` to take full env content and return resolved content string; (3) update the API handler to return content in the response and skip file write when `out_path` is empty; (4) update herald-agent to print to stdout when `--out -`. Then update herald-test to use the new pattern, and write the README.

**Tech Stack:** Go 1.24, `github.com/elabx-org/herald` module, Komodo builds for CI (no local Go available — push to trigger build after each task).

---

## Context you need to know

### Key file locations
- `internal/resolver/env_scanner.go` — scans env content for op:// refs
- `internal/materialize/env.go` — resolves refs and writes output file
- `internal/api/materialize.go` — HTTP handler for POST /v1/materialize/env
- `cmd/herald-agent/sync.go` — CLI sync command (sends request to API)
- `elmerfds/stacks` repo at `/config/workspace/gh/stacks` — compose files for all stacks

### How Komodo periphery works (critical context)
Komodo periphery runs **inside a container** with the host Docker socket mounted. This means:
- `mkdir`, `chmod` shell commands in `pre_deploy` → affect **periphery's container filesystem only**
- `docker exec`, `docker run -v` → use the **host Docker daemon** → affect real containers/host
- `.env.resolved` written in pre_deploy CWD lives in periphery's container filesystem
- Docker compose reads `.env.resolved` from periphery's CWD (accessible because compose CLI runs there)
- This is fine: docker compose passes resolved values to Docker daemon, the file is ephemeral

### Current behaviour being replaced
- `--out` defaults to `/run/herald/{stack}.env` (host bind mount, requires chmod 777 — insecure)
- API writes only resolved secret key=value pairs (loses non-secret lines)
- Response body doesn't include resolved content

### Target behaviour
- `--out -` → herald-agent prints resolved content to stdout (periphery captures via `> .env.resolved`)
- `--out /path` → write to file (for backward compat if ever needed)
- Default `--out` changes to `-` (stdout)
- API returns complete resolved env content (all lines, op:// substituted, comments/blanks preserved)
- Response includes `content` field with resolved env content

### The deploy pattern for every stack after this
```bash
# Komodo pre_deploy:
cat extra.env | docker exec -i herald /herald-agent sync --stack mystack --env-file - > .env.resolved
# (--out defaults to - now, so no flag needed)

# Komodo post_deploy:
rm -f .env.resolved

# compose.yaml service:
env_file:
  - .env.resolved
```

### Running tests
No Go installed locally. Tests are validated through the Komodo build pipeline:
1. Push code to `elabx-org/herald` on GitHub
2. Komodo build `herald` auto-triggers (webhook enabled)
3. Check build logs for test output
4. If tests fail, fix and push again

---

## Task 1: Add `ResolveEnvContent` to resolver package

**Files:**
- Modify: `internal/resolver/env_scanner.go`
- Test: `internal/resolver/env_scanner_test.go`

This function takes the raw env file content and a map of `key → resolved value`, and returns the complete env content with op:// values substituted. Non-secret lines, comments, and blank lines are preserved exactly.

**Step 1: Add the test**

Add to `internal/resolver/env_scanner_test.go`:

```go
func TestResolveEnvContent(t *testing.T) {
	content := "# Comment\nDB_PASSWORD=op://Vault/pg/password\nAPP_URL=https://example.com\nSMTP_KEY=op://Vault/smtp/key\n\n"
	resolved := map[string]string{
		"DB_PASSWORD": "secret-db-pass",
		"SMTP_KEY":    "secret-smtp-key",
	}

	got := resolver.ResolveEnvContent(content, resolved)

	if !strings.Contains(got, "DB_PASSWORD=secret-db-pass") {
		t.Errorf("expected DB_PASSWORD resolved, got:\n%s", got)
	}
	if !strings.Contains(got, "SMTP_KEY=secret-smtp-key") {
		t.Errorf("expected SMTP_KEY resolved, got:\n%s", got)
	}
	if !strings.Contains(got, "APP_URL=https://example.com") {
		t.Errorf("expected APP_URL preserved, got:\n%s", got)
	}
	if !strings.Contains(got, "# Comment") {
		t.Errorf("expected comment preserved, got:\n%s", got)
	}
	if strings.Contains(got, "op://") {
		t.Errorf("expected no op:// refs in output, got:\n%s", got)
	}
}
```

**Step 2: Add the implementation**

Add to the bottom of `internal/resolver/env_scanner.go`:

```go
// ResolveEnvContent returns the env file content with op:// refs replaced by
// their resolved values. Comments, blank lines, and non-secret lines are
// preserved unchanged.
func ResolveEnvContent(content string, resolved map[string]string) string {
	var sb strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			sb.WriteString(line + "\n")
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) == 2 {
			if val, ok := resolved[parts[0]]; ok {
				sb.WriteString(parts[0] + "=" + val + "\n")
				continue
			}
		}
		sb.WriteString(line + "\n")
	}
	return sb.String()
}
```

Note: `bufio` and `strings` are already imported in this file.

**Step 3: Commit and push**

```bash
cd /config/workspace/gh/herald
git add internal/resolver/env_scanner.go internal/resolver/env_scanner_test.go
git commit -m "feat: add ResolveEnvContent for complete env file pass-through"
git push origin main
```

Then trigger the Komodo build and verify it passes.

---

## Task 2: Update `EnvMaterializer.Materialize` to return resolved content

**Files:**
- Modify: `internal/materialize/env.go`
- Modify: `internal/materialize/env_test.go`

Change the signature so `Materialize` takes the full `envContent string`, returns `(string, *Result, error)` where the string is the complete resolved env content. File write is optional (only when `outPath != ""`).

**Step 1: Update the test**

Replace `TestMaterializeEnv` in `internal/materialize/env_test.go`:

```go
func TestMaterializeEnv(t *testing.T) {
	dir, _ := os.MkdirTemp("", "herald-mat-*")
	defer os.RemoveAll(dir)

	store, _ := cache.New(dir+"/cache.db", "test-key-32chars-exactly!!!!!!")
	defer store.Close()

	refs := map[string]*resolver.SecretRef{
		"DB_PASSWORD": {Vault: "Vault", Item: "item", Field: "password", Raw: "op://Vault/item/password"},
	}
	envContent := "APP_URL=https://example.com\nDB_PASSWORD=op://Vault/item/password\n"

	mat := materialize.NewEnvMaterializer(store, &mockMgr{val: "resolved-secret"}, "memory", 3600)
	outPath := dir + "/test.env"

	content, result, err := mat.Materialize(context.Background(), "myapp", refs, envContent, outPath)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if result.Resolved != 1 {
		t.Errorf("Resolved = %d, want 1", result.Resolved)
	}
	if !strings.Contains(content, "DB_PASSWORD=resolved-secret") {
		t.Errorf("content missing DB_PASSWORD=resolved-secret, got:\n%s", content)
	}
	if !strings.Contains(content, "APP_URL=https://example.com") {
		t.Errorf("content missing APP_URL pass-through, got:\n%s", content)
	}

	// File should also be written
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != content {
		t.Errorf("file content differs from returned content")
	}
}

func TestMaterializeEnvNoFile(t *testing.T) {
	refs := map[string]*resolver.SecretRef{
		"SECRET": {Vault: "V", Item: "i", Field: "f", Raw: "op://V/i/f"},
	}
	envContent := "SECRET=op://V/i/f\n"

	mat := materialize.NewEnvMaterializer(nil, &mockMgr{val: "val"}, "memory", 3600)

	// outPath="" means no file write
	content, result, err := mat.Materialize(context.Background(), "myapp", refs, envContent, "")
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if result.Resolved != 1 {
		t.Errorf("Resolved = %d, want 1", result.Resolved)
	}
	if !strings.Contains(content, "SECRET=val") {
		t.Errorf("content missing SECRET=val, got:\n%s", content)
	}
}
```

**Step 2: Rewrite `internal/materialize/env.go`**

```go
package materialize

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/elabx-org/herald/internal/cache"
	"github.com/elabx-org/herald/internal/resolver"
)

type Resolver interface {
	Resolve(ctx context.Context, vault, item, field string) (string, string, error)
}

type Result struct {
	Resolved   int
	CacheHits  int
	Failed     int
	DurationMs int64
}

type EnvMaterializer struct {
	store         *cache.Store
	manager       Resolver
	defaultPolicy string
	defaultTTL    int
}

func NewEnvMaterializer(store *cache.Store, mgr Resolver, defaultPolicy string, defaultTTL int) *EnvMaterializer {
	return &EnvMaterializer{store: store, manager: mgr, defaultPolicy: defaultPolicy, defaultTTL: defaultTTL}
}

// Materialize resolves all op:// refs in envContent and returns the complete
// resolved env content (non-secret lines preserved). If outPath is non-empty,
// the resolved content is also written to that file.
func (m *EnvMaterializer) Materialize(ctx context.Context, stack string, refs map[string]*resolver.SecretRef, envContent string, outPath string) (string, *Result, error) {
	start := time.Now()
	result := &Result{}
	resolvedVals := make(map[string]string)

	for varName, ref := range refs {
		cacheKey := fmt.Sprintf("%s/%s/%s", ref.Vault, ref.Item, ref.Field)

		if m.store != nil {
			if entry, err := m.store.Get(cacheKey); err == nil {
				resolvedVals[varName] = entry.Value
				result.CacheHits++
				continue
			}
		}

		val, providerName, err := m.manager.Resolve(ctx, ref.Vault, ref.Item, ref.Field)
		if err != nil {
			result.Failed++
			return "", nil, fmt.Errorf("resolve %s (%s): %w", varName, ref.Raw, err)
		}

		if m.store != nil {
			m.store.Set(cacheKey, &cache.Entry{
				Value:     val,
				Provider:  providerName,
				Policy:    m.defaultPolicy,
				ExpiresAt: time.Now().Add(time.Duration(m.defaultTTL) * time.Second),
			})
		}

		resolvedVals[varName] = val
		result.Resolved++
	}

	// Build complete resolved env content
	content := resolver.ResolveEnvContent(envContent, resolvedVals)

	// Write to file if path specified
	if outPath != "" {
		if err := writeFile(outPath, content); err != nil {
			return "", nil, fmt.Errorf("write env file: %w", err)
		}
	}

	result.DurationMs = time.Since(start).Milliseconds()
	return content, result, nil
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}
```

**Step 3: Commit and push**

```bash
cd /config/workspace/gh/herald
git add internal/materialize/env.go internal/materialize/env_test.go
git commit -m "feat: Materialize returns complete resolved env content, file write optional"
git push origin main
```

---

## Task 3: Update the API handler

**Files:**
- Modify: `internal/api/materialize.go`

Changes:
- Add `Content` to response struct
- Remove mandatory `env_content` check (empty content is valid — stack with no secrets)
- Pass `envContent` to `Materialize`
- Return `content` in response
- When `out_path` is empty, skip file write (outPath="" → no file)
- When no refs found, still return the full env content (pass-through)

**Step 1: Rewrite `internal/api/materialize.go`**

```go
package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/elabx-org/herald/internal/materialize"
	"github.com/elabx-org/herald/internal/resolver"
	"github.com/rs/zerolog/log"
)

type materializeEnvRequest struct {
	Stack      string `json:"stack"`
	OutPath    string `json:"out_path"`
	EnvContent string `json:"env_content"`
}

type materializeEnvResponse struct {
	Resolved   int    `json:"resolved"`
	CacheHits  int    `json:"cache_hits"`
	Failed     int    `json:"failed"`
	DurationMs int64  `json:"duration_ms"`
	OutPath    string `json:"out_path,omitempty"`
	Content    string `json:"content"`
}

func (s *Server) handleMaterializeEnv(w http.ResponseWriter, r *http.Request) {
	var req materializeEnvRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Stack == "" {
		http.Error(w, "stack is required", http.StatusBadRequest)
		return
	}

	// Parse env_content for op:// references
	refs, err := resolver.ScanEnvFile(strings.NewReader(req.EnvContent))
	if err != nil {
		log.Error().Err(err).Str("stack", req.Stack).Msg("materialize: failed to scan env content")
		http.Error(w, "failed to scan env content: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(refs) == 0 {
		// No secrets — return env content unchanged
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(materializeEnvResponse{
			OutPath: req.OutPath,
			Content: req.EnvContent,
		})
		return
	}

	if s.manager == nil {
		http.Error(w, "no secret provider configured", http.StatusServiceUnavailable)
		return
	}

	mat := materialize.NewEnvMaterializer(s.cache, s.manager, s.cfg.Cache.DefaultPolicy, s.cfg.Cache.DefaultTTL)
	content, result, err := mat.Materialize(r.Context(), req.Stack, refs, req.EnvContent, req.OutPath)
	if err != nil {
		log.Error().Err(err).Str("stack", req.Stack).Str("out", req.OutPath).Msg("materialize: failed")
		http.Error(w, "materialize failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Info().
		Str("stack", req.Stack).
		Str("out", req.OutPath).
		Int("resolved", result.Resolved).
		Int("cache_hits", result.CacheHits).
		Int64("duration_ms", result.DurationMs).
		Msg("materialize: complete")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(materializeEnvResponse{
		Resolved:   result.Resolved,
		CacheHits:  result.CacheHits,
		Failed:     result.Failed,
		DurationMs: result.DurationMs,
		OutPath:    req.OutPath,
		Content:    content,
	})
}
```

**Step 2: Commit and push**

```bash
cd /config/workspace/gh/herald
git add internal/api/materialize.go
git commit -m "feat: materialize API returns complete resolved content, out_path optional"
git push origin main
```

---

## Task 4: Update herald-agent sync command

**Files:**
- Modify: `cmd/herald-agent/sync.go`

Changes:
- Default `--out` is now `-` (stdout) instead of `/run/herald/{stack}.env`
- When `--out -`: send `out_path: ""` to API, read `content` from response, print to stdout
- When `--out /path`: send `out_path`, write content to file (or rely on server-side write)
- Response is now decoded as JSON to get `content` field

**Step 1: Rewrite `cmd/herald-agent/sync.go`**

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var (
	flagStack   string
	flagOut     string
	flagURL     string
	flagToken   string
	flagRetries int
	flagEnvFile string
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Resolve secrets for a stack and write resolved env to stdout or a file",
	RunE:  runSync,
}

func init() {
	syncCmd.Flags().StringVar(&flagStack, "stack", "", "Stack name (required)")
	syncCmd.Flags().StringVar(&flagOut, "out", "-", "Output path: '-' prints to stdout, or an absolute file path")
	syncCmd.Flags().StringVar(&flagURL, "url", envOrDefault("HERALD_URL", "http://herald:8765"), "Herald service URL")
	syncCmd.Flags().StringVar(&flagToken, "token", os.Getenv("HERALD_API_TOKEN"), "Herald API bearer token")
	syncCmd.Flags().IntVar(&flagRetries, "retries", 3, "Number of retries on failure")
	syncCmd.Flags().StringVar(&flagEnvFile, "env-file", "", "Path to env file to scan for op:// refs (use - for stdin)")
	syncCmd.MarkFlagRequired("stack")
	rootCmd.AddCommand(syncCmd)
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func runSync(cmd *cobra.Command, args []string) error {
	envContent, err := readEnvContent(flagEnvFile)
	if err != nil {
		return fmt.Errorf("read env file: %w", err)
	}

	// When outputting to stdout, don't ask server to write a file
	outPath := flagOut
	if outPath == "-" {
		outPath = ""
	}

	payload := map[string]interface{}{
		"stack":       flagStack,
		"out_path":    outPath,
		"env_content": envContent,
	}

	var lastErr error
	var content string
	for attempt := 0; attempt <= flagRetries; attempt++ {
		if attempt > 0 {
			fmt.Fprintf(os.Stderr, "herald-agent: retry %d/%d after error: %v\n", attempt, flagRetries, lastErr)
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}

		content, lastErr = doSync(payload)
		if lastErr == nil {
			break
		}
	}
	if lastErr != nil {
		fmt.Fprintf(os.Stderr, "herald-agent: failed after %d retries: %v\n", flagRetries, lastErr)
		return lastErr
	}

	if flagOut == "-" {
		fmt.Print(content)
	} else {
		if err := os.WriteFile(flagOut, []byte(content), 0600); err != nil {
			return fmt.Errorf("write output file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "herald-agent: secrets written to %s\n", flagOut)
	}
	return nil
}

func readEnvContent(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type syncResponse struct {
	Content string `json:"content"`
}

func doSync(payload map[string]interface{}) (string, error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, flagURL+"/v1/materialize/env", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if flagToken != "" {
		req.Header.Set("Authorization", "Bearer "+flagToken)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("connect to herald: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("herald returned HTTP %d", resp.StatusCode)
	}

	var sr syncResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return sr.Content, nil
}
```

**Step 2: Commit and push**

```bash
cd /config/workspace/gh/herald
git add cmd/herald-agent/sync.go
git commit -m "feat: herald-agent sync --out - prints resolved env to stdout"
git push origin main
```

**Step 3: Trigger Komodo build and verify it succeeds**

```
mcp__komodo__run_build(build="herald")
```

Check build logs. Expected: all tests pass, binary builds.

---

## Task 5: Update herald-test stack to use the new pattern

This retires the `/run/herald` volume approach and validates the new stdout pattern end-to-end.

**Files:**
- Modify: `/config/workspace/gh/stacks/titan/herald-test/compose.yaml`

**Step 1: Update the compose to remove the volume mount and use env_file**

```yaml
services:
  herald-test:
    image: alpine:latest
    container_name: herald-test
    restart: "no"
    env_file:
      - .env.resolved
    command: >
      sh -c '
        echo "=== Herald Integration Test ==="
        echo ""
        echo "--- Env Injection Test ---"
        if [ -n "$$TEST_USERNAME" ]; then
          echo "[PASS] TEST_USERNAME is set (length=$${#TEST_USERNAME})"
        else
          echo "[FAIL] TEST_USERNAME is empty"
        fi
        if [ -n "$$TEST_PASSWORD" ]; then
          echo "[PASS] TEST_PASSWORD is set (length=$${#TEST_PASSWORD})"
        else
          echo "[FAIL] TEST_PASSWORD is empty"
        fi
        if [ -n "$$TEST_API_KEY" ]; then
          echo "[PASS] TEST_API_KEY is set (length=$${#TEST_API_KEY})"
        else
          echo "[FAIL] TEST_API_KEY is empty"
        fi
        echo ""
        echo "--- op:// URIs must NOT appear in env (secret leak check) ---"
        if env | grep -q "op://"; then
          echo "[FAIL] Raw op:// URI found in environment - secrets not resolved!"
        else
          echo "[PASS] No raw op:// URIs in environment"
        fi
        echo ""
        echo "--- NON_SECRET_VAR must be present ---"
        if [ -n "$$NON_SECRET_VAR" ]; then
          echo "[PASS] NON_SECRET_VAR is set: $$NON_SECRET_VAR"
        else
          echo "[FAIL] NON_SECRET_VAR is missing (non-secret pass-through broken)"
        fi
        echo ""
        echo "=== Test Complete ==="
        sleep infinity
      '
    logging:
      driver: "json-file"
      options:
        max-size: "1m"
        max-file: "1"
```

**Step 2: Update the extra.env to use op:// refs for secrets**

`/config/workspace/gh/stacks/titan/herald-test/extra.env`:
```bash
TEST_USERNAME=op://HomeLab/herald-test/username
TEST_PASSWORD=op://HomeLab/herald-test/password
TEST_API_KEY=op://HomeLab/herald-test/api_key
NON_SECRET_VAR=this-is-not-a-secret
```

(This is already correct — no change needed here.)

**Step 3: Update the Komodo herald-test stack pre_deploy and post_deploy**

Using the Komodo MCP tool:
```python
mcp__komodo__update_stack(id="69a30164f3c29ad585fa27db", config={
    "pre_deploy": {
        "path": "",
        "command": "cat extra.env | docker exec -i herald /herald-agent sync --stack herald-test --env-file - > .env.resolved"
    },
    "post_deploy": {
        "path": "",
        "command": "rm -f .env.resolved"
    }
})
```

**Step 4: Remove the old herald stack pre_deploy** (no longer need chmod via docker run)

```python
mcp__komodo__update_stack(id="69a291e3f3c29ad585fa117d", config={
    "pre_deploy": {"path": "", "command": ""}
})
```

**Step 5: Commit compose change and deploy**

```bash
cd /config/workspace/gh/stacks
git add titan/herald-test/compose.yaml
git commit -m "feat: herald-test uses .env.resolved pattern (no /run/herald volume)"
git push origin main
```

Then deploy:
```
mcp__komodo__deploy_stack(stack="herald-test")
```

Check logs:
```
mcp__komodo__get_stack_logs(stack="herald-test", tail=30)
```

Expected output:
```
[PASS] TEST_USERNAME is set (length=...)
[PASS] TEST_PASSWORD is set (length=...)
[PASS] TEST_API_KEY is set (length=...)
[PASS] No raw op:// URIs in environment
[PASS] NON_SECRET_VAR is set: this-is-not-a-secret
```

The `NON_SECRET_VAR` test confirms that non-secret lines are passed through correctly — this is the key new behaviour.

---

## Task 6: Write the README

**Files:**
- Create: `/config/workspace/gh/herald/README.md`

**Step 1: Write the README**

```markdown
# Herald

Herald is a secret middleware service that bridges [1Password](https://1password.com) to [Komodo](https://komo.do)-managed Docker Compose stacks. It replaces plaintext secrets in `extra.env` files with `op://` references that are resolved at deploy time — keeping secrets out of git while using the same env var delivery model as tools like Doppler and Infisical.

## How it works

```
extra.env (committed to git)              .env.resolved (ephemeral during deploy)
─────────────────────────────────────     ──────────────────────────────────────────
APP_URL=https://myapp.example.com     ──▶ APP_URL=https://myapp.example.com
DB_PASSWORD=op://HomeLab/myapp/db_pw  ──▶ DB_PASSWORD=xK9mP2qR7vNsLd
SMTP_KEY=op://HomeLab/myapp/smtp_key  ──▶ SMTP_KEY=sk-live-abc123...
```

At deploy time, Komodo's `pre_deploy` hook pipes `extra.env` through Herald, which resolves every `op://` reference via the 1Password SDK and outputs the complete resolved env file. Docker Compose reads this file and passes the values to containers as standard environment variables. The resolved file is deleted by `post_deploy`.

## Security model

| Risk | Status |
|------|--------|
| Secrets in git | ✅ Eliminated — only `op://` refs in git |
| Secrets on disk (persistent) | ✅ Eliminated — `.env.resolved` deleted post-deploy |
| Secrets on disk (transient) | ⚠️ Exists ~seconds during deploy window |
| Secrets in `docker inspect env` | ⚠️ Accepted — inherent to env var model |
| Unauthorised secret access | ✅ Controlled via 1Password service accounts |
| Audit trail | ✅ Full — 1Password logs every service account access |

This is the same model used by Doppler, Infisical, and 1Password's `op run` CLI. The `docker inspect` risk requires an attacker to already have Docker socket access, at which point full host access is already compromised.

## Architecture

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
┌──────────────────────────────┐
│  Herald container            │
│  (ghcr.io/elabx-org/herald) │
│                              │
│  herald-agent (CLI)          │
│    → POST /v1/materialize/env│
│    ← resolved env content    │
│                              │
│  Herald API (Go HTTP server) │
│    → 1Password SDK           │
│    ← resolved secret values  │
└──────────────────────────────┘
```

Herald runs as a long-lived service. `herald-agent` is a sidecar binary in the same container used by Komodo's `docker exec` to communicate with the Herald API.

## Setup

### 1. Deploy Herald

Herald is deployed as a Komodo stack from `elmerfds/stacks` (`titan/herald/compose.yaml`). It requires two 1Password service accounts:

| Token | 1Password Permission | Purpose |
|-------|----------------------|---------|
| `OP_SERVICE_ACCOUNT_TOKEN` | Read-only, specific vaults | Resolve secrets at deploy time |
| `OP_PROVISION_TOKEN` | Write, specific vaults | Create new secret items via MCP |

Store both tokens as Komodo variables (`HERALD_SA_READ_TOKEN`, `HERALD_SA_PROVISION_TOKEN`) and reference them in the stack's environment config.

### 2. Configure 1Password service accounts

In 1Password, create two service accounts:

**herald-read** (used at deploy time):
- Access: Read items in the vaults containing your secrets (e.g. `HomeLab`)
- Token → Komodo variable `HERALD_SA_READ_TOKEN`

**herald-provision** (used for AI-assisted secret creation):
- Access: Read + Write items in target vaults
- Token → Komodo variable `HERALD_SA_PROVISION_TOKEN`

### 3. Create the herald-internal Docker network

```bash
docker network create herald-internal
```

### 4. Deploy the herald stack

```
mcp__komodo__deploy_stack(stack="herald")
```

Verify it's healthy:
```bash
curl -H "Authorization: Bearer $HERALD_API_TOKEN" http://10.0.0.9:8765/v1/health
```

## Migrating a stack to use Herald

### Step 1: Create secrets in 1Password

Use the MCP tool to create a secret item for the stack:

```
herald_provision_secret(
  vault="HomeLab",
  item="myapp",
  fields={
    "db_password": {"concealed": true},
    "smtp_key": {"concealed": true},
    "api_key": {"concealed": true}
  }
)
```

Note the `op://` references returned (e.g. `op://HomeLab/myapp/db_password`).

### Step 2: Update `extra.env`

Replace plaintext secret values with `op://` references:

```bash
# Before
DB_PASSWORD=plaintext-password-123

# After
DB_PASSWORD=op://HomeLab/myapp/db_password
```

Non-secret values stay as-is. Comments and blank lines are preserved.

### Step 3: Update the compose file

Change services from `env_file: - extra.env` to `env_file: - .env.resolved`:

```yaml
services:
  myapp:
    env_file:
      - .env.resolved   # was: extra.env
```

### Step 4: Configure Komodo stack hooks

In Komodo, update the stack's pre_deploy and post_deploy:

**pre_deploy:**
```bash
cat extra.env | docker exec -i herald /herald-agent sync --stack mystack --env-file - > .env.resolved
```

**post_deploy:**
```bash
rm -f .env.resolved
```

### Step 5: Deploy

```
mcp__komodo__deploy_stack(stack="mystack")
```

The deployment will:
1. `pre_deploy`: Pipe `extra.env` into herald-agent → Herald resolves all `op://` refs → writes `.env.resolved` to Komodo's working directory
2. `docker compose up`: Services read from `.env.resolved`, receive actual secret values
3. `post_deploy`: `.env.resolved` is deleted

## How the `op://` URI format works

```
op://[vault]/[item]/[field]
op://HomeLab/myapp/db_password
      │       │      └── Field name in the item
      │       └────────── Item name (or UUID)
      └────────────────── Vault name (or UUID)
```

Herald resolves the field named `db_password` from the item named `myapp` in the `HomeLab` vault.

## API reference

Herald exposes a JSON HTTP API on port 8765. All endpoints (except `/v1/health`) require `Authorization: Bearer <token>`.

### `GET /v1/health`

Returns service health. No authentication required.

```json
{"status": "ok", "version": "0.0.6"}
```

### `POST /v1/materialize/env`

Resolve `op://` references in env file content and return the complete resolved env content.

**Request:**
```json
{
  "stack": "myapp",
  "env_content": "APP_URL=https://example.com\nDB_PASSWORD=op://HomeLab/myapp/db_password\n",
  "out_path": ""
}
```

- `stack`: Stack name (used for logging/audit)
- `env_content`: Raw env file content, may contain `op://` references
- `out_path`: If non-empty, also write resolved content to this file path inside the Herald container

**Response:**
```json
{
  "resolved": 1,
  "cache_hits": 0,
  "failed": 0,
  "duration_ms": 120,
  "content": "APP_URL=https://example.com\nDB_PASSWORD=xK9mP2qR7vNsLd\n"
}
```

- `content`: Complete resolved env file content (all lines, `op://` refs substituted, non-secrets preserved)

### `POST /v1/provision`

Create a new item in 1Password. Requires `OP_PROVISION_TOKEN` configured in the Herald container.

**Request:**
```json
{
  "vault": "HomeLab",
  "item": "myapp",
  "category": "Login",
  "fields": {
    "db_password": {"concealed": true},
    "api_key": {"value": "known-value", "concealed": true},
    "username": {"value": "myapp-user"}
  }
}
```

**Response:**
```json
{
  "vault_id": "abc123...",
  "item_id": "xyz456...",
  "refs": {
    "db_password": "op://HomeLab/myapp/db_password",
    "api_key": "op://HomeLab/myapp/api_key",
    "username": "op://HomeLab/myapp/username"
  }
}
```

## MCP tools (AI-assisted secret management)

Herald ships with an MCP server (`mcp-herald`) that exposes these tools to Claude:

### `herald_provision_secret`

Create a new secret item in 1Password. Herald auto-generates values for empty concealed fields.

### `herald_sync_stack`

Trigger secret synchronisation for a stack (resolve + write env file).

### `herald_health`

Check if the Herald service is reachable.

## Environment variables

| Variable | Source | Purpose |
|----------|--------|---------|
| `HERALD_API_TOKEN` | Komodo variable | Bearer token for Herald API authentication |
| `OP_SERVICE_ACCOUNT_TOKEN` | Komodo variable (`HERALD_SA_READ_TOKEN`) | 1Password read-only service account |
| `OP_PROVISION_TOKEN` | Komodo variable (`HERALD_SA_PROVISION_TOKEN`) | 1Password write service account |
| `HERALD_CACHE_KEY` | Komodo variable | Encryption key for the local secret cache |
| `HERALD_URL` | Stack env | URL of the Herald service (default: `http://herald:8765`) |

## Troubleshooting

### `herald returned HTTP 500` in pre_deploy

Check Herald logs:
```
mcp__komodo__get_stack_logs(stack="herald", tail=50)
```

Common causes:
- 1Password service account token expired or revoked
- Item or vault name in `op://` ref doesn't match 1Password
- Herald container not running

### `.env.resolved` not found

The `pre_deploy` script failed silently. Check the Komodo deploy operation logs:
```
mcp__komodo__list_updates(limit=5)
```

If `cat extra.env | docker exec -i herald ...` fails, it means Herald couldn't be reached. Verify Herald is running:
```
mcp__komodo__get_stack_logs(stack="herald", tail=20)
```

### Secrets still showing `op://` in container

The `pre_deploy` ran but `docker compose` is still using the old `extra.env` instead of `.env.resolved`. Verify the compose file references `.env.resolved` (not `extra.env`) in `env_file:`.
```

**Step 2: Commit the README**

```bash
cd /config/workspace/gh/herald
git add README.md
git commit -m "docs: add comprehensive README covering setup, usage, API, and security model"
git push origin main
```

---

## Task 7: Trigger final build and verify

**Step 1: Trigger Komodo herald build**

```
mcp__komodo__run_build(build="herald")
```

Wait for completion. Expected: all tests pass, binary builds.

**Step 2: Deploy herald with new binary**

```
mcp__komodo__deploy_stack(stack="herald")
```

**Step 3: Deploy herald-test and verify all PASS**

```
mcp__komodo__deploy_stack(stack="herald-test")
mcp__komodo__get_stack_logs(stack="herald-test", tail=30)
```

Expected:
```
[PASS] TEST_USERNAME is set (length=...)
[PASS] TEST_PASSWORD is set (length=...)
[PASS] TEST_API_KEY is set (length=...)
[PASS] No raw op:// URIs in environment
[PASS] NON_SECRET_VAR is set: this-is-not-a-secret
```

The `NON_SECRET_VAR` pass-through test is the key new validation — it confirms non-secret lines survive the resolution pass.
