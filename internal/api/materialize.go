package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/elabx-org/herald/internal/audit"
	"github.com/elabx-org/herald/internal/resolver"
)

type materializeRequest struct {
	Stack       string `json:"stack"`
	EnvContent  string `json:"env_content"`
	OutPath     string `json:"out_path"`
	BypassCache bool   `json:"bypass_cache"`
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

	itemsSeen := make(map[string]bool)
	refsSeen := make(map[string]bool)
	for _, ref := range refs {
		val, err := s.opts.Manager.Resolve(ctx, ref.Vault, ref.Item, ref.Field)
		if err != nil {
			resp.Failed++
			continue
		}
		resolved[ref.Raw] = val
		resp.Resolved++
		itemsSeen[ref.Item] = true
		refsSeen[ref.Raw] = true
	}

	// Update stack index
	if req.Stack != "" {
		items := make([]string, 0, len(itemsSeen))
		for item := range itemsSeen {
			items = append(items, item)
		}
		rawRefs := make([]string, 0, len(refsSeen))
		for r := range refsSeen {
			rawRefs = append(rawRefs, r)
		}
		s.indexUpsert(req.Stack, items, rawRefs)
	}

	resp.Content = resolver.SubstituteRefs(req.EnvContent, resolved)
	resp.DurationMs = time.Since(start).Milliseconds()

	if s.opts.AuditLogger != nil && req.Stack != "" {
		status := "ok"
		if resp.Failed > 0 {
			status = "partial"
		}
		if resp.Resolved == 0 && resp.Failed > 0 {
			status = "error"
		}
		s.opts.AuditLogger.Log(audit.Entry{
			Action:     "materialize",
			Stack:      req.Stack,
			DurationMs: resp.DurationMs,
			Policy:     status,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}
