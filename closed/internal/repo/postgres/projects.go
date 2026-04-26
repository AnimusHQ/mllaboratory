package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

type ProjectStore struct {
	db DB
}

func NewProjectStore(db DB) *ProjectStore {
	if db == nil {
		return nil
	}
	return &ProjectStore{db: db}
}

func (s *ProjectStore) Create(ctx context.Context, project domain.Project) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("project store not initialized")
	}
	if err := project.Validate(); err != nil {
		return err
	}
	metadataJSON, err := encodeMetadata(project.Metadata)
	if err != nil {
		return fmt.Errorf("encode metadata: %w", err)
	}
	createdAt := normalizeTime(project.CreatedAt)
	if err := requireIntegrity(project.IntegritySHA256); err != nil {
		return err
	}
	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO projects (
			project_id,
			name,
			description,
			metadata,
			created_at,
			created_by,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		strings.TrimSpace(project.ID),
		strings.TrimSpace(project.Name),
		strings.TrimSpace(project.Description),
		metadataJSON,
		createdAt,
		strings.TrimSpace(project.CreatedBy),
		strings.TrimSpace(project.IntegritySHA256),
	)
	if err != nil {
		return fmt.Errorf("insert project: %w", err)
	}
	return nil
}

func (s *ProjectStore) Get(ctx context.Context, id string) (domain.Project, error) {
	if s == nil || s.db == nil {
		return domain.Project{}, fmt.Errorf("project store not initialized")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Project{}, fmt.Errorf("project id is required")
	}
	var (
		project      domain.Project
		metadataJSON []byte
	)
	row := s.db.QueryRowContext(
		ctx,
		`SELECT project_id, name, description, metadata, created_at, created_by, integrity_sha256
		 FROM projects
		 WHERE project_id = $1`,
		id,
	)
	if err := row.Scan(&project.ID, &project.Name, &project.Description, &metadataJSON, &project.CreatedAt, &project.CreatedBy, &project.IntegritySHA256); err != nil {
		return domain.Project{}, handleNotFound(err)
	}
	meta, err := decodeMetadata(metadataJSON)
	if err != nil {
		return domain.Project{}, fmt.Errorf("decode metadata: %w", err)
	}
	project.Metadata = meta
	return project, nil
}

func (s *ProjectStore) List(ctx context.Context, filter repo.ProjectFilter) ([]domain.Project, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("project store not initialized")
	}
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 2)

	if strings.TrimSpace(filter.Name) != "" {
		args = append(args, strings.TrimSpace(filter.Name))
		clauses = append(clauses, fmt.Sprintf("name = $%d", len(args)))
	}
	if strings.TrimSpace(filter.CreatedBy) != "" {
		args = append(args, strings.TrimSpace(filter.CreatedBy))
		clauses = append(clauses, fmt.Sprintf("created_by = $%d", len(args)))
	}

	query := `SELECT project_id, name, description, metadata, created_at, created_by, integrity_sha256 FROM projects`
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY created_at DESC"
	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	projects := make([]domain.Project, 0)
	for rows.Next() {
		var p domain.Project
		var metadataJSON []byte
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &metadataJSON, &p.CreatedAt, &p.CreatedBy, &p.IntegritySHA256); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		meta, err := decodeMetadata(metadataJSON)
		if err != nil {
			return nil, fmt.Errorf("decode metadata: %w", err)
		}
		p.Metadata = meta
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	return projects, nil
}
