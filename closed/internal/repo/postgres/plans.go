package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/animus-labs/animus-go/closed/internal/repo"
)

type PlanStore struct {
	db DB
}

func NewPlanStore(db DB) *PlanStore {
	if db == nil {
		return nil
	}
	return &PlanStore{db: db}
}

func (s *PlanStore) UpsertPlan(ctx context.Context, projectID, runID string, planJSON []byte) (repo.PlanRecord, error) {
	if s == nil || s.db == nil {
		return repo.PlanRecord{}, fmt.Errorf("plan store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	if projectID == "" {
		return repo.PlanRecord{}, fmt.Errorf("project id is required")
	}
	if runID == "" {
		return repo.PlanRecord{}, fmt.Errorf("run id is required")
	}
	if len(planJSON) == 0 {
		return repo.PlanRecord{}, fmt.Errorf("plan is required")
	}

	planID := uuid.NewString()
	var record repo.PlanRecord
	err := s.db.QueryRowContext(
		ctx,
		`INSERT INTO execution_plans (
			plan_id,
			run_id,
			project_id,
			plan
		) VALUES ($1,$2,$3,$4)
		ON CONFLICT (run_id) DO NOTHING
		RETURNING plan_id, run_id, project_id, plan, created_at`,
		planID,
		runID,
		projectID,
		planJSON,
	).Scan(&record.ID, &record.RunID, &record.ProjectID, &record.Plan, &record.CreatedAt)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return repo.PlanRecord{}, fmt.Errorf("insert plan: %w", err)
		}
		existing, err := s.GetPlan(ctx, projectID, runID)
		if err != nil {
			return repo.PlanRecord{}, err
		}
		if !bytes.Equal(existing.Plan, planJSON) {
			return repo.PlanRecord{}, fmt.Errorf("execution plan already exists for run %s", runID)
		}
		return existing, nil
	}
	return record, nil
}

func (s *PlanStore) GetPlan(ctx context.Context, projectID, runID string) (repo.PlanRecord, error) {
	if s == nil || s.db == nil {
		return repo.PlanRecord{}, fmt.Errorf("plan store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	if projectID == "" {
		return repo.PlanRecord{}, fmt.Errorf("project id is required")
	}
	if runID == "" {
		return repo.PlanRecord{}, fmt.Errorf("run id is required")
	}
	var record repo.PlanRecord
	row := s.db.QueryRowContext(
		ctx,
		`SELECT plan_id, run_id, project_id, plan, created_at
		 FROM execution_plans
		 WHERE project_id = $1 AND run_id = $2`,
		projectID,
		runID,
	)
	if err := row.Scan(&record.ID, &record.RunID, &record.ProjectID, &record.Plan, &record.CreatedAt); err != nil {
		return repo.PlanRecord{}, handleNotFound(err)
	}
	return record, nil
}
