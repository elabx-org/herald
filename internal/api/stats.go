package api

import (
	"encoding/json"
	"net/http"
	"time"
)

type statsResponse struct {
	UptimeSeconds int64   `json:"uptime_seconds"`
	TotalSyncs    int64   `json:"total_syncs"`
	TotalResolved int64   `json:"total_resolved"`
	TotalCacheHits int64  `json:"total_cache_hits"`
	TotalStaleHits int64  `json:"total_stale_hits"`
	TotalFailed   int64   `json:"total_failed"`
	CacheHitRate  float64 `json:"cache_hit_rate"` // 0.0–1.0, fraction of fetches served from cache
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	syncs := s.statSyncs.Load()
	resolved := s.statResolved.Load()
	cacheHits := s.statCacheHits.Load()
	staleHits := s.statStaleHits.Load()
	failed := s.statFailed.Load()

	total := resolved + cacheHits + staleHits
	var hitRate float64
	if total > 0 {
		hitRate = float64(cacheHits) / float64(total)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statsResponse{
		UptimeSeconds:  int64(time.Since(startTime).Seconds()),
		TotalSyncs:     syncs,
		TotalResolved:  resolved,
		TotalCacheHits: cacheHits,
		TotalStaleHits: staleHits,
		TotalFailed:    failed,
		CacheHitRate:   hitRate,
	})
}
