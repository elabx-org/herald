package api

import (
	"encoding/json"
	"net/http"
	"time"
)

type HealthResponse struct {
	Status    string           `json:"status"`
	Providers []ProviderStatus `json:"providers"`
	Uptime    int64            `json:"uptime_seconds"`
}

type ProviderStatus struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

var startTime = time.Now()

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	statuses := []ProviderStatus{}
	overallOK := true

	if s.manager != nil {
		healths := s.manager.Health(r.Context())
		for _, h := range healths {
			ps := ProviderStatus{Name: h.Name, LatencyMs: h.LatencyMs}
			if h.Healthy {
				ps.Status = "ok"
			} else {
				ps.Status = "degraded"
				ps.Error = h.Error
				overallOK = false
			}
			statuses = append(statuses, ps)
		}
	}

	status := "ok"
	code := http.StatusOK
	if !overallOK && len(statuses) > 0 {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}

	resp := HealthResponse{
		Status:    status,
		Providers: statuses,
		Uptime:    int64(time.Since(startTime).Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(resp)
}
