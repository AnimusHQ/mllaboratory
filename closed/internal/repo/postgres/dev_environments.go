package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
)

type DevEnvironmentStore struct {
	db DB
}

type DevEnvPolicyStore struct {
	db DB
}

type DevEnvSessionStore struct {
	db DB
}

const (
	insertDevEnvironmentQuery = `INSERT INTO dev_environments (
			dev_env_id,
			project_id,
			template_ref,
			template_version,
			template_integrity_sha256,
			repo_url,
			ref_type,
			ref_value,
			commit_pin,
			image_name,
			image_ref,
			ttl_seconds,
			state,
			created_at,
			created_by,
			last_access_at,
			expires_at,
			policy_snapshot_sha256,
			dp_job_name,
			dp_namespace,
			idempotency_key,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)
		ON CONFLICT (project_id, idempotency_key) DO NOTHING
		RETURNING dev_env_id, project_id, template_ref, template_version, template_integrity_sha256,
			repo_url, ref_type, ref_value, commit_pin, image_name, image_ref, ttl_seconds, state,
			created_at, created_by, last_access_at, expires_at, policy_snapshot_sha256, dp_job_name,
			dp_namespace, idempotency_key, integrity_sha256`
	selectDevEnvironmentByIDQuery = `SELECT dev_env_id, project_id, template_ref, template_version, template_integrity_sha256,
			repo_url, ref_type, ref_value, commit_pin, image_name, image_ref, ttl_seconds, state,
			created_at, created_by, last_access_at, expires_at, policy_snapshot_sha256, dp_job_name,
			dp_namespace, idempotency_key, integrity_sha256
		FROM dev_environments
		WHERE project_id = $1 AND dev_env_id = $2`
	selectDevEnvironmentByIdempotencyQuery = `SELECT dev_env_id, project_id, template_ref, template_version, template_integrity_sha256,
			repo_url, ref_type, ref_value, commit_pin, image_name, image_ref, ttl_seconds, state,
			created_at, created_by, last_access_at, expires_at, policy_snapshot_sha256, dp_job_name,
			dp_namespace, idempotency_key, integrity_sha256
		FROM dev_environments
		WHERE project_id = $1 AND idempotency_key = $2`
	selectDevEnvironmentListQuery = `SELECT dev_env_id, project_id, template_ref, template_version, template_integrity_sha256,
			repo_url, ref_type, ref_value, commit_pin, image_name, image_ref, ttl_seconds, state,
			created_at, created_by, last_access_at, expires_at, policy_snapshot_sha256, dp_job_name,
			dp_namespace, idempotency_key, integrity_sha256
		FROM dev_environments
		WHERE project_id = $1
			AND ($2 = '' OR state = $2)
		ORDER BY created_at DESC
		LIMIT $3`
	selectDevEnvironmentExpiredQuery = `SELECT dev_env_id, project_id, template_ref, template_version, template_integrity_sha256,
			repo_url, ref_type, ref_value, commit_pin, image_name, image_ref, ttl_seconds, state,
			created_at, created_by, last_access_at, expires_at, policy_snapshot_sha256, dp_job_name,
			dp_namespace, idempotency_key, integrity_sha256
		FROM dev_environments
		WHERE project_id = $1
			AND state IN ('provisioning', 'active')
			AND expires_at <= $2
		ORDER BY expires_at ASC
		LIMIT $3`
	updateDevEnvironmentStateQuery = `UPDATE dev_environments
		SET state = $3, dp_job_name = $4, dp_namespace = $5
		WHERE project_id = $1 AND dev_env_id = $2`
	updateDevEnvironmentAccessQuery = `UPDATE dev_environments
		SET last_access_at = $3
		WHERE project_id = $1 AND dev_env_id = $2
			AND (last_access_at IS NULL OR last_access_at < $4)`

	insertDevEnvPolicySnapshotQuery = `INSERT INTO dev_env_policy_snapshots (
			snapshot_id,
			dev_env_id,
			project_id,
			snapshot,
			snapshot_sha256,
			created_at,
			created_by,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (dev_env_id) DO NOTHING`
	selectDevEnvPolicySnapshotQuery = `SELECT snapshot
		FROM dev_env_policy_snapshots
		WHERE project_id = $1 AND dev_env_id = $2`
	selectDevEnvPolicySnapshotSHAQuery = `SELECT snapshot_sha256
		FROM dev_env_policy_snapshots
		WHERE project_id = $1 AND dev_env_id = $2`

	insertDevEnvSessionQuery = `INSERT INTO dev_env_access_sessions (
			session_id,
			dev_env_id,
			project_id,
			issued_at,
			expires_at,
			issued_by,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (session_id) DO NOTHING`
	selectDevEnvSessionByIDQuery = `SELECT session_id, dev_env_id, project_id, issued_at, expires_at, issued_by, integrity_sha256
		FROM dev_env_access_sessions
		WHERE project_id = $1 AND session_id = $2`
)

type DevEnvironmentRecord struct {
	Environment    domain.DevEnvironment
	IdempotencyKey string
}

func NewDevEnvironmentStore(db DB) *DevEnvironmentStore {
	if db == nil {
		return nil
	}
	return &DevEnvironmentStore{db: db}
}

func NewDevEnvPolicyStore(db DB) *DevEnvPolicyStore {
	if db == nil {
		return nil
	}
	return &DevEnvPolicyStore{db: db}
}

func NewDevEnvSessionStore(db DB) *DevEnvSessionStore {
	if db == nil {
		return nil
	}
	return &DevEnvSessionStore{db: db}
}

func (s *DevEnvironmentStore) Create(ctx context.Context, env domain.DevEnvironment, idempotencyKey string) (DevEnvironmentRecord, bool, error) {
	if s == nil || s.db == nil {
		return DevEnvironmentRecord{}, false, fmt.Errorf("dev environment store not initialized")
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if strings.TrimSpace(env.ProjectID) == "" {
		return DevEnvironmentRecord{}, false, fmt.Errorf("project id is required")
	}
	if idempotencyKey == "" {
		return DevEnvironmentRecord{}, false, fmt.Errorf("idempotency key is required")
	}
	if strings.TrimSpace(env.ID) == "" {
		return DevEnvironmentRecord{}, false, fmt.Errorf("dev env id is required")
	}
	if strings.TrimSpace(env.TemplateRef) == "" {
		return DevEnvironmentRecord{}, false, fmt.Errorf("template ref is required")
	}
	if env.TemplateDefinitionVersion <= 0 {
		return DevEnvironmentRecord{}, false, fmt.Errorf("template version is required")
	}
	if err := requireIntegrity(env.TemplateIntegritySHA256); err != nil {
		return DevEnvironmentRecord{}, false, err
	}
	if strings.TrimSpace(env.RepoURL) == "" {
		return DevEnvironmentRecord{}, false, fmt.Errorf("repo url is required")
	}
	if strings.TrimSpace(env.RefType) == "" {
		return DevEnvironmentRecord{}, false, fmt.Errorf("ref type is required")
	}
	if strings.TrimSpace(env.RefValue) == "" {
		return DevEnvironmentRecord{}, false, fmt.Errorf("ref value is required")
	}
	if strings.TrimSpace(env.ImageName) == "" {
		return DevEnvironmentRecord{}, false, fmt.Errorf("image name is required")
	}
	if strings.TrimSpace(env.ImageRef) == "" {
		return DevEnvironmentRecord{}, false, fmt.Errorf("image ref is required")
	}
	if env.TTLSeconds <= 0 {
		return DevEnvironmentRecord{}, false, fmt.Errorf("ttl seconds is required")
	}
	if strings.TrimSpace(env.State) == "" {
		return DevEnvironmentRecord{}, false, fmt.Errorf("state is required")
	}
	if strings.TrimSpace(env.CreatedBy) == "" {
		return DevEnvironmentRecord{}, false, fmt.Errorf("created by is required")
	}
	if env.ExpiresAt.IsZero() {
		return DevEnvironmentRecord{}, false, fmt.Errorf("expires at is required")
	}
	if err := requireIntegrity(env.IntegritySHA256); err != nil {
		return DevEnvironmentRecord{}, false, err
	}
	if err := requireIntegrity(env.PolicySnapshotSHA256); err != nil {
		return DevEnvironmentRecord{}, false, err
	}

	record := DevEnvironmentRecord{}
	createdAt := normalizeTime(env.CreatedAt)
	expiresAt := normalizeTime(env.ExpiresAt)
	var lastAccessAt sql.NullTime
	if env.LastAccessAt != nil && !env.LastAccessAt.IsZero() {
		lastAccessAt = sql.NullTime{Time: env.LastAccessAt.UTC(), Valid: true}
	}
	var commitPin sql.NullString
	err := s.db.QueryRowContext(
		ctx,
		insertDevEnvironmentQuery,
		env.ID,
		env.ProjectID,
		env.TemplateRef,
		env.TemplateDefinitionVersion,
		env.TemplateIntegritySHA256,
		strings.TrimSpace(env.RepoURL),
		strings.TrimSpace(env.RefType),
		strings.TrimSpace(env.RefValue),
		nullIfEmpty(env.CommitPin),
		env.ImageName,
		env.ImageRef,
		env.TTLSeconds,
		env.State,
		createdAt,
		env.CreatedBy,
		lastAccessAt,
		expiresAt,
		env.PolicySnapshotSHA256,
		nullIfEmpty(env.DPJobName),
		nullIfEmpty(env.DPNamespace),
		idempotencyKey,
		env.IntegritySHA256,
	).Scan(
		&record.Environment.ID,
		&record.Environment.ProjectID,
		&record.Environment.TemplateRef,
		&record.Environment.TemplateDefinitionVersion,
		&record.Environment.TemplateIntegritySHA256,
		&record.Environment.RepoURL,
		&record.Environment.RefType,
		&record.Environment.RefValue,
		&commitPin,
		&record.Environment.ImageName,
		&record.Environment.ImageRef,
		&record.Environment.TTLSeconds,
		&record.Environment.State,
		&record.Environment.CreatedAt,
		&record.Environment.CreatedBy,
		&lastAccessAt,
		&record.Environment.ExpiresAt,
		&record.Environment.PolicySnapshotSHA256,
		&record.Environment.DPJobName,
		&record.Environment.DPNamespace,
		&record.IdempotencyKey,
		&record.Environment.IntegritySHA256,
	)
	if err != nil {
		if err != sql.ErrNoRows {
			return DevEnvironmentRecord{}, false, fmt.Errorf("insert dev environment: %w", err)
		}
		existing, err := s.GetByIdempotencyKey(ctx, env.ProjectID, idempotencyKey)
		if err != nil {
			return DevEnvironmentRecord{}, false, err
		}
		return existing, false, nil
	}
	if lastAccessAt.Valid {
		t := lastAccessAt.Time.UTC()
		record.Environment.LastAccessAt = &t
	}
	if commitPin.Valid {
		record.Environment.CommitPin = commitPin.String
	}
	record.Environment.TemplateDefinitionID = record.Environment.TemplateRef
	return record, true, nil
}

func (s *DevEnvironmentStore) Get(ctx context.Context, projectID, devEnvID string) (DevEnvironmentRecord, error) {
	if s == nil || s.db == nil {
		return DevEnvironmentRecord{}, fmt.Errorf("dev environment store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	devEnvID = strings.TrimSpace(devEnvID)
	if projectID == "" || devEnvID == "" {
		return DevEnvironmentRecord{}, fmt.Errorf("project id and dev env id are required")
	}
	record := DevEnvironmentRecord{}
	var lastAccessAt sql.NullTime
	var commitPin sql.NullString
	row := s.db.QueryRowContext(ctx, selectDevEnvironmentByIDQuery, projectID, devEnvID)
	if err := row.Scan(
		&record.Environment.ID,
		&record.Environment.ProjectID,
		&record.Environment.TemplateRef,
		&record.Environment.TemplateDefinitionVersion,
		&record.Environment.TemplateIntegritySHA256,
		&record.Environment.RepoURL,
		&record.Environment.RefType,
		&record.Environment.RefValue,
		&commitPin,
		&record.Environment.ImageName,
		&record.Environment.ImageRef,
		&record.Environment.TTLSeconds,
		&record.Environment.State,
		&record.Environment.CreatedAt,
		&record.Environment.CreatedBy,
		&lastAccessAt,
		&record.Environment.ExpiresAt,
		&record.Environment.PolicySnapshotSHA256,
		&record.Environment.DPJobName,
		&record.Environment.DPNamespace,
		&record.IdempotencyKey,
		&record.Environment.IntegritySHA256,
	); err != nil {
		return DevEnvironmentRecord{}, handleNotFound(err)
	}
	if lastAccessAt.Valid {
		t := lastAccessAt.Time.UTC()
		record.Environment.LastAccessAt = &t
	}
	if commitPin.Valid {
		record.Environment.CommitPin = commitPin.String
	}
	record.Environment.TemplateDefinitionID = record.Environment.TemplateRef
	return record, nil
}

func (s *DevEnvironmentStore) GetByIdempotencyKey(ctx context.Context, projectID, idempotencyKey string) (DevEnvironmentRecord, error) {
	if s == nil || s.db == nil {
		return DevEnvironmentRecord{}, fmt.Errorf("dev environment store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if projectID == "" || idempotencyKey == "" {
		return DevEnvironmentRecord{}, fmt.Errorf("project id and idempotency key are required")
	}
	record := DevEnvironmentRecord{}
	var lastAccessAt sql.NullTime
	var commitPin sql.NullString
	row := s.db.QueryRowContext(ctx, selectDevEnvironmentByIdempotencyQuery, projectID, idempotencyKey)
	if err := row.Scan(
		&record.Environment.ID,
		&record.Environment.ProjectID,
		&record.Environment.TemplateRef,
		&record.Environment.TemplateDefinitionVersion,
		&record.Environment.TemplateIntegritySHA256,
		&record.Environment.RepoURL,
		&record.Environment.RefType,
		&record.Environment.RefValue,
		&commitPin,
		&record.Environment.ImageName,
		&record.Environment.ImageRef,
		&record.Environment.TTLSeconds,
		&record.Environment.State,
		&record.Environment.CreatedAt,
		&record.Environment.CreatedBy,
		&lastAccessAt,
		&record.Environment.ExpiresAt,
		&record.Environment.PolicySnapshotSHA256,
		&record.Environment.DPJobName,
		&record.Environment.DPNamespace,
		&record.IdempotencyKey,
		&record.Environment.IntegritySHA256,
	); err != nil {
		return DevEnvironmentRecord{}, handleNotFound(err)
	}
	if lastAccessAt.Valid {
		t := lastAccessAt.Time.UTC()
		record.Environment.LastAccessAt = &t
	}
	if commitPin.Valid {
		record.Environment.CommitPin = commitPin.String
	}
	record.Environment.TemplateDefinitionID = record.Environment.TemplateRef
	return record, nil
}

func (s *DevEnvironmentStore) List(ctx context.Context, projectID, state string, limit int) ([]DevEnvironmentRecord, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("dev environment store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project id is required")
	}
	state = strings.TrimSpace(state)
	limit = clampLimit(limit, 100, 500)
	rows, err := s.db.QueryContext(ctx, selectDevEnvironmentListQuery, projectID, state, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DevEnvironmentRecord{}
	for rows.Next() {
		record := DevEnvironmentRecord{}
		var lastAccessAt sql.NullTime
		var commitPin sql.NullString
		if err := rows.Scan(
			&record.Environment.ID,
			&record.Environment.ProjectID,
			&record.Environment.TemplateRef,
			&record.Environment.TemplateDefinitionVersion,
			&record.Environment.TemplateIntegritySHA256,
			&record.Environment.RepoURL,
			&record.Environment.RefType,
			&record.Environment.RefValue,
			&commitPin,
			&record.Environment.ImageName,
			&record.Environment.ImageRef,
			&record.Environment.TTLSeconds,
			&record.Environment.State,
			&record.Environment.CreatedAt,
			&record.Environment.CreatedBy,
			&lastAccessAt,
			&record.Environment.ExpiresAt,
			&record.Environment.PolicySnapshotSHA256,
			&record.Environment.DPJobName,
			&record.Environment.DPNamespace,
			&record.IdempotencyKey,
			&record.Environment.IntegritySHA256,
		); err != nil {
			return nil, err
		}
		if lastAccessAt.Valid {
			t := lastAccessAt.Time.UTC()
			record.Environment.LastAccessAt = &t
		}
		if commitPin.Valid {
			record.Environment.CommitPin = commitPin.String
		}
		record.Environment.TemplateDefinitionID = record.Environment.TemplateRef
		out = append(out, record)
	}
	return out, rows.Err()
}

func (s *DevEnvironmentStore) ListExpired(ctx context.Context, projectID string, now time.Time, limit int) ([]DevEnvironmentRecord, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("dev environment store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project id is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	limit = clampLimit(limit, 100, 500)
	rows, err := s.db.QueryContext(ctx, selectDevEnvironmentExpiredQuery, projectID, now.UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DevEnvironmentRecord{}
	for rows.Next() {
		record := DevEnvironmentRecord{}
		var lastAccessAt sql.NullTime
		var commitPin sql.NullString
		if err := rows.Scan(
			&record.Environment.ID,
			&record.Environment.ProjectID,
			&record.Environment.TemplateRef,
			&record.Environment.TemplateDefinitionVersion,
			&record.Environment.TemplateIntegritySHA256,
			&record.Environment.RepoURL,
			&record.Environment.RefType,
			&record.Environment.RefValue,
			&commitPin,
			&record.Environment.ImageName,
			&record.Environment.ImageRef,
			&record.Environment.TTLSeconds,
			&record.Environment.State,
			&record.Environment.CreatedAt,
			&record.Environment.CreatedBy,
			&lastAccessAt,
			&record.Environment.ExpiresAt,
			&record.Environment.PolicySnapshotSHA256,
			&record.Environment.DPJobName,
			&record.Environment.DPNamespace,
			&record.IdempotencyKey,
			&record.Environment.IntegritySHA256,
		); err != nil {
			return nil, err
		}
		if lastAccessAt.Valid {
			t := lastAccessAt.Time.UTC()
			record.Environment.LastAccessAt = &t
		}
		if commitPin.Valid {
			record.Environment.CommitPin = commitPin.String
		}
		record.Environment.TemplateDefinitionID = record.Environment.TemplateRef
		out = append(out, record)
	}
	return out, rows.Err()
}

func (s *DevEnvironmentStore) UpdateState(ctx context.Context, projectID, devEnvID, state, dpJobName, dpNamespace string) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("dev environment store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	devEnvID = strings.TrimSpace(devEnvID)
	state = strings.TrimSpace(state)
	if projectID == "" || devEnvID == "" || state == "" {
		return false, fmt.Errorf("project id, dev env id, and state are required")
	}
	res, err := s.db.ExecContext(ctx, updateDevEnvironmentStateQuery, projectID, devEnvID, state, nullIfEmpty(dpJobName), nullIfEmpty(dpNamespace))
	if err != nil {
		return false, err
	}
	updated, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return updated > 0, nil
}

func (s *DevEnvironmentStore) UpdateLastAccess(ctx context.Context, projectID, devEnvID string, accessedAt time.Time, minInterval time.Duration) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("dev environment store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	devEnvID = strings.TrimSpace(devEnvID)
	if projectID == "" || devEnvID == "" {
		return false, fmt.Errorf("project id and dev env id are required")
	}
	if accessedAt.IsZero() {
		accessedAt = time.Now().UTC()
	}
	if minInterval < 0 {
		minInterval = 0
	}
	minTime := accessedAt.Add(-minInterval).UTC()
	res, err := s.db.ExecContext(ctx, updateDevEnvironmentAccessQuery, projectID, devEnvID, accessedAt.UTC(), minTime)
	if err != nil {
		return false, err
	}
	updated, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return updated > 0, nil
}

func (s *DevEnvPolicyStore) Insert(ctx context.Context, devEnvID, projectID string, snapshot domain.PolicySnapshot, snapshotJSON []byte, createdAt time.Time, createdBy, integritySHA string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("dev env policy store not initialized")
	}
	devEnvID = strings.TrimSpace(devEnvID)
	projectID = strings.TrimSpace(projectID)
	if devEnvID == "" || projectID == "" {
		return fmt.Errorf("dev env id and project id are required")
	}
	if len(snapshotJSON) == 0 {
		return fmt.Errorf("snapshot json is required")
	}
	if strings.TrimSpace(createdBy) == "" {
		return fmt.Errorf("created by is required")
	}
	if err := requireIntegrity(integritySHA); err != nil {
		return err
	}
	if err := requireIntegrity(snapshot.SnapshotSHA256); err != nil {
		return err
	}
	createdAt = normalizeTime(createdAt)
	_, err := s.db.ExecContext(
		ctx,
		insertDevEnvPolicySnapshotQuery,
		snapshot.SnapshotSHA256,
		devEnvID,
		projectID,
		snapshotJSON,
		snapshot.SnapshotSHA256,
		createdAt,
		createdBy,
		integritySHA,
	)
	return err
}

func (s *DevEnvPolicyStore) Get(ctx context.Context, projectID, devEnvID string) (domain.PolicySnapshot, error) {
	if s == nil || s.db == nil {
		return domain.PolicySnapshot{}, fmt.Errorf("dev env policy store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	devEnvID = strings.TrimSpace(devEnvID)
	if projectID == "" || devEnvID == "" {
		return domain.PolicySnapshot{}, fmt.Errorf("project id and dev env id are required")
	}
	row := s.db.QueryRowContext(ctx, selectDevEnvPolicySnapshotQuery, projectID, devEnvID)
	var snapshotJSON []byte
	if err := row.Scan(&snapshotJSON); err != nil {
		return domain.PolicySnapshot{}, handleNotFound(err)
	}
	var snapshot domain.PolicySnapshot
	if err := json.Unmarshal(snapshotJSON, &snapshot); err != nil {
		return domain.PolicySnapshot{}, err
	}
	return snapshot, nil
}

func (s *DevEnvPolicyStore) GetSHA(ctx context.Context, projectID, devEnvID string) (string, error) {
	if s == nil || s.db == nil {
		return "", fmt.Errorf("dev env policy store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	devEnvID = strings.TrimSpace(devEnvID)
	if projectID == "" || devEnvID == "" {
		return "", fmt.Errorf("project id and dev env id are required")
	}
	row := s.db.QueryRowContext(ctx, selectDevEnvPolicySnapshotSHAQuery, projectID, devEnvID)
	var snapshotSHA sql.NullString
	if err := row.Scan(&snapshotSHA); err != nil {
		return "", handleNotFound(err)
	}
	return strings.TrimSpace(snapshotSHA.String), nil
}

func (s *DevEnvSessionStore) Insert(ctx context.Context, session domain.DevEnvAccessSession, integritySHA string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("dev env session store not initialized")
	}
	sessionID := strings.TrimSpace(session.SessionID)
	projectID := strings.TrimSpace(session.ProjectID)
	devEnvID := strings.TrimSpace(session.DevEnvID)
	if sessionID == "" || projectID == "" || devEnvID == "" {
		return fmt.Errorf("session id, project id, and dev env id are required")
	}
	if strings.TrimSpace(session.IssuedBy) == "" {
		return fmt.Errorf("issued by is required")
	}
	if session.ExpiresAt.IsZero() {
		return fmt.Errorf("expires at is required")
	}
	if err := requireIntegrity(integritySHA); err != nil {
		return err
	}
	issuedAt := normalizeTime(session.IssuedAt)
	_, err := s.db.ExecContext(
		ctx,
		insertDevEnvSessionQuery,
		sessionID,
		devEnvID,
		projectID,
		issuedAt,
		session.ExpiresAt.UTC(),
		session.IssuedBy,
		integritySHA,
	)
	return err
}

func (s *DevEnvSessionStore) GetBySessionID(ctx context.Context, projectID, sessionID string) (domain.DevEnvAccessSession, error) {
	if s == nil || s.db == nil {
		return domain.DevEnvAccessSession{}, fmt.Errorf("dev env session store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	sessionID = strings.TrimSpace(sessionID)
	if projectID == "" || sessionID == "" {
		return domain.DevEnvAccessSession{}, fmt.Errorf("project id and session id are required")
	}
	row := s.db.QueryRowContext(ctx, selectDevEnvSessionByIDQuery, projectID, sessionID)
	var session domain.DevEnvAccessSession
	if err := row.Scan(
		&session.SessionID,
		&session.DevEnvID,
		&session.ProjectID,
		&session.IssuedAt,
		&session.ExpiresAt,
		&session.IssuedBy,
		new(sql.NullString),
	); err != nil {
		return domain.DevEnvAccessSession{}, handleNotFound(err)
	}
	return session, nil
}

func clampLimit(value, fallback, max int) int {
	if value <= 0 {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}
