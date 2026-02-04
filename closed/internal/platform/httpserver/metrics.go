package httpserver

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"
)

var metricsStartTime = time.Now().UTC()

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
	})
}
