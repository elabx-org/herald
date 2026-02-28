package api

import (
	"encoding/json"
	"net/http"
)

type materializeEnvRequest struct {
	Stack   string `json:"stack"`
	OutPath string `json:"out_path"`
}

type materializeEnvResponse struct {
	Resolved   int    `json:"resolved"`
	CacheHits  int    `json:"cache_hits"`
	Failed     int    `json:"failed"`
	DurationMs int64  `json:"duration_ms"`
	OutPath    string `json:"out_path"`
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
	if req.OutPath == "" {
		req.OutPath = "/run/herald/" + req.Stack + ".env"
	}

	// Scan extra.env for the stack (implementation in Phase 7 when repo cache is ready)
	// For now return a stub
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(materializeEnvResponse{OutPath: req.OutPath})
}
