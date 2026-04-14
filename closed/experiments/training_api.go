package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/platform/redaction"
	"github.com/google/uuid"
)

type executeExperimentRunRequest struct {
	ExperimentID     string         `json:"experiment_id"`
	DatasetVersionID string         `json:"dataset_version_id"`
	ImageRef         string         `json:"image_ref"`
	GitRepo          string         `json:"git_repo,omitempty"`
	GitCommit        string         `json:"git_commit,omitempty"`
	GitRef           string         `json:"git_ref,omitempty"`
	Params           map[string]any `json:"params,omitempty"`
	Resources        map[string]any `json:"resources,omitempty"`
}

func (api *experimentsAPI) handleExecuteExperimentRun(w http.ResponseWriter, r *http.Request) {
	api.writeError(w, r, http.StatusNotImplemented, "training_executor_disabled")
}

type ingestExperimentRunMetricsRequest struct {
	Step    int64          `json:"step"`
	Metrics map[string]any `json:"metrics"`
	Meta    map[string]any `json:"metadata,omitempty"`
}

func (api *experimentsAPI) handleIngestExperimentRunMetrics(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	runID := strings.TrimSpace(r.PathValue("run_id"))
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}

	var req ingestExperimentRunMetricsRequest
	if err := decodeJSON(r, &req); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}
	if req.Step < 0 {
		api.writeError(w, r, http.StatusBadRequest, "invalid_step")
		return
	}
	if len(req.Metrics) == 0 {
		api.writeError(w, r, http.StatusBadRequest, "metrics_required")
		return
	}

	metrics := make(map[string]float64, len(req.Metrics))
	for k, v := range req.Metrics {
		name := strings.TrimSpace(k)
		if name == "" {
			api.writeError(w, r, http.StatusBadRequest, "invalid_metric_name")
			return
		}
		switch n := v.(type) {
		case float64:
			metrics[name] = n
		case int:
			metrics[name] = float64(n)
		case int64:
			metrics[name] = float64(n)
		default:
			api.writeError(w, r, http.StatusBadRequest, "invalid_metric_value")
			return
		}
	}

	metadataMap := req.Meta
	if metadataMap == nil {
		metadataMap = map[string]any{}
	}
	metadataJSON, err := json.Marshal(metadataMap)
	if err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_metadata")
		return
	}

	tx, err := api.db.BeginTx(r.Context(), nil)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback() }()

	var one int
	if err := tx.QueryRowContext(r.Context(), `SELECT 1 FROM experiment_runs WHERE run_id = $1`, runID).Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	now := time.Now().UTC()
	inserted := 0
	names := make([]string, 0, len(metrics))
	for name := range metrics {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		value := metrics[name]
		sampleID := uuid.NewString()

		type integrityInput struct {
			SampleID    string          `json:"sample_id"`
			RunID       string          `json:"run_id"`
			RecordedAt  time.Time       `json:"recorded_at"`
			RecordedBy  string          `json:"recorded_by"`
			Step        int64           `json:"step"`
			Name        string          `json:"name"`
			Value       float64         `json:"value"`
			Metadata    json.RawMessage `json:"metadata"`
			RequestID   string          `json:"request_id,omitempty"`
			UserAgent   string          `json:"user_agent,omitempty"`
			RemoteAddr  string          `json:"remote_addr,omitempty"`
			DatapilotID string          `json:"datapilot_id,omitempty"`
		}
		integrity, err := integritySHA256(integrityInput{
			SampleID:   sampleID,
			RunID:      runID,
			RecordedAt: now,
			RecordedBy: identity.Subject,
			Step:       req.Step,
			Name:       name,
			Value:      value,
			Metadata:   metadataJSON,
			RequestID:  r.Header.Get("X-Request-Id"),
			UserAgent:  r.UserAgent(),
			RemoteAddr: r.RemoteAddr,
		})
		if err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}

		res, err := tx.ExecContext(
			r.Context(),
			`INSERT INTO experiment_run_metric_samples (
				sample_id,
				run_id,
				recorded_at,
				recorded_by,
				step,
				name,
				value,
				metadata,
				integrity_sha256
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (run_id, name, step) DO NOTHING`,
			sampleID,
			runID,
			now,
			identity.Subject,
			req.Step,
			name,
			value,
			metadataJSON,
			integrity,
		)
		if err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		affected, _ := res.RowsAffected()
		if affected > 0 {
			inserted++
		}
	}

	if err := tx.Commit(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	status := http.StatusCreated
	if inserted == 0 {
		status = http.StatusOK
	}
	api.writeJSON(w, status, map[string]any{
		"run_id":     runID,
		"step":       req.Step,
		"inserted":   inserted,
		"received":   len(metrics),
		"request_id": r.Header.Get("X-Request-Id"),
	})
}

type experimentRunMetricSample struct {
	SampleID   string          `json:"sample_id"`
	RunID      string          `json:"run_id"`
	RecordedAt time.Time       `json:"recorded_at"`
	RecordedBy string          `json:"recorded_by"`
	Step       int64           `json:"step"`
	Name       string          `json:"name"`
	Value      float64         `json:"value"`
	Metadata   json.RawMessage `json:"metadata"`
}

func (api *experimentsAPI) handleListExperimentRunMetrics(w http.ResponseWriter, r *http.Request) {
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

	limit := clampInt(parseIntQuery(r, "limit", 200), 1, 1000)
	nameFilter := strings.TrimSpace(r.URL.Query().Get("name"))

	var (
		rows *sql.Rows
		err  error
	)
	if nameFilter != "" {
		rows, err = api.db.QueryContext(
			r.Context(),
			`SELECT sample_id, recorded_at, recorded_by, step, name, value, metadata
			 FROM experiment_run_metric_samples
			 WHERE run_id = $1 AND name = $2
			 ORDER BY step DESC
			 LIMIT $3`,
			runID,
			nameFilter,
			limit,
		)
	} else {
		rows, err = api.db.QueryContext(
			r.Context(),
			`SELECT DISTINCT ON (name) sample_id, recorded_at, recorded_by, step, name, value, metadata
			 FROM experiment_run_metric_samples
			 WHERE run_id = $1
			 ORDER BY name, step DESC
			 LIMIT $2`,
			runID,
			limit,
		)
	}
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer rows.Close()

	out := make([]experimentRunMetricSample, 0, limit)
	for rows.Next() {
		var (
			sample   experimentRunMetricSample
			metadata []byte
		)
		if err := rows.Scan(&sample.SampleID, &sample.RecordedAt, &sample.RecordedBy, &sample.Step, &sample.Name, &sample.Value, &metadata); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		sample.RunID = runID
		sample.Metadata = normalizeJSON(metadata)
		out = append(out, sample)
	}
	if err := rows.Err(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if nameFilter != "" {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}

	api.writeJSON(w, http.StatusOK, map[string]any{
		"run_id":  runID,
		"name":    nameFilter,
		"samples": out,
	})
}

type createExperimentRunEventRequest struct {
	OccurredAt *time.Time     `json:"occurred_at,omitempty"`
	Level      string         `json:"level,omitempty"`
	Message    string         `json:"message"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type experimentRunEvent struct {
	EventID    int64           `json:"event_id"`
	RunID      string          `json:"run_id"`
	OccurredAt time.Time       `json:"occurred_at"`
	Actor      string          `json:"actor"`
	Level      string          `json:"level"`
	Message    string          `json:"message"`
	Metadata   json.RawMessage `json:"metadata"`
}

func (api *experimentsAPI) handleCreateExperimentRunEvent(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	runID := strings.TrimSpace(r.PathValue("run_id"))
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}

	var req createExperimentRunEventRequest
	if err := decodeJSON(r, &req); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}

	message := redaction.RedactString(strings.TrimSpace(req.Message))
	if message == "" {
		api.writeError(w, r, http.StatusBadRequest, "message_required")
		return
	}
	level := strings.ToLower(strings.TrimSpace(req.Level))
	if level == "" {
		level = "info"
	}
	switch level {
	case "debug", "info", "warn", "error":
	default:
		api.writeError(w, r, http.StatusBadRequest, "invalid_level")
		return
	}

	occurredAt := time.Now().UTC()
	if req.OccurredAt != nil && !req.OccurredAt.IsZero() {
		occurredAt = req.OccurredAt.UTC()
	}

	metaMap := req.Metadata
	if metaMap == nil {
		metaMap = map[string]any{}
	}
	metaMap = redaction.RedactMetadata(metaMap)
	metaJSON, err := json.Marshal(metaMap)
	if err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_metadata")
		return
	}

	type integrityInput struct {
		RunID       string          `json:"run_id"`
		OccurredAt  time.Time       `json:"occurred_at"`
		Actor       string          `json:"actor"`
		Level       string          `json:"level"`
		Message     string          `json:"message"`
		Metadata    json.RawMessage `json:"metadata"`
		RequestID   string          `json:"request_id,omitempty"`
		UserAgent   string          `json:"user_agent,omitempty"`
		RemoteAddr  string          `json:"remote_addr,omitempty"`
		DatapilotID string          `json:"datapilot_id,omitempty"`
	}
	integrity, err := integritySHA256(integrityInput{
		RunID:      runID,
		OccurredAt: occurredAt,
		Actor:      identity.Subject,
		Level:      level,
		Message:    message,
		Metadata:   metaJSON,
		RequestID:  r.Header.Get("X-Request-Id"),
		UserAgent:  r.UserAgent(),
		RemoteAddr: r.RemoteAddr,
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	var eventID int64
	err = api.db.QueryRowContext(
		r.Context(),
		`INSERT INTO experiment_run_events (
			run_id,
			occurred_at,
			actor,
			level,
			message,
			metadata,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING event_id`,
		runID,
		occurredAt,
		identity.Subject,
		level,
		message,
		metaJSON,
		integrity,
	).Scan(&eventID)
	if err != nil {
		if isForeignKeyViolation(err) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	w.Header().Set("Location", "/experiment-runs/"+runID+"/events/"+strconv.FormatInt(eventID, 10))
	api.writeJSON(w, http.StatusCreated, experimentRunEvent{
		EventID:    eventID,
		RunID:      runID,
		OccurredAt: occurredAt,
		Actor:      identity.Subject,
		Level:      level,
		Message:    message,
		Metadata:   metaJSON,
	})
}

func (api *experimentsAPI) handleListExperimentRunEvents(w http.ResponseWriter, r *http.Request) {
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

	limit := clampInt(parseIntQuery(r, "limit", 200), 1, 1000)
	beforeRaw := strings.TrimSpace(r.URL.Query().Get("before_event_id"))
	var beforeID int64
	if beforeRaw != "" {
		parsed, err := strconv.ParseInt(beforeRaw, 10, 64)
		if err != nil || parsed < 0 {
			api.writeError(w, r, http.StatusBadRequest, "invalid_before_event_id")
			return
		}
		beforeID = parsed
	}

	args := []any{runID}
	query := `SELECT event_id, occurred_at, actor, level, message, metadata
		FROM experiment_run_events
		WHERE run_id = $1`
	if beforeID > 0 {
		args = append(args, beforeID)
		query += " AND event_id < $" + strconv.Itoa(len(args))
	}
	args = append(args, limit)
	query += " ORDER BY event_id DESC LIMIT $" + strconv.Itoa(len(args))

	rows, err := api.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer rows.Close()

	out := make([]experimentRunEvent, 0, limit)
	for rows.Next() {
		var (
			ev          experimentRunEvent
			metadataRaw []byte
		)
		if err := rows.Scan(&ev.EventID, &ev.OccurredAt, &ev.Actor, &ev.Level, &ev.Message, &metadataRaw); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		ev.RunID = runID
		ev.Metadata = normalizeJSON(metadataRaw)
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	resp := map[string]any{
		"run_id": runID,
		"events": out,
	}
	if len(out) > 0 {
		resp["next_before_event_id"] = out[len(out)-1].EventID
	}
	api.writeJSON(w, http.StatusOK, resp)
}

func (api *experimentsAPI) insertRunStateEvent(ctx context.Context, tx *sql.Tx, runID string, status string, observedAt time.Time, details map[string]any) (bool, error) {
	if tx == nil {
		return false, errors.New("tx is required")
	}
	runID = strings.TrimSpace(runID)
	status = strings.ToLower(strings.TrimSpace(status))
	if runID == "" || status == "" {
		return false, errors.New("run_id and status are required")
	}
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	if details == nil {
		details = map[string]any{}
	}
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return false, err
	}

	stateID := uuid.NewString()
	type integrityInput struct {
		StateID     string          `json:"state_id"`
		RunID       string          `json:"run_id"`
		Status      string          `json:"status"`
		ObservedAt  time.Time       `json:"observed_at"`
		Details     json.RawMessage `json:"details"`
		DatapilotID string          `json:"datapilot_id,omitempty"`
	}
	integrity, err := integritySHA256(integrityInput{
		StateID:    stateID,
		RunID:      runID,
		Status:     status,
		ObservedAt: observedAt,
		Details:    detailsJSON,
	})
	if err != nil {
		return false, err
	}

	res, err := tx.ExecContext(ctx,
		`INSERT INTO experiment_run_state_events (state_id, run_id, status, observed_at, details, integrity_sha256)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 ON CONFLICT (run_id, status) DO NOTHING`,
		stateID,
		runID,
		status,
		observedAt,
		detailsJSON,
		integrity,
	)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func (api *experimentsAPI) insertRunEvent(ctx context.Context, tx *sql.Tx, runID string, actor string, level string, message string, metadata map[string]any) error {
	if tx == nil {
		return errors.New("tx is required")
	}
	runID = strings.TrimSpace(runID)
	actor = strings.TrimSpace(actor)
	level = strings.ToLower(strings.TrimSpace(level))
	message = strings.TrimSpace(message)
	if runID == "" || actor == "" || level == "" || message == "" {
		return errors.New("run_id, actor, level, and message are required")
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	type integrityInput struct {
		RunID      string          `json:"run_id"`
		OccurredAt time.Time       `json:"occurred_at"`
		Actor      string          `json:"actor"`
		Level      string          `json:"level"`
		Message    string          `json:"message"`
		Metadata   json.RawMessage `json:"metadata"`
	}

	occurredAt := time.Now().UTC()
	integrity, err := integritySHA256(integrityInput{
		RunID:      runID,
		OccurredAt: occurredAt,
		Actor:      actor,
		Level:      level,
		Message:    message,
		Metadata:   metadataJSON,
	})
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO experiment_run_events (
			run_id,
			occurred_at,
			actor,
			level,
			message,
			metadata,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		runID,
		occurredAt,
		actor,
		level,
		message,
		metadataJSON,
		integrity,
	)
	return err
}
