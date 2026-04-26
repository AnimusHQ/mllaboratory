package webhooks

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

type metrics struct {
	attempts     uint64
	successes    uint64
	failures     uint64
	latencySumNs uint64
	latencyCount uint64
}

var metricsCollector = &metrics{}

func recordAttemptMetrics(outcome AttemptOutcome, latency time.Duration) {
	atomic.AddUint64(&metricsCollector.attempts, 1)
	switch outcome {
	case AttemptOutcomeSuccess:
		atomic.AddUint64(&metricsCollector.successes, 1)
	case AttemptOutcomePermanentFailure:
		atomic.AddUint64(&metricsCollector.failures, 1)
	}
	atomic.AddUint64(&metricsCollector.latencySumNs, uint64(latency))
	atomic.AddUint64(&metricsCollector.latencyCount, 1)
}

// PrometheusMetrics emits webhook delivery metrics.
func PrometheusMetrics(w io.Writer) {
	if w == nil {
		return
	}
	attempts := atomic.LoadUint64(&metricsCollector.attempts)
	successes := atomic.LoadUint64(&metricsCollector.successes)
	failures := atomic.LoadUint64(&metricsCollector.failures)
	latencySum := atomic.LoadUint64(&metricsCollector.latencySumNs)
	latencyCount := atomic.LoadUint64(&metricsCollector.latencyCount)

	fmt.Fprint(w, "# HELP animus_webhook_delivery_attempts_total Total webhook delivery attempts.\n")
	fmt.Fprint(w, "# TYPE animus_webhook_delivery_attempts_total counter\n")
	fmt.Fprintf(w, "animus_webhook_delivery_attempts_total %d\n", attempts)
	fmt.Fprint(w, "# HELP animus_webhook_delivery_success_total Total successful webhook deliveries.\n")
	fmt.Fprint(w, "# TYPE animus_webhook_delivery_success_total counter\n")
	fmt.Fprintf(w, "animus_webhook_delivery_success_total %d\n", successes)
	fmt.Fprint(w, "# HELP animus_webhook_delivery_failure_total Total failed webhook deliveries.\n")
	fmt.Fprint(w, "# TYPE animus_webhook_delivery_failure_total counter\n")
	fmt.Fprintf(w, "animus_webhook_delivery_failure_total %d\n", failures)
	fmt.Fprint(w, "# HELP animus_webhook_delivery_latency_seconds Webhook delivery latency summary.\n")
	fmt.Fprint(w, "# TYPE animus_webhook_delivery_latency_seconds summary\n")
	fmt.Fprintf(w, "animus_webhook_delivery_latency_seconds_sum %.6f\n", float64(latencySum)/float64(time.Second))
	fmt.Fprintf(w, "animus_webhook_delivery_latency_seconds_count %d\n", latencyCount)
	fmt.Fprint(w, "# HELP animus_webhook_dlq_size Webhook DLQ size (not used).\n")
	fmt.Fprint(w, "# TYPE animus_webhook_dlq_size gauge\n")
	fmt.Fprint(w, "animus_webhook_dlq_size 0\n")
}
