package httpserver

import (
	"fmt"
	"io"
	"sync"
	"time"
)

type httpMetricsStore struct {
	mu            sync.Mutex
	requestCounts map[httpRequestKey]uint64
	latencies     map[httpLatencyKey]*latencyMetric
}

type httpRequestKey struct {
	Service     string
	Method      string
	StatusClass string
}

type httpLatencyKey struct {
	Service string
	Method  string
}

type latencyMetric struct {
	buckets []float64
	counts  []uint64
	sum     float64
	count   uint64
}

func newHTTPMetricsStore() *httpMetricsStore {
	return &httpMetricsStore{
		requestCounts: make(map[httpRequestKey]uint64),
		latencies:     make(map[httpLatencyKey]*latencyMetric),
	}
}

var httpMetricsStoreInstance = newHTTPMetricsStore()
var httpMetricsOnce sync.Once

func ensureHTTPMetricsRegistered() {
	httpMetricsOnce.Do(func() {
		RegisterMetricsProvider(httpMetricsStoreInstance.WritePrometheus)
	})
}

func recordHTTPMetrics(service, method string, status int, duration time.Duration) {
	if service == "" || method == "" {
		return
	}
	statusClass := fmt.Sprintf("%dxx", status/100)
	key := httpRequestKey{Service: service, Method: method, StatusClass: statusClass}
	latencyKey := httpLatencyKey{Service: service, Method: method}

	httpMetricsStoreInstance.mu.Lock()
	httpMetricsStoreInstance.requestCounts[key]++
	metric, ok := httpMetricsStoreInstance.latencies[latencyKey]
	if !ok {
		metric = &latencyMetric{
			buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			counts:  make([]uint64, 11),
		}
		httpMetricsStoreInstance.latencies[latencyKey] = metric
	}
	metric.observe(duration.Seconds())
	httpMetricsStoreInstance.mu.Unlock()
}

func (m *latencyMetric) observe(value float64) {
	m.sum += value
	m.count++
	for i, bucket := range m.buckets {
		if value <= bucket {
			m.counts[i]++
		}
	}
}

func (m *httpMetricsStore) WritePrometheus(w io.Writer) {
	if w == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	fmt.Fprint(w, "# HELP animus_http_requests_total Total HTTP requests.\n")
	fmt.Fprint(w, "# TYPE animus_http_requests_total counter\n")
	for key, count := range m.requestCounts {
		fmt.Fprintf(w, "animus_http_requests_total{service=\"%s\",method=\"%s\",status_class=\"%s\"} %d\n",
			key.Service, key.Method, key.StatusClass, count)
	}

	fmt.Fprint(w, "# HELP animus_http_request_duration_seconds HTTP request latency buckets.\n")
	fmt.Fprint(w, "# TYPE animus_http_request_duration_seconds histogram\n")
	for key, metric := range m.latencies {
		for i, bucket := range metric.buckets {
			fmt.Fprintf(w, "animus_http_request_duration_seconds_bucket{service=\"%s\",method=\"%s\",le=\"%.3f\"} %d\n",
				key.Service, key.Method, bucket, metric.counts[i])
		}
		fmt.Fprintf(w, "animus_http_request_duration_seconds_sum{service=\"%s\",method=\"%s\"} %.6f\n",
			key.Service, key.Method, metric.sum)
		fmt.Fprintf(w, "animus_http_request_duration_seconds_count{service=\"%s\",method=\"%s\"} %d\n",
			key.Service, key.Method, metric.count)
	}
}
