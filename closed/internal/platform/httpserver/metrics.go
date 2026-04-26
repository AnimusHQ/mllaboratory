package httpserver

import (
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	metricsStartTime = time.Now().UTC()
	metricsMu        sync.RWMutex
	metricsProviders []func(io.Writer)
)

// RegisterMetrics exposes a minimal Prometheus-compatible endpoint.
func RegisterMetrics(mux *http.ServeMux, service string) {
	if mux == nil {
		return
	}
	service = strings.TrimSpace(service)
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		uptime := time.Since(metricsStartTime).Seconds()
		fmt.Fprintf(w, "# HELP animus_uptime_seconds Service uptime in seconds.\n")
		fmt.Fprintf(w, "# TYPE animus_uptime_seconds gauge\n")
		fmt.Fprintf(w, "animus_uptime_seconds{service=\"%s\"} %.0f\n", service, uptime)
		fmt.Fprintf(w, "# HELP animus_go_goroutines Number of goroutines.\n")
		fmt.Fprintf(w, "# TYPE animus_go_goroutines gauge\n")
		fmt.Fprintf(w, "animus_go_goroutines{service=\"%s\"} %d\n", service, runtime.NumGoroutine())
		metricsMu.RLock()
		providers := append([]func(io.Writer){}, metricsProviders...)
		metricsMu.RUnlock()
		for _, provider := range providers {
			if provider != nil {
				provider(w)
			}
		}
	})
}

// RegisterMetricsProvider adds a callback for custom metrics to /metrics.
func RegisterMetricsProvider(provider func(io.Writer)) {
	if provider == nil {
		return
	}
	metricsMu.Lock()
	metricsProviders = append(metricsProviders, provider)
	metricsMu.Unlock()
}
