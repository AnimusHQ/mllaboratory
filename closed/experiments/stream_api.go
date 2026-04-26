package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type experimentRunStateEvent struct {
	StateID    string          `json:"state_id"`
	RunID      string          `json:"run_id"`
	Status     string          `json:"status"`
	ObservedAt time.Time       `json:"observed_at"`
	Details    json.RawMessage `json:"details"`
}

func writeSSE(w http.ResponseWriter, event string, id string, payload any) error {
	if event != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
			return err
		}
	}
	if id != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", id); err != nil {
			return err
		}
	}
	blob, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", blob); err != nil {
		return err
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

func (api *experimentsAPI) handleStreamExperimentRun(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(r.PathValue("run_id"))
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}

	var one int
	if err := api.db.QueryRowContext(r.Context(), `SELECT 1 FROM experiment_runs WHERE run_id = $1`, runID).Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	afterEventID := int64(0)
	afterEventIDProvided := false
	if raw := strings.TrimSpace(r.URL.Query().Get("after_event_id")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 0 {
			api.writeError(w, r, http.StatusBadRequest, "invalid_after_event_id")
			return
		}
		afterEventID = parsed
		afterEventIDProvided = true
	}

	if !afterEventIDProvided {
		if err := api.db.QueryRowContext(r.Context(), `SELECT COALESCE(MAX(event_id),0) FROM experiment_run_events WHERE run_id = $1`, runID).Scan(&afterEventID); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		api.writeError(w, r, http.StatusInternalServerError, "streaming_not_supported")
		return
	}
	flusher.Flush()

	now := time.Now().UTC()
	_ = writeSSE(w, "ready", "", map[string]any{
		"run_id":     runID,
		"server_ts":  now.Unix(),
		"request_id": r.Header.Get("X-Request-Id"),
	})

	lastEventID := afterEventID
	lastMetricAt := now
	lastMetricID := ""
	lastStateAt := now
	lastStateID := ""

	var (
		stateID    string
		status     string
		observedAt time.Time
		details    []byte
	)
	err := api.db.QueryRowContext(
		r.Context(),
		`SELECT state_id, status, observed_at, details
		 FROM experiment_run_state_events
		 WHERE run_id = $1
		 ORDER BY observed_at DESC, state_id DESC
		 LIMIT 1`,
		runID,
	).Scan(&stateID, &status, &observedAt, &details)
	if err == nil {
		lastStateAt = observedAt.UTC()
		lastStateID = stateID
		_ = writeSSE(w, "status", stateID, experimentRunStateEvent{
			StateID:    stateID,
			RunID:      runID,
			Status:     status,
			ObservedAt: observedAt.UTC(),
			Details:    normalizeJSON(details),
		})
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) && !errors.Is(err, context.Canceled) {
		_ = writeSSE(w, "error", "", map[string]any{"error": "internal_error"})
		return
	}

	poll := time.NewTicker(1 * time.Second)
	heartbeat := time.NewTicker(15 * time.Second)
	defer poll.Stop()
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		case <-poll.C:
			if err := api.streamNewStateEvents(r, w, runID, &lastStateAt, &lastStateID); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return
				}
				_ = writeSSE(w, "error", "", map[string]any{"error": err.Error()})
				return
			}
			if err := api.streamNewMetricSamples(r, w, runID, &lastMetricAt, &lastMetricID); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return
				}
				_ = writeSSE(w, "error", "", map[string]any{"error": err.Error()})
				return
			}
			if err := api.streamNewRunEvents(r, w, runID, &lastEventID); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return
				}
				_ = writeSSE(w, "error", "", map[string]any{"error": err.Error()})
				return
			}
		}
	}
}

func (api *experimentsAPI) streamNewStateEvents(r *http.Request, w http.ResponseWriter, runID string, lastAt *time.Time, lastID *string) error {
	rows, err := api.db.QueryContext(
		r.Context(),
		`SELECT state_id, status, observed_at, details
		 FROM experiment_run_state_events
		 WHERE run_id = $1
		   AND (observed_at > $2 OR (observed_at = $2 AND state_id > $3))
		 ORDER BY observed_at ASC, state_id ASC
		 LIMIT 100`,
		runID,
		lastAt.UTC(),
		*lastID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			stateID    string
			status     string
			observedAt time.Time
			details    []byte
		)
		if err := rows.Scan(&stateID, &status, &observedAt, &details); err != nil {
			return err
		}
		*lastAt = observedAt.UTC()
		*lastID = stateID
		if err := writeSSE(w, "status", stateID, experimentRunStateEvent{
			StateID:    stateID,
			RunID:      runID,
			Status:     status,
			ObservedAt: observedAt.UTC(),
			Details:    normalizeJSON(details),
		}); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (api *experimentsAPI) streamNewMetricSamples(r *http.Request, w http.ResponseWriter, runID string, lastAt *time.Time, lastID *string) error {
	rows, err := api.db.QueryContext(
		r.Context(),
		`SELECT sample_id, recorded_at, recorded_by, step, name, value, metadata
		 FROM experiment_run_metric_samples
		 WHERE run_id = $1
		   AND (recorded_at > $2 OR (recorded_at = $2 AND sample_id > $3))
		 ORDER BY recorded_at ASC, sample_id ASC
		 LIMIT 2000`,
		runID,
		lastAt.UTC(),
		*lastID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			sampleID   string
			recordedAt time.Time
			recordedBy string
			step       int64
			name       string
			value      float64
			metadata   []byte
		)
		if err := rows.Scan(&sampleID, &recordedAt, &recordedBy, &step, &name, &value, &metadata); err != nil {
			return err
		}
		*lastAt = recordedAt.UTC()
		*lastID = sampleID
		if err := writeSSE(w, "metric", sampleID, experimentRunMetricSample{
			SampleID:   sampleID,
			RunID:      runID,
			RecordedAt: recordedAt.UTC(),
			RecordedBy: recordedBy,
			Step:       step,
			Name:       name,
			Value:      value,
			Metadata:   normalizeJSON(metadata),
		}); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (api *experimentsAPI) streamNewRunEvents(r *http.Request, w http.ResponseWriter, runID string, lastEventID *int64) error {
	rows, err := api.db.QueryContext(
		r.Context(),
		`SELECT event_id, occurred_at, actor, level, message, metadata
		 FROM experiment_run_events
		 WHERE run_id = $1 AND event_id > $2
		 ORDER BY event_id ASC
		 LIMIT 1000`,
		runID,
		*lastEventID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			eventID    int64
			occurredAt time.Time
			actor      string
			level      string
			message    string
			metadata   []byte
		)
		if err := rows.Scan(&eventID, &occurredAt, &actor, &level, &message, &metadata); err != nil {
			return err
		}
		*lastEventID = eventID
		if err := writeSSE(w, "event", strconv.FormatInt(eventID, 10), experimentRunEvent{
			EventID:    eventID,
			RunID:      runID,
			OccurredAt: occurredAt.UTC(),
			Actor:      actor,
			Level:      level,
			Message:    message,
			Metadata:   normalizeJSON(metadata),
		}); err != nil {
			return err
		}
	}
	return rows.Err()
}
