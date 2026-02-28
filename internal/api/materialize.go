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
	EnvContent string `json:"env_content"` // raw env file content with op:// refs
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
