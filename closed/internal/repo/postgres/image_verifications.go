package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/integrations/registryverify"
)

type ImageVerificationStore struct {
	db DB
}

const (
	upsertImageVerificationQuery = `INSERT INTO image_verifications (
			project_id,
			image_digest_ref,
			policy_mode,
			provider,
			status,
			signed,
			verified,
			failure_reason,
			details_jsonb,
			created_at,
			verified_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (project_id, image_digest_ref, policy_mode, provider) DO UPDATE
		SET status = EXCLUDED.status,
			signed = EXCLUDED.signed,
			verified = EXCLUDED.verified,
			failure_reason = EXCLUDED.failure_reason,
			details_jsonb = EXCLUDED.details_jsonb,
			verified_at = EXCLUDED.verified_at
		RETURNING id, project_id, image_digest_ref, policy_mode, provider, status, signed, verified, failure_reason, details_jsonb, created_at, verified_at`
	selectLatestImageVerificationQuery = `SELECT id, project_id, image_digest_ref, policy_mode, provider, status, signed, verified, failure_reason, details_jsonb, created_at, verified_at
		FROM image_verifications
		WHERE project_id = $1 AND image_digest_ref = $2
		ORDER BY verified_at DESC NULLS LAST, created_at DESC
		LIMIT 1`
	selectImageVerificationsListQuery = `SELECT id, project_id, image_digest_ref, policy_mode, provider, status, signed, verified, failure_reason, details_jsonb, created_at, verified_at
		FROM image_verifications
		WHERE project_id = $1
			AND ($2 = '' OR image_digest_ref = $2)
		ORDER BY verified_at DESC NULLS LAST, created_at DESC
		LIMIT $3`
)

func NewImageVerificationStore(db DB) *ImageVerificationStore {
	if db == nil {
		return nil
	}
	return &ImageVerificationStore{db: db}
}

type ImageVerificationFilter struct {
	ProjectID      string
	ImageDigestRef string
	Limit          int
}

func (s *ImageVerificationStore) Upsert(ctx context.Context, record registryverify.Record) (registryverify.Record, error) {
	if s == nil || s.db == nil {
		return registryverify.Record{}, fmt.Errorf("image verification store not initialized")
	}
	record.ProjectID = strings.TrimSpace(record.ProjectID)
	record.ImageDigestRef = strings.TrimSpace(record.ImageDigestRef)
	record.PolicyMode = strings.TrimSpace(record.PolicyMode)
	record.Provider = strings.TrimSpace(record.Provider)
	if record.ProjectID == "" || record.ImageDigestRef == "" || record.PolicyMode == "" || record.Provider == "" {
		return registryverify.Record{}, fmt.Errorf("project_id, image_digest_ref, policy_mode, provider are required")
	}
	if strings.TrimSpace(string(record.Status)) == "" {
		return registryverify.Record{}, fmt.Errorf("status is required")
	}
	if record.Details == nil {
		record.Details = json.RawMessage(`{}`)
	}
	createdAt := normalizeTime(record.CreatedAt)
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	var verifiedAt sql.NullTime
	if record.VerifiedAt != nil && !record.VerifiedAt.IsZero() {
		verifiedAt = sql.NullTime{Time: record.VerifiedAt.UTC(), Valid: true}
	}
	row := s.db.QueryRowContext(
		ctx,
		upsertImageVerificationQuery,
		record.ProjectID,
		record.ImageDigestRef,
		record.PolicyMode,
		record.Provider,
		string(record.Status),
		record.Signed,
		record.Verified,
		nullIfEmpty(record.FailureReason),
		record.Details,
		createdAt,
		verifiedAt,
	)
	return scanImageVerification(row)
}

func (s *ImageVerificationStore) GetLatestByImage(ctx context.Context, projectID, imageDigestRef string) (registryverify.Record, error) {
	if s == nil || s.db == nil {
		return registryverify.Record{}, fmt.Errorf("image verification store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	imageDigestRef = strings.TrimSpace(imageDigestRef)
	if projectID == "" || imageDigestRef == "" {
		return registryverify.Record{}, fmt.Errorf("project_id and image_digest_ref are required")
	}
	row := s.db.QueryRowContext(ctx, selectLatestImageVerificationQuery, projectID, imageDigestRef)
	return scanImageVerification(row)
}

func (s *ImageVerificationStore) List(ctx context.Context, filter ImageVerificationFilter) ([]registryverify.Record, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("image verification store not initialized")
	}
	filter.ProjectID = strings.TrimSpace(filter.ProjectID)
	if filter.ProjectID == "" {
		return nil, fmt.Errorf("project id is required")
	}
	filter.ImageDigestRef = strings.TrimSpace(filter.ImageDigestRef)
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, selectImageVerificationsListQuery, filter.ProjectID, filter.ImageDigestRef, limit)
	if err != nil {
		return nil, fmt.Errorf("list image verifications: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]registryverify.Record, 0)
	for rows.Next() {
		record, err := scanImageVerification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list image verifications: %w", err)
	}
	return out, nil
}

func scanImageVerification(scanner interface {
	Scan(dest ...any) error
}) (registryverify.Record, error) {
	var record registryverify.Record
	var status string
	var failure sql.NullString
	var details json.RawMessage
	var verifiedAt sql.NullTime
	if err := scanner.Scan(
		&record.ID,
		&record.ProjectID,
		&record.ImageDigestRef,
		&record.PolicyMode,
		&record.Provider,
		&status,
		&record.Signed,
		&record.Verified,
		&failure,
		&details,
		&record.CreatedAt,
		&verifiedAt,
	); err != nil {
		return registryverify.Record{}, handleNotFound(err)
	}
	record.Status = registryverify.Status(status)
	record.FailureReason = strings.TrimSpace(failure.String)
	if len(details) == 0 {
		record.Details = json.RawMessage(`{}`)
	} else {
		record.Details = details
	}
	if verifiedAt.Valid {
		value := verifiedAt.Time
		record.VerifiedAt = &value
	}
	return record, nil
}
