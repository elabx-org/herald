package api

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	materializeDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "herald_materialize_duration_seconds",
		Help:    "Duration of materialize requests in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"provider", "stack"})

	cacheHits = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "herald_cache_hits_total",
		Help: "Total number of cache hits.",
	}, []string{"provider"})

	cacheMisses = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "herald_cache_misses_total",
		Help: "Total number of cache misses.",
	}, []string{"provider"})

	rotateTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "herald_rotate_total",
		Help: "Total number of rotate requests.",
	}, []string{"vault"})
)

func init() {
	prometheus.MustRegister(materializeDuration, cacheHits, cacheMisses, rotateTotal)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	promhttp.Handler().ServeHTTP(w, r)
}
