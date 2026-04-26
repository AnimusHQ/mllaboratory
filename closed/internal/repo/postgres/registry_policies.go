package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/integrations/registryverify"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

type RegistryPolicyStore struct {
	db DB
}

const (
	upsertRegistryPolicyQuery = `INSERT INTO registry_policies (
			project_id,
			mode,
			provider,
			created_at,
			updated_at
		) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (project_id) DO UPDATE
		SET mode = EXCLUDED.mode,
			provider = EXCLUDED.provider,
			updated_at = EXCLUDED.updated_at
		RETURNING project_id, mode, provider`
	selectRegistryPolicyQuery = `SELECT project_id, mode, provider
		FROM registry_policies
		WHERE project_id = $1`
)

func NewRegistryPolicyStore(db DB) *RegistryPolicyStore {
	if db == nil {
		return nil
	}
	return &RegistryPolicyStore{db: db}
}

func (s *RegistryPolicyStore) Get(ctx context.Context, projectID string) (registryverify.PolicyRecord, error) {
	if s == nil || s.db == nil {
		return registryverify.PolicyRecord{}, fmt.Errorf("registry policy store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return registryverify.PolicyRecord{}, fmt.Errorf("project id is required")
	}
	row := s.db.QueryRowContext(ctx, selectRegistryPolicyQuery, projectID)
	var record registryverify.PolicyRecord
	if err := row.Scan(&record.ProjectID, &record.Mode, &record.Provider); err != nil {
		if errors.Is(handleNotFound(err), repo.ErrNotFound) {
			return registryverify.PolicyRecord{}, registryverify.ErrPolicyNotFound
		}
		return registryverify.PolicyRecord{}, err
	}
	return record, nil
}

func (s *RegistryPolicyStore) Upsert(ctx context.Context, record registryverify.PolicyRecord) (registryverify.PolicyRecord, error) {
	if s == nil || s.db == nil {
		return registryverify.PolicyRecord{}, fmt.Errorf("registry policy store not initialized")
	}
	record.ProjectID = strings.TrimSpace(record.ProjectID)
	record.Mode = strings.TrimSpace(record.Mode)
	record.Provider = strings.TrimSpace(record.Provider)
	if record.ProjectID == "" || record.Mode == "" || record.Provider == "" {
		return registryverify.PolicyRecord{}, fmt.Errorf("project_id, mode, provider are required")
	}
	createdAt := time.Now().UTC()
	row := s.db.QueryRowContext(ctx, upsertRegistryPolicyQuery, record.ProjectID, record.Mode, record.Provider, createdAt, createdAt)
	if err := row.Scan(&record.ProjectID, &record.Mode, &record.Provider); err != nil {
		return registryverify.PolicyRecord{}, fmt.Errorf("upsert registry policy: %w", err)
	}
	return record, nil
}
