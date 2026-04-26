package auditexport

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"
)

type metricsKey struct {
	sinkType string
	outcome  AttemptOutcome
}

type latencyMetric struct {
	sumNs uint64
	count uint64
}

var (
	metricsMu     sync.Mutex
	attemptTotals = map[metricsKey]uint64{}
	latencyTotals = map[string]latencyMetric{}
)

func recordAttemptMetrics(sinkType string, outcome AttemptOutcome, latency time.Duration) {
	sinkType = strings.TrimSpace(sinkType)
	if sinkType == "" {
		sinkType = "unknown"
	}
	metricsMu.Lock()
	attemptTotals[metricsKey{sinkType: sinkType, outcome: outcome}]++
	metric := latencyTotals[sinkType]
	metric.sumNs += uint64(latency)
	metric.count++
	latencyTotals[sinkType] = metric
	metricsMu.Unlock()
}

// PrometheusMetrics emits audit export delivery metrics.
func PrometheusMetrics(store DeliveryStore) func(io.Writer) {
	return func(w io.Writer) {
		if w == nil {
			return
		}
		metricsMu.Lock()
		attemptCopy := make(map[metricsKey]uint64, len(attemptTotals))
		for k, v := range attemptTotals {
			attemptCopy[k] = v
		}
		latencyCopy := make(map[string]latencyMetric, len(latencyTotals))
		for k, v := range latencyTotals {
			latencyCopy[k] = v
		}
		metricsMu.Unlock()

		fmt.Fprint(w, "# HELP animus_audit_export_attempts_total Total audit export delivery attempts.\n")
		fmt.Fprint(w, "# TYPE animus_audit_export_attempts_total counter\n")
		keys := make([]metricsKey, 0, len(attemptCopy))
		for k := range attemptCopy {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].sinkType == keys[j].sinkType {
				return keys[i].outcome < keys[j].outcome
			}
			return keys[i].sinkType < keys[j].sinkType
		})
		for _, k := range keys {
			fmt.Fprintf(w, "animus_audit_export_attempts_total{sink_type=\"%s\",outcome=\"%s\"} %d\n", k.sinkType, k.outcome, attemptCopy[k])
		}

		fmt.Fprint(w, "# HELP animus_audit_export_latency_seconds Audit export delivery latency summary.\n")
		fmt.Fprint(w, "# TYPE animus_audit_export_latency_seconds summary\n")
		latencyKeys := make([]string, 0, len(latencyCopy))
		for k := range latencyCopy {
			latencyKeys = append(latencyKeys, k)
		}
		sort.Strings(latencyKeys)
		for _, k := range latencyKeys {
			metric := latencyCopy[k]
			fmt.Fprintf(w, "animus_audit_export_latency_seconds_sum{sink_type=\"%s\"} %.6f\n", k, float64(metric.sumNs)/float64(time.Second))
			fmt.Fprintf(w, "animus_audit_export_latency_seconds_count{sink_type=\"%s\"} %d\n", k, metric.count)
		}

		dlqSize := 0
		if store != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer cancel()
			if count, err := store.CountByStatus(ctx, DeliveryStatusDLQ); err == nil {
				dlqSize = count
			}
		}
		fmt.Fprint(w, "# HELP animus_audit_export_dlq_size Audit export DLQ size.\n")
		fmt.Fprint(w, "# TYPE animus_audit_export_dlq_size gauge\n")
		fmt.Fprintf(w, "animus_audit_export_dlq_size %d\n", dlqSize)
	}
}
