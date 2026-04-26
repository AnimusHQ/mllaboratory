package lineageevent

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Event struct {
	OccurredAt  time.Time
	Actor       string
	RequestID   string
	SubjectType string
	SubjectID   string
	Predicate   string
	ObjectType  string
	ObjectID    string
	Metadata    any
}

type QueryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (e Event) Validate() error {
	if e.OccurredAt.IsZero() {
		return errors.New("OccurredAt is required")
	}
	if strings.TrimSpace(e.Actor) == "" {
		return errors.New("Actor is required")
	}
	if strings.TrimSpace(e.SubjectType) == "" {
		return errors.New("SubjectType is required")
	}
	if strings.TrimSpace(e.SubjectID) == "" {
		return errors.New("SubjectID is required")
	}
	if strings.TrimSpace(e.Predicate) == "" {
		return errors.New("Predicate is required")
	}
	if strings.TrimSpace(e.ObjectType) == "" {
		return errors.New("ObjectType is required")
	}
	if strings.TrimSpace(e.ObjectID) == "" {
		return errors.New("ObjectID is required")
	}
	return nil
}

func Insert(ctx context.Context, q QueryRower, event Event) (int64, error) {
	if q == nil {
		return 0, errors.New("queryer is required")
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	if err := event.Validate(); err != nil {
		return 0, err
	}

	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return 0, fmt.Errorf("marshal metadata: %w", err)
	}

	integrity, err := ComputeIntegritySHA256(event, metadataJSON)
	if err != nil {
		return 0, err
	}

	var requestID sql.NullString
	if strings.TrimSpace(event.RequestID) != "" {
		requestID = sql.NullString{String: strings.TrimSpace(event.RequestID), Valid: true}
	}

	var id int64
	err = q.QueryRowContext(
		ctx,
		`INSERT INTO lineage_events (
			occurred_at,
			actor,
			request_id,
			subject_type,
			subject_id,
			predicate,
			object_type,
			object_id,
			metadata,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING event_id`,
		event.OccurredAt.UTC(),
		strings.TrimSpace(event.Actor),
		requestID,
		strings.TrimSpace(event.SubjectType),
		strings.TrimSpace(event.SubjectID),
		strings.TrimSpace(event.Predicate),
		strings.TrimSpace(event.ObjectType),
		strings.TrimSpace(event.ObjectID),
		metadataJSON,
		integrity,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert lineage event: %w", err)
	}
	return id, nil
}

func ComputeIntegritySHA256(event Event, metadataJSON []byte) (string, error) {
	type integrityInput struct {
		OccurredAt  time.Time       `json:"occurred_at"`
		Actor       string          `json:"actor"`
		RequestID   string          `json:"request_id,omitempty"`
		SubjectType string          `json:"subject_type"`
		SubjectID   string          `json:"subject_id"`
		Predicate   string          `json:"predicate"`
		ObjectType  string          `json:"object_type"`
		ObjectID    string          `json:"object_id"`
		Metadata    json.RawMessage `json:"metadata"`
	}

	in := integrityInput{
		OccurredAt:  event.OccurredAt.UTC(),
		Actor:       strings.TrimSpace(event.Actor),
		RequestID:   strings.TrimSpace(event.RequestID),
		SubjectType: strings.TrimSpace(event.SubjectType),
		SubjectID:   strings.TrimSpace(event.SubjectID),
		Predicate:   strings.TrimSpace(event.Predicate),
		ObjectType:  strings.TrimSpace(event.ObjectType),
		ObjectID:    strings.TrimSpace(event.ObjectID),
		Metadata:    metadataJSON,
	}

	blob, err := json.Marshal(in)
	if err != nil {
		return "", fmt.Errorf("marshal integrity: %w", err)
	}
	sum := sha256.Sum256(blob)
	return hex.EncodeToString(sum[:]), nil
}
