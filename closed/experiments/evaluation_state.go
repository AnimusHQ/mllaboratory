package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (api *experimentsAPI) insertEvaluationStateEvent(ctx context.Context, tx *sql.Tx, evaluationID string, status string, observedAt time.Time, details map[string]any) (bool, error) {
	if tx == nil {
		return false, errors.New("tx is required")
	}
	evaluationID = strings.TrimSpace(evaluationID)
	status = strings.ToLower(strings.TrimSpace(status))
	if evaluationID == "" || status == "" {
		return false, errors.New("evaluation_id and status are required")
	}
	if _, ok := allowedRunStatuses[status]; !ok {
		return false, errors.New("invalid status")
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
		StateID      string          `json:"state_id"`
		EvaluationID string          `json:"evaluation_id"`
		Status       string          `json:"status"`
		ObservedAt   time.Time       `json:"observed_at"`
		Details      json.RawMessage `json:"details"`
	}
	integrity, err := integritySHA256(integrityInput{
		StateID:      stateID,
		EvaluationID: evaluationID,
		Status:       status,
		ObservedAt:   observedAt,
		Details:      detailsJSON,
	})
	if err != nil {
		return false, err
	}

	res, err := tx.ExecContext(
		ctx,
		`INSERT INTO experiment_run_evaluation_state_events (state_id, evaluation_id, status, observed_at, details, integrity_sha256)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 ON CONFLICT (evaluation_id, status) DO NOTHING`,
		stateID,
		evaluationID,
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
