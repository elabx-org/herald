package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/elabx-org/herald/internal/materialize"
	"github.com/elabx-org/herald/internal/resolver"
	"github.com/rs/zerolog/log"
)

type materializeEnvRequest struct {
	Stack        string `json:"stack"`
	OutPath      string `json:"out_path"`
	EnvContent   string `json:"env_content"`   // raw env file content with op:// refs
	BypassCache  bool   `json:"bypass_cache"`  // if true, skip cache read+write (always fetch fresh)
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
		if err := json.NewEncoder(w).Encode(materializeEnvResponse{
			OutPath: req.OutPath,
			Content: req.EnvContent,
		}); err != nil {
			log.Error().Err(err).Str("stack", req.Stack).Msg("materialize: encode response failed")
		}
		return
	}

	if s.manager == nil {
		http.Error(w, "no secret provider configured", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	store := s.cache
	if req.BypassCache {
		store = nil
	}
	mat := materialize.NewEnvMaterializer(store, s.manager, s.cfg.Cache.DefaultPolicy, s.cfg.Cache.DefaultTTL)
	content, result, err := mat.Materialize(ctx, req.Stack, refs, req.EnvContent, req.OutPath)
	if err != nil {
		log.Error().Err(err).Str("stack", req.Stack).Str("out", req.OutPath).Msg("materialize: failed")
		http.Error(w, "materialize failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Update stack index: tracks which stacks reference which 1Password items,
	// enabling /v1/inventory queries and /v1/rotate/{item} targeted redeployment.
	itemRefs := make(map[string][]string)
	for rawURI, ref := range refs {
		itemRefs[ref.Item] = append(itemRefs[ref.Item], rawURI)
	}
	s.index.Upsert(req.Stack, &StackInfo{
		SecretCount: len(refs),
		Providers:   s.manager.Names(),
		Policies:    []string{s.cfg.Cache.DefaultPolicy},
		LastSynced:  time.Now(),
		ItemRefs:    itemRefs,
	})

	log.Info().
		Str("stack", req.Stack).
		Str("out", req.OutPath).
		Int("resolved", result.Resolved).
		Int("cache_hits", result.CacheHits).
		Int64("duration_ms", result.DurationMs).
		Msg("materialize: complete")

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(materializeEnvResponse{
		Resolved:   result.Resolved,
		CacheHits:  result.CacheHits,
		Failed:     result.Failed,
		DurationMs: result.DurationMs,
		OutPath:    req.OutPath,
		Content:    content,
	}); err != nil {
		log.Error().Err(err).Str("stack", req.Stack).Msg("materialize: encode response failed")
	}
}
