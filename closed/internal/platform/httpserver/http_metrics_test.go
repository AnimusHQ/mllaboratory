package httpserver

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestHTTPMetricsWritePrometheus(t *testing.T) {
	httpMetricsStoreInstance = newHTTPMetricsStore()
	recordHTTPMetrics("svc", "GET", 200, 15*time.Millisecond)

	var buf bytes.Buffer
	httpMetricsStoreInstance.WritePrometheus(&buf)
	out := buf.String()

	if !strings.Contains(out, "animus_http_requests_total{service=\"svc\",method=\"GET\",status_class=\"2xx\"} 1") {
		t.Fatalf("expected request counter in metrics: %s", out)
	}
	if !strings.Contains(out, "animus_http_request_duration_seconds_count{service=\"svc\",method=\"GET\"} 1") {
		t.Fatalf("expected latency count in metrics: %s", out)
	}
}
