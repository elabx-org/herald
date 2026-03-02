package api

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"go.etcd.io/bbolt"

	"github.com/elabx-org/herald/internal/audit"
	"github.com/elabx-org/herald/internal/core"
	"github.com/elabx-org/herald/internal/core/cache"
	"github.com/elabx-org/herald/internal/integrations"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Options struct {
	APIToken     string
	Manager      *core.Manager
	Integrations []integrations.Integration
	UIFS         fs.FS
	AuditLogger  *audit.Logger
	IndexDB      *bbolt.DB // optional: bbolt DB for index persistence (same db as cache)
}

// stackEntry tracks which items a stack has resolved.
type stackEntry struct {
	Stack        string    `json:"stack"`
	Items        []string  `json:"items"`
	Refs         []string  `json:"refs,omitempty"`
	LastSeen     time.Time `json:"last_seen"`
	ResolveCount int       `json:"resolve_count"`
}

var indexBucket = []byte("index")

type Server struct {
	router    chi.Router
	opts      Options
	startTime time.Time

	indexMu sync.RWMutex
	index   map[string]*stackEntry // stack name → entry
}

func NewServer(opts Options) *Server {
	s := &Server{
		opts:      opts,
		router:    chi.NewRouter(),
		startTime: time.Now(),
		index:     make(map[string]*stackEntry),
	}
	// Load persisted index from bbolt if available
	if opts.IndexDB != nil {
		opts.IndexDB.Update(func(tx *bbolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists(indexBucket)
			return err
		})
		s.loadIndex()
	}

	s.router.Use(middleware.Recoverer)
	s.router.Use(requestIDMiddleware)
	s.router.Get("/ping", s.handlePing)

	s.router.Group(func(r chi.Router) {
		r.Get("/v2/health", s.handleHealth)
		r.Get("/v1/health", s.handleHealth)
		r.Get("/v2/stats", s.handleStats)
		r.Get("/v1/stats", s.handleStats)
		r.Get("/metrics", s.handleMetrics)
	})

	s.router.Group(func(r chi.Router) {
		if opts.APIToken != "" {
			r.Use(s.bearerAuthMiddleware)
		}
		r.Use(bodySizeMiddleware(1 << 20)) // 1MB
		r.Post("/v2/materialize/env", s.handleMaterialize)
		r.Post("/v1/materialize/env", s.handleMaterialize)
		r.Get("/v2/inventory", s.handleInventory)
		r.Get("/v1/inventory", s.handleInventory)
		r.Get("/v2/inventory/{stack}", s.handleInventoryStack)
		r.Get("/v1/inventory/{stack}", s.handleInventoryStack)
		r.Get("/v2/audit", s.handleAudit)
		r.Get("/v1/audit", s.handleAudit)
		r.Post("/v2/rotate/{item}", s.handleRotate)
		r.Post("/v1/rotate/{itemID}", s.handleRotate)
		r.Post("/v2/rotate/{vault}/{item}", s.handleRotateVault)
		r.Post("/v1/rotate/{vault}/{itemID}", s.handleRotateVault)
		r.Get("/v2/cache", s.handleCacheList)
		r.Delete("/v2/cache/{stack}", s.handleCacheDeleteStack)
		r.Delete("/v1/cache/{stack}", s.handleCacheDeleteStack)
		r.Delete("/v2/cache", s.handleCacheFlush)
		r.Delete("/v1/cache", s.handleCacheFlush)
		r.Post("/v2/provision", s.handleProvision)
		r.Post("/v1/provision", s.handleProvision)
		r.Get("/v2/events", s.handleSSE)
		r.Get("/v2/providers", s.handleProviders)
		r.Post("/v2/providers/check", s.handleProvidersCheck)
	})

	// Serve embedded UI (when built with embed_ui tag)
	if opts.UIFS != nil {
		s.router.Handle("/*", http.FileServer(http.FS(opts.UIFS)))
	}

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// loadIndex reads the persisted stack index from bbolt on startup.
func (s *Server) loadIndex() {
	if s.opts.IndexDB == nil {
		return
	}
	s.opts.IndexDB.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(indexBucket)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var e stackEntry
			if err := json.Unmarshal(v, &e); err == nil {
				s.index[string(k)] = &e
			}
			return nil
		})
	})
}

// saveIndexEntry persists a single stack entry to bbolt.
func (s *Server) saveIndexEntry(e *stackEntry) {
	if s.opts.IndexDB == nil {
		return
	}
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	s.opts.IndexDB.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(indexBucket)
		if b == nil {
			return nil
		}
		return b.Put([]byte(e.Stack), data)
	})
}

// indexUpsert records that a stack resolved a set of items and their raw URIs.
func (s *Server) indexUpsert(stack string, items []string, refs []string) {
	if stack == "" {
		return
	}
	s.indexMu.Lock()
	defer s.indexMu.Unlock()
	e, ok := s.index[stack]
	if !ok {
		e = &stackEntry{Stack: stack}
		s.index[stack] = e
	}
	// merge items (deduplicate)
	seenItems := make(map[string]bool, len(e.Items))
	for _, it := range e.Items {
		seenItems[it] = true
	}
	for _, it := range items {
		if !seenItems[it] {
			e.Items = append(e.Items, it)
			seenItems[it] = true
		}
	}
	// merge refs (deduplicate)
	seenRefs := make(map[string]bool, len(e.Refs))
	for _, r := range e.Refs {
		seenRefs[r] = true
	}
	for _, r := range refs {
		if !seenRefs[r] {
			e.Refs = append(e.Refs, r)
			seenRefs[r] = true
		}
	}
	e.LastSeen = time.Now()
	e.ResolveCount++
	s.saveIndexEntry(e)
}

func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"status":         "ok",
		"uptime_seconds": int64(time.Since(s.startTime).Seconds()),
	}
	if s.opts.Manager != nil {
		resp["providers"] = s.opts.Manager.ProviderStatuses()
	}
	writeJSON(w, http.StatusOK, resp)
}

type statsResponse struct {
	CacheEntries  int      `json:"cache_entries"`
	Providers     []string `json:"providers"`
	Stacks        int      `json:"stacks"`
	UptimeSeconds int64    `json:"uptime_seconds"`
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	resp := statsResponse{
		UptimeSeconds: int64(time.Since(s.startTime).Seconds()),
	}
	if s.opts.Manager != nil {
		resp.CacheEntries = s.opts.Manager.CacheCount()
		resp.Providers = s.opts.Manager.ProviderNames()
	}
	s.indexMu.RLock()
	resp.Stacks = len(s.index)
	s.indexMu.RUnlock()
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleInventory(w http.ResponseWriter, r *http.Request) {
	s.indexMu.RLock()
	out := make([]*stackEntry, 0, len(s.index))
	for _, e := range s.index {
		out = append(out, e)
	}
	s.indexMu.RUnlock()
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleInventoryStack(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "stack")
	s.indexMu.RLock()
	e, ok := s.index[name]
	s.indexMu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "stack not found", getRequestID(r))
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	if s.opts.AuditLogger == nil {
		writeJSON(w, http.StatusOK, []struct{}{})
		return
	}
	q := r.URL.Query()
	opts := audit.QueryOptions{
		Stack:  q.Get("stack"),
		Secret: q.Get("secret"),
	}
	entries, err := s.opts.AuditLogger.Query(opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query_error", err.Error(), getRequestID(r))
		return
	}
	if entries == nil {
		entries = []audit.Entry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) handleCacheList(w http.ResponseWriter, r *http.Request) {
	if s.opts.Manager == nil {
		writeJSON(w, http.StatusOK, []struct{}{})
		return
	}
	entries := s.opts.Manager.CacheList()
	if entries == nil {
		entries = []cache.ListEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) handleProvidersCheck(w http.ResponseWriter, r *http.Request) {
	if s.opts.Manager == nil {
		writeJSON(w, http.StatusOK, []struct{}{})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	writeJSON(w, http.StatusOK, s.opts.Manager.CheckNow(ctx))
}

func (s *Server) handleCacheDeleteStack(w http.ResponseWriter, r *http.Request) {
	stack := chi.URLParam(r, "stack")
	if s.opts.Manager == nil {
		writeError(w, http.StatusServiceUnavailable, "no_cache", "cache not configured", getRequestID(r))
		return
	}
	// Look up items for this stack from index
	s.indexMu.RLock()
	e, ok := s.index[stack]
	s.indexMu.RUnlock()
	if !ok {
		writeJSON(w, http.StatusOK, map[string]int{"flushed": 0})
		return
	}
	total := 0
	for _, item := range e.Items {
		total += s.opts.Manager.FlushItem(r.Context(), "", item)
	}
	writeJSON(w, http.StatusOK, map[string]int{"flushed": total})
}

func (s *Server) handleCacheFlush(w http.ResponseWriter, r *http.Request) {
	if s.opts.Manager == nil {
		writeError(w, http.StatusServiceUnavailable, "no_cache", "cache not configured", getRequestID(r))
		return
	}
	if err := s.opts.Manager.FlushAll(); err != nil {
		writeError(w, http.StatusInternalServerError, "flush_error", err.Error(), getRequestID(r))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
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
