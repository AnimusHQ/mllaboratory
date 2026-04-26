package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/repo"
	"github.com/google/uuid"
)

type RoleBindingStore struct {
	db DB
}

const (
	insertRoleBindingQuery = `INSERT INTO project_role_bindings (
		binding_id,
		project_id,
		subject_type,
		subject,
		role,
		created_at,
		created_by,
		updated_at,
		updated_by,
		integrity_sha256
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`

	selectRoleBindingByKeyQuery = `SELECT binding_id, project_id, subject_type, subject, role,
		created_at, created_by, updated_at, updated_by, integrity_sha256
		FROM project_role_bindings
		WHERE project_id = $1 AND subject_type = $2 AND subject = $3`

	selectRoleBindingByIDQuery = `SELECT binding_id, project_id, subject_type, subject, role,
		created_at, created_by, updated_at, updated_by, integrity_sha256
		FROM project_role_bindings
		WHERE project_id = $1 AND binding_id = $2`

	listRoleBindingsByProjectQuery = `SELECT binding_id, project_id, subject_type, subject, role,
		created_at, created_by, updated_at, updated_by, integrity_sha256
		FROM project_role_bindings
		WHERE project_id = $1
		ORDER BY subject_type ASC, subject ASC`
)

func NewRoleBindingStore(db DB) *RoleBindingStore {
	if db == nil {
		return nil
	}
	return &RoleBindingStore{db: db}
}

func (s *RoleBindingStore) Upsert(ctx context.Context, record repo.RoleBindingRecord) (repo.RoleBindingRecord, bool, error) {
	if s == nil || s.db == nil {
		return repo.RoleBindingRecord{}, false, fmt.Errorf("role binding store not initialized")
	}
	record.ProjectID = strings.TrimSpace(record.ProjectID)
	record.SubjectType = strings.ToLower(strings.TrimSpace(record.SubjectType))
	record.Subject = strings.TrimSpace(record.Subject)
	record.Role = strings.ToLower(strings.TrimSpace(record.Role))
	record.CreatedBy = strings.TrimSpace(record.CreatedBy)
	record.UpdatedBy = strings.TrimSpace(record.UpdatedBy)
	record.IntegritySHA = strings.TrimSpace(record.IntegritySHA)

	if record.ProjectID == "" || record.SubjectType == "" || record.Subject == "" || record.Role == "" {
		return repo.RoleBindingRecord{}, false, fmt.Errorf("project_id, subject_type, subject, role are required")
	}
	if record.CreatedBy == "" {
		return repo.RoleBindingRecord{}, false, fmt.Errorf("created_by is required")
	}
	if record.UpdatedBy == "" {
		record.UpdatedBy = record.CreatedBy
	}
	if record.IntegritySHA == "" {
		return repo.RoleBindingRecord{}, false, fmt.Errorf("integrity sha256 is required")
	}

	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = record.CreatedAt
	}

	existing, err := s.getByKey(ctx, record.ProjectID, record.SubjectType, record.Subject)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return repo.RoleBindingRecord{}, false, err
	}
	if err == nil {
		updated, err := s.updateBinding(ctx, existing.BindingID, record)
		return updated, false, err
	}

	if strings.TrimSpace(record.BindingID) == "" {
		record.BindingID = uuid.NewString()
	}
	_, err = s.db.ExecContext(
		ctx,
		insertRoleBindingQuery,
		record.BindingID,
		record.ProjectID,
		record.SubjectType,
		record.Subject,
		record.Role,
		record.CreatedAt.UTC(),
		record.CreatedBy,
		record.UpdatedAt.UTC(),
		record.UpdatedBy,
		record.IntegritySHA,
	)
	if err != nil {
		return repo.RoleBindingRecord{}, false, fmt.Errorf("insert role binding: %w", err)
	}
	created, err := s.GetByID(ctx, record.ProjectID, record.BindingID)
	return created, true, err
}

func (s *RoleBindingStore) ListByProject(ctx context.Context, projectID string) ([]repo.RoleBindingRecord, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("role binding store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	rows, err := s.db.QueryContext(ctx, listRoleBindingsByProjectQuery, projectID)
	if err != nil {
		return nil, fmt.Errorf("list role bindings: %w", err)
	}
	defer rows.Close()

	out := make([]repo.RoleBindingRecord, 0)
	for rows.Next() {
		rec, err := scanRoleBinding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list role bindings: %w", err)
	}
	return out, nil
}

func (s *RoleBindingStore) ListBySubjects(ctx context.Context, projectID string, subjects []repo.RoleBindingSubject) ([]repo.RoleBindingRecord, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("role binding store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	if len(subjects) == 0 {
		return nil, nil
	}

	clauses := make([]string, 0, len(subjects))
	args := make([]any, 0, len(subjects)*2+1)
	args = append(args, projectID)

	for _, subj := range subjects {
		typeVal := strings.ToLower(strings.TrimSpace(subj.Type))
		value := strings.TrimSpace(subj.Value)
		if typeVal == "" || value == "" {
			continue
		}
		args = append(args, typeVal, value)
		idx := len(args) - 1
		clauses = append(clauses, fmt.Sprintf("(subject_type = $%d AND subject = $%d)", idx-1, idx))
	}
	if len(clauses) == 0 {
		return nil, nil
	}

	query := `SELECT binding_id, project_id, subject_type, subject, role,
		created_at, created_by, updated_at, updated_by, integrity_sha256
		FROM project_role_bindings
		WHERE project_id = $1 AND (` + strings.Join(clauses, " OR ") + `)
		ORDER BY role DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list role bindings by subjects: %w", err)
	}
	defer rows.Close()

	out := make([]repo.RoleBindingRecord, 0)
	for rows.Next() {
		rec, err := scanRoleBinding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list role bindings by subjects: %w", err)
	}
	return out, nil
}

func (s *RoleBindingStore) Delete(ctx context.Context, projectID, bindingID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("role binding store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	bindingID = strings.TrimSpace(bindingID)
	if projectID == "" || bindingID == "" {
		return fmt.Errorf("project_id and binding_id are required")
	}
	res, err := s.db.ExecContext(
		ctx,
		`DELETE FROM project_role_bindings WHERE project_id = $1 AND binding_id = $2`,
		projectID,
		bindingID,
	)
	if err != nil {
		return fmt.Errorf("delete role binding: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete role binding: %w", err)
	}
	if rows == 0 {
		return repo.ErrNotFound
	}
	return nil
}

func (s *RoleBindingStore) GetByID(ctx context.Context, projectID, bindingID string) (repo.RoleBindingRecord, error) {
	if s == nil || s.db == nil {
		return repo.RoleBindingRecord{}, fmt.Errorf("role binding store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	bindingID = strings.TrimSpace(bindingID)
	if projectID == "" || bindingID == "" {
		return repo.RoleBindingRecord{}, fmt.Errorf("project_id and binding_id are required")
	}
	row := s.db.QueryRowContext(ctx, selectRoleBindingByIDQuery, projectID, bindingID)
	rec, err := scanRoleBinding(row)
	if err != nil {
		return repo.RoleBindingRecord{}, err
	}
	return rec, nil
}

func (s *RoleBindingStore) getByKey(ctx context.Context, projectID, subjectType, subject string) (repo.RoleBindingRecord, error) {
	row := s.db.QueryRowContext(ctx, selectRoleBindingByKeyQuery, projectID, subjectType, subject)
	rec, err := scanRoleBinding(row)
	if err != nil {
		return repo.RoleBindingRecord{}, err
	}
	return rec, nil
}

func (s *RoleBindingStore) updateBinding(ctx context.Context, bindingID string, record repo.RoleBindingRecord) (repo.RoleBindingRecord, error) {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE project_role_bindings
		 SET role = $1,
		     updated_at = $2,
		     updated_by = $3,
		     integrity_sha256 = $4
		 WHERE project_id = $5 AND binding_id = $6`,
		record.Role,
		record.UpdatedAt.UTC(),
		record.UpdatedBy,
		record.IntegritySHA,
		record.ProjectID,
		bindingID,
	)
	if err != nil {
		return repo.RoleBindingRecord{}, fmt.Errorf("update role binding: %w", err)
	}
	return s.GetByID(ctx, record.ProjectID, bindingID)
}

type roleBindingScanner interface {
	Scan(dest ...any) error
}

func scanRoleBinding(row roleBindingScanner) (repo.RoleBindingRecord, error) {
	var record repo.RoleBindingRecord
	err := row.Scan(
		&record.BindingID,
		&record.ProjectID,
		&record.SubjectType,
		&record.Subject,
		&record.Role,
		&record.CreatedAt,
		&record.CreatedBy,
		&record.UpdatedAt,
		&record.UpdatedBy,
		&record.IntegritySHA,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repo.RoleBindingRecord{}, repo.ErrNotFound
		}
		return repo.RoleBindingRecord{}, fmt.Errorf("scan role binding: %w", err)
	}
	return record, nil
}
