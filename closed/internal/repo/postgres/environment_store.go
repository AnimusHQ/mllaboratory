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

type EnvironmentStore struct {
	db DB
}

const (
	insertEnvironmentDefinitionQuery = `INSERT INTO environment_definitions (
			environment_definition_id,
			project_id,
			name,
			version,
			description,
			base_images,
			resource_defaults,
			resource_limits,
			allowed_accelerators,
			network_class_ref,
			secret_access_class_ref,
			status,
			supersedes_definition_id,
			metadata,
			created_at,
			created_by,
			integrity_sha256,
			idempotency_key
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		ON CONFLICT (project_id, idempotency_key) DO NOTHING
		RETURNING environment_definition_id, project_id, name, version, description, base_images,
			resource_defaults, resource_limits, allowed_accelerators, network_class_ref, secret_access_class_ref,
			status, supersedes_definition_id, metadata, created_at, created_by, integrity_sha256, idempotency_key`
	selectEnvironmentDefinitionByIDQuery = `SELECT environment_definition_id, project_id, name, version, description,
			base_images, resource_defaults, resource_limits, allowed_accelerators, network_class_ref,
			secret_access_class_ref, status, supersedes_definition_id, metadata, created_at, created_by, integrity_sha256, idempotency_key
		FROM environment_definitions
		WHERE project_id = $1 AND environment_definition_id = $2`
	selectEnvironmentDefinitionByIdempotencyQuery = `SELECT environment_definition_id, project_id, name, version, description,
			base_images, resource_defaults, resource_limits, allowed_accelerators, network_class_ref,
			secret_access_class_ref, status, supersedes_definition_id, metadata, created_at, created_by, integrity_sha256, idempotency_key
		FROM environment_definitions
		WHERE project_id = $1 AND idempotency_key = $2`
	selectEnvironmentDefinitionsListQuery = `SELECT environment_definition_id, project_id, name, version, description,
			base_images, resource_defaults, resource_limits, allowed_accelerators, network_class_ref,
			secret_access_class_ref, status, supersedes_definition_id, metadata, created_at, created_by, integrity_sha256, idempotency_key
		FROM environment_definitions
		WHERE project_id = $1
			AND ($2 = '' OR name = $2)
			AND ($3 = '' OR status = $3)
		ORDER BY created_at DESC
		LIMIT $4`
	selectEnvironmentDefinitionMaxVersionQuery = `SELECT COALESCE(MAX(version), 0)
		FROM environment_definitions
		WHERE project_id = $1 AND name = $2`

	insertEnvironmentLockQuery = `INSERT INTO environment_locks (
			lock_id,
			project_id,
			environment_definition_id,
			environment_definition_version,
			images,
			resource_defaults,
			resource_limits,
			allowed_accelerators,
			network_class_ref,
			secret_access_class_ref,
			dependency_checksums,
			sbom_ref,
			env_hash,
			created_at,
			created_by,
			integrity_sha256,
			idempotency_key
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		ON CONFLICT (project_id, idempotency_key) DO NOTHING
		RETURNING lock_id, project_id, environment_definition_id, environment_definition_version,
			images, resource_defaults, resource_limits, allowed_accelerators, network_class_ref,
			secret_access_class_ref, dependency_checksums, sbom_ref, env_hash, created_at, created_by, integrity_sha256, idempotency_key`
	selectEnvironmentLockByIDQuery = `SELECT lock_id, project_id, environment_definition_id, environment_definition_version,
			images, resource_defaults, resource_limits, allowed_accelerators, network_class_ref,
			secret_access_class_ref, dependency_checksums, sbom_ref, env_hash, created_at, created_by, integrity_sha256, idempotency_key
		FROM environment_locks
		WHERE project_id = $1 AND lock_id = $2`
	selectEnvironmentLockByIdempotencyQuery = `SELECT lock_id, project_id, environment_definition_id, environment_definition_version,
			images, resource_defaults, resource_limits, allowed_accelerators, network_class_ref,
			secret_access_class_ref, dependency_checksums, sbom_ref, env_hash, created_at, created_by, integrity_sha256, idempotency_key
		FROM environment_locks
		WHERE project_id = $1 AND idempotency_key = $2`
	selectEnvironmentLocksListQuery = `SELECT lock_id, project_id, environment_definition_id, environment_definition_version,
			images, resource_defaults, resource_limits, allowed_accelerators, network_class_ref,
			secret_access_class_ref, dependency_checksums, sbom_ref, env_hash, created_at, created_by, integrity_sha256, idempotency_key
		FROM environment_locks
		WHERE project_id = $1
			AND ($2 = '' OR environment_definition_id = $2)
		ORDER BY created_at DESC
		LIMIT $3`
)

func NewEnvironmentStore(db DB) *EnvironmentStore {
	if db == nil {
		return nil
	}
	return &EnvironmentStore{db: db}
}

type EnvironmentDefinitionRecord struct {
	Definition     domain.EnvironmentDefinition
	IdempotencyKey string
}

type EnvironmentLockRecord struct {
	Lock           domain.EnvLock
	IdempotencyKey string
}

func (s *EnvironmentStore) CreateDefinition(ctx context.Context, def domain.EnvironmentDefinition, idempotencyKey string) (EnvironmentDefinitionRecord, bool, error) {
	if s == nil || s.db == nil {
		return EnvironmentDefinitionRecord{}, false, fmt.Errorf("environment store not initialized")
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if strings.TrimSpace(def.ProjectID) == "" {
		return EnvironmentDefinitionRecord{}, false, fmt.Errorf("project id is required")
	}
	if idempotencyKey == "" {
		return EnvironmentDefinitionRecord{}, false, fmt.Errorf("idempotency key is required")
	}
	if strings.TrimSpace(def.ID) == "" {
		return EnvironmentDefinitionRecord{}, false, fmt.Errorf("definition id is required")
	}
	if strings.TrimSpace(def.Name) == "" {
		return EnvironmentDefinitionRecord{}, false, fmt.Errorf("definition name is required")
	}
	if def.Version <= 0 {
		return EnvironmentDefinitionRecord{}, false, fmt.Errorf("definition version is required")
	}
	if strings.TrimSpace(def.Status) == "" {
		return EnvironmentDefinitionRecord{}, false, fmt.Errorf("definition status is required")
	}
	if strings.TrimSpace(def.CreatedBy) == "" {
		return EnvironmentDefinitionRecord{}, false, fmt.Errorf("created by is required")
	}
	if err := requireIntegrity(def.IntegritySHA256); err != nil {
		return EnvironmentDefinitionRecord{}, false, err
	}

	baseImagesJSON, err := json.Marshal(def.BaseImages)
	if err != nil {
		return EnvironmentDefinitionRecord{}, false, fmt.Errorf("marshal base images: %w", err)
	}
	resourceDefaultsJSON, err := json.Marshal(def.ResourceDefaults)
	if err != nil {
		return EnvironmentDefinitionRecord{}, false, fmt.Errorf("marshal resource defaults: %w", err)
	}
	resourceLimitsJSON, err := json.Marshal(def.ResourceLimits)
	if err != nil {
		return EnvironmentDefinitionRecord{}, false, fmt.Errorf("marshal resource limits: %w", err)
	}
	acceleratorsJSON, err := json.Marshal(def.AllowedAccelerators)
	if err != nil {
		return EnvironmentDefinitionRecord{}, false, fmt.Errorf("marshal allowed accelerators: %w", err)
	}
	metadataJSON, err := json.Marshal(def.Metadata)
	if err != nil {
		return EnvironmentDefinitionRecord{}, false, fmt.Errorf("marshal metadata: %w", err)
	}

	record := EnvironmentDefinitionRecord{}
	createdAt := normalizeTime(def.CreatedAt)
	err = s.db.QueryRowContext(
		ctx,
		insertEnvironmentDefinitionQuery,
		def.ID,
		def.ProjectID,
		def.Name,
		def.Version,
		nullIfEmpty(def.Description),
		baseImagesJSON,
		resourceDefaultsJSON,
		resourceLimitsJSON,
		acceleratorsJSON,
		nullIfEmpty(def.NetworkClassRef),
		nullIfEmpty(def.SecretAccessClassRef),
		def.Status,
		nullIfEmpty(def.SupersedesDefinitionID),
		metadataJSON,
		createdAt,
		def.CreatedBy,
		def.IntegritySHA256,
		idempotencyKey,
	).Scan(
		&record.Definition.ID,
		&record.Definition.ProjectID,
		&record.Definition.Name,
		&record.Definition.Version,
		&record.Definition.Description,
		&baseImagesJSON,
		&resourceDefaultsJSON,
		&resourceLimitsJSON,
		&acceleratorsJSON,
		&record.Definition.NetworkClassRef,
		&record.Definition.SecretAccessClassRef,
		&record.Definition.Status,
		&record.Definition.SupersedesDefinitionID,
		&metadataJSON,
		&record.Definition.CreatedAt,
		&record.Definition.CreatedBy,
		&record.Definition.IntegritySHA256,
		&record.IdempotencyKey,
	)
	if err != nil {
		if err != sql.ErrNoRows {
			return EnvironmentDefinitionRecord{}, false, fmt.Errorf("insert environment definition: %w", err)
		}
		existing, err := s.GetDefinitionByIdempotencyKey(ctx, def.ProjectID, idempotencyKey)
		if err != nil {
			return EnvironmentDefinitionRecord{}, false, err
		}
		return existing, false, nil
	}
	if err := hydrateDefinitionJSON(&record.Definition, baseImagesJSON, resourceDefaultsJSON, resourceLimitsJSON, acceleratorsJSON, metadataJSON); err != nil {
		return EnvironmentDefinitionRecord{}, false, err
	}
	return record, true, nil
}

func (s *EnvironmentStore) GetDefinition(ctx context.Context, projectID, definitionID string) (EnvironmentDefinitionRecord, error) {
	if s == nil || s.db == nil {
		return EnvironmentDefinitionRecord{}, fmt.Errorf("environment store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	definitionID = strings.TrimSpace(definitionID)
	if projectID == "" || definitionID == "" {
		return EnvironmentDefinitionRecord{}, fmt.Errorf("project id and definition id are required")
	}
	record := EnvironmentDefinitionRecord{}
	var baseImagesJSON, resourceDefaultsJSON, resourceLimitsJSON, acceleratorsJSON, metadataJSON []byte
	row := s.db.QueryRowContext(ctx, selectEnvironmentDefinitionByIDQuery, projectID, definitionID)
	if err := row.Scan(
		&record.Definition.ID,
		&record.Definition.ProjectID,
		&record.Definition.Name,
		&record.Definition.Version,
		&record.Definition.Description,
		&baseImagesJSON,
		&resourceDefaultsJSON,
		&resourceLimitsJSON,
		&acceleratorsJSON,
		&record.Definition.NetworkClassRef,
		&record.Definition.SecretAccessClassRef,
		&record.Definition.Status,
		&record.Definition.SupersedesDefinitionID,
		&metadataJSON,
		&record.Definition.CreatedAt,
		&record.Definition.CreatedBy,
		&record.Definition.IntegritySHA256,
		&record.IdempotencyKey,
	); err != nil {
		return EnvironmentDefinitionRecord{}, handleNotFound(err)
	}
	if err := hydrateDefinitionJSON(&record.Definition, baseImagesJSON, resourceDefaultsJSON, resourceLimitsJSON, acceleratorsJSON, metadataJSON); err != nil {
		return EnvironmentDefinitionRecord{}, err
	}
	return record, nil
}

func (s *EnvironmentStore) GetDefinitionByIdempotencyKey(ctx context.Context, projectID, idempotencyKey string) (EnvironmentDefinitionRecord, error) {
	if s == nil || s.db == nil {
		return EnvironmentDefinitionRecord{}, fmt.Errorf("environment store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if projectID == "" || idempotencyKey == "" {
		return EnvironmentDefinitionRecord{}, fmt.Errorf("project id and idempotency key are required")
	}
	record := EnvironmentDefinitionRecord{}
	var baseImagesJSON, resourceDefaultsJSON, resourceLimitsJSON, acceleratorsJSON, metadataJSON []byte
	row := s.db.QueryRowContext(ctx, selectEnvironmentDefinitionByIdempotencyQuery, projectID, idempotencyKey)
	if err := row.Scan(
		&record.Definition.ID,
		&record.Definition.ProjectID,
		&record.Definition.Name,
		&record.Definition.Version,
		&record.Definition.Description,
		&baseImagesJSON,
		&resourceDefaultsJSON,
		&resourceLimitsJSON,
		&acceleratorsJSON,
		&record.Definition.NetworkClassRef,
		&record.Definition.SecretAccessClassRef,
		&record.Definition.Status,
		&record.Definition.SupersedesDefinitionID,
		&metadataJSON,
		&record.Definition.CreatedAt,
		&record.Definition.CreatedBy,
		&record.Definition.IntegritySHA256,
		&record.IdempotencyKey,
	); err != nil {
		return EnvironmentDefinitionRecord{}, handleNotFound(err)
	}
	if err := hydrateDefinitionJSON(&record.Definition, baseImagesJSON, resourceDefaultsJSON, resourceLimitsJSON, acceleratorsJSON, metadataJSON); err != nil {
		return EnvironmentDefinitionRecord{}, err
	}
	return record, nil
}

func (s *EnvironmentStore) ListDefinitions(ctx context.Context, projectID, name, status string, limit int) ([]EnvironmentDefinitionRecord, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("environment store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project id is required")
	}
	name = strings.TrimSpace(name)
	status = strings.TrimSpace(status)
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, selectEnvironmentDefinitionsListQuery, projectID, name, status, limit)
	if err != nil {
		return nil, fmt.Errorf("list environment definitions: %w", err)
	}
	defer rows.Close()

	out := []EnvironmentDefinitionRecord{}
	for rows.Next() {
		record := EnvironmentDefinitionRecord{}
		var baseImagesJSON, resourceDefaultsJSON, resourceLimitsJSON, acceleratorsJSON, metadataJSON []byte
		if err := rows.Scan(
			&record.Definition.ID,
			&record.Definition.ProjectID,
			&record.Definition.Name,
			&record.Definition.Version,
			&record.Definition.Description,
			&baseImagesJSON,
			&resourceDefaultsJSON,
			&resourceLimitsJSON,
			&acceleratorsJSON,
			&record.Definition.NetworkClassRef,
			&record.Definition.SecretAccessClassRef,
			&record.Definition.Status,
			&record.Definition.SupersedesDefinitionID,
			&metadataJSON,
			&record.Definition.CreatedAt,
			&record.Definition.CreatedBy,
			&record.Definition.IntegritySHA256,
			&record.IdempotencyKey,
		); err != nil {
			return nil, err
		}
		if err := hydrateDefinitionJSON(&record.Definition, baseImagesJSON, resourceDefaultsJSON, resourceLimitsJSON, acceleratorsJSON, metadataJSON); err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *EnvironmentStore) NextDefinitionVersion(ctx context.Context, projectID, name string) (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("environment store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	name = strings.TrimSpace(name)
	if projectID == "" || name == "" {
		return 0, fmt.Errorf("project id and name are required")
	}
	var max int
	if err := s.db.QueryRowContext(ctx, selectEnvironmentDefinitionMaxVersionQuery, projectID, name).Scan(&max); err != nil {
		return 0, err
	}
	return max + 1, nil
}

func (s *EnvironmentStore) CreateLock(ctx context.Context, lock domain.EnvLock, projectID, createdBy, idempotencyKey string, createdAt time.Time, integritySHA string) (EnvironmentLockRecord, bool, error) {
	if s == nil || s.db == nil {
		return EnvironmentLockRecord{}, false, fmt.Errorf("environment store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	createdBy = strings.TrimSpace(createdBy)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if projectID == "" {
		return EnvironmentLockRecord{}, false, fmt.Errorf("project id is required")
	}
	if strings.TrimSpace(lock.LockID) == "" {
		return EnvironmentLockRecord{}, false, fmt.Errorf("lock id is required")
	}
	if strings.TrimSpace(lock.EnvironmentDefinitionID) == "" {
		return EnvironmentLockRecord{}, false, fmt.Errorf("environment definition id is required")
	}
	if lock.EnvironmentDefinitionVersion <= 0 {
		return EnvironmentLockRecord{}, false, fmt.Errorf("environment definition version is required")
	}
	if strings.TrimSpace(lock.EnvHash) == "" {
		return EnvironmentLockRecord{}, false, fmt.Errorf("env hash is required")
	}
	if createdBy == "" {
		return EnvironmentLockRecord{}, false, fmt.Errorf("created by is required")
	}
	if idempotencyKey == "" {
		return EnvironmentLockRecord{}, false, fmt.Errorf("idempotency key is required")
	}
	if err := requireIntegrity(integritySHA); err != nil {
		return EnvironmentLockRecord{}, false, err
	}

	imagesJSON, err := json.Marshal(lock.Images)
	if err != nil {
		return EnvironmentLockRecord{}, false, fmt.Errorf("marshal images: %w", err)
	}
	resourceDefaultsJSON, err := json.Marshal(lock.ResourceDefaults)
	if err != nil {
		return EnvironmentLockRecord{}, false, fmt.Errorf("marshal resource defaults: %w", err)
	}
	resourceLimitsJSON, err := json.Marshal(lock.ResourceLimits)
	if err != nil {
		return EnvironmentLockRecord{}, false, fmt.Errorf("marshal resource limits: %w", err)
	}
	acceleratorsJSON, err := json.Marshal(lock.AllowedAccelerators)
	if err != nil {
		return EnvironmentLockRecord{}, false, fmt.Errorf("marshal allowed accelerators: %w", err)
	}
	dependencyJSON, err := json.Marshal(lock.DependencyChecksums)
	if err != nil {
		return EnvironmentLockRecord{}, false, fmt.Errorf("marshal dependency checksums: %w", err)
	}

	createdAt = normalizeTime(createdAt)
	record := EnvironmentLockRecord{}
	err = s.db.QueryRowContext(
		ctx,
		insertEnvironmentLockQuery,
		lock.LockID,
		projectID,
		lock.EnvironmentDefinitionID,
		lock.EnvironmentDefinitionVersion,
		imagesJSON,
		resourceDefaultsJSON,
		resourceLimitsJSON,
		acceleratorsJSON,
		nullIfEmpty(lock.NetworkClassRef),
		nullIfEmpty(lock.SecretAccessClassRef),
		dependencyJSON,
		nullIfEmpty(lock.SBOMRef),
		lock.EnvHash,
		createdAt,
		createdBy,
		integritySHA,
		idempotencyKey,
	).Scan(
		&record.Lock.LockID,
		&record.Lock.ProjectID,
		&record.Lock.EnvironmentDefinitionID,
		&record.Lock.EnvironmentDefinitionVersion,
		&imagesJSON,
		&resourceDefaultsJSON,
		&resourceLimitsJSON,
		&acceleratorsJSON,
		&record.Lock.NetworkClassRef,
		&record.Lock.SecretAccessClassRef,
		&dependencyJSON,
		&record.Lock.SBOMRef,
		&record.Lock.EnvHash,
		&record.Lock.CreatedAt,
		&record.Lock.CreatedBy,
		&record.Lock.IntegritySHA256,
		&record.IdempotencyKey,
	)
	if err != nil {
		if err != sql.ErrNoRows {
			return EnvironmentLockRecord{}, false, fmt.Errorf("insert environment lock: %w", err)
		}
		existing, err := s.GetLockByIdempotencyKey(ctx, projectID, idempotencyKey)
		if err != nil {
			return EnvironmentLockRecord{}, false, err
		}
		return existing, false, nil
	}
	if err := hydrateLockJSON(&record.Lock, imagesJSON, resourceDefaultsJSON, resourceLimitsJSON, acceleratorsJSON, dependencyJSON); err != nil {
		return EnvironmentLockRecord{}, false, err
	}
	return record, true, nil
}

func (s *EnvironmentStore) GetLock(ctx context.Context, projectID, lockID string) (EnvironmentLockRecord, error) {
	if s == nil || s.db == nil {
		return EnvironmentLockRecord{}, fmt.Errorf("environment store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	lockID = strings.TrimSpace(lockID)
	if projectID == "" || lockID == "" {
		return EnvironmentLockRecord{}, fmt.Errorf("project id and lock id are required")
	}
	record := EnvironmentLockRecord{}
	var imagesJSON, resourceDefaultsJSON, resourceLimitsJSON, acceleratorsJSON, dependencyJSON []byte
	row := s.db.QueryRowContext(ctx, selectEnvironmentLockByIDQuery, projectID, lockID)
	if err := row.Scan(
		&record.Lock.LockID,
		&record.Lock.ProjectID,
		&record.Lock.EnvironmentDefinitionID,
		&record.Lock.EnvironmentDefinitionVersion,
		&imagesJSON,
		&resourceDefaultsJSON,
		&resourceLimitsJSON,
		&acceleratorsJSON,
		&record.Lock.NetworkClassRef,
		&record.Lock.SecretAccessClassRef,
		&dependencyJSON,
		&record.Lock.SBOMRef,
		&record.Lock.EnvHash,
		&record.Lock.CreatedAt,
		&record.Lock.CreatedBy,
		&record.Lock.IntegritySHA256,
		&record.IdempotencyKey,
	); err != nil {
		return EnvironmentLockRecord{}, handleNotFound(err)
	}
	if err := hydrateLockJSON(&record.Lock, imagesJSON, resourceDefaultsJSON, resourceLimitsJSON, acceleratorsJSON, dependencyJSON); err != nil {
		return EnvironmentLockRecord{}, err
	}
	return record, nil
}

func (s *EnvironmentStore) GetLockByIdempotencyKey(ctx context.Context, projectID, idempotencyKey string) (EnvironmentLockRecord, error) {
	if s == nil || s.db == nil {
		return EnvironmentLockRecord{}, fmt.Errorf("environment store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if projectID == "" || idempotencyKey == "" {
		return EnvironmentLockRecord{}, fmt.Errorf("project id and idempotency key are required")
	}
	record := EnvironmentLockRecord{}
	var imagesJSON, resourceDefaultsJSON, resourceLimitsJSON, acceleratorsJSON, dependencyJSON []byte
	row := s.db.QueryRowContext(ctx, selectEnvironmentLockByIdempotencyQuery, projectID, idempotencyKey)
	if err := row.Scan(
		&record.Lock.LockID,
		&record.Lock.ProjectID,
		&record.Lock.EnvironmentDefinitionID,
		&record.Lock.EnvironmentDefinitionVersion,
		&imagesJSON,
		&resourceDefaultsJSON,
		&resourceLimitsJSON,
		&acceleratorsJSON,
		&record.Lock.NetworkClassRef,
		&record.Lock.SecretAccessClassRef,
		&dependencyJSON,
		&record.Lock.SBOMRef,
		&record.Lock.EnvHash,
		&record.Lock.CreatedAt,
		&record.Lock.CreatedBy,
		&record.Lock.IntegritySHA256,
		&record.IdempotencyKey,
	); err != nil {
		return EnvironmentLockRecord{}, handleNotFound(err)
	}
	if err := hydrateLockJSON(&record.Lock, imagesJSON, resourceDefaultsJSON, resourceLimitsJSON, acceleratorsJSON, dependencyJSON); err != nil {
		return EnvironmentLockRecord{}, err
	}
	return record, nil
}

func (s *EnvironmentStore) ListLocks(ctx context.Context, projectID, definitionID string, limit int) ([]EnvironmentLockRecord, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("environment store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	definitionID = strings.TrimSpace(definitionID)
	if projectID == "" {
		return nil, fmt.Errorf("project id is required")
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, selectEnvironmentLocksListQuery, projectID, definitionID, limit)
	if err != nil {
		return nil, fmt.Errorf("list environment locks: %w", err)
	}
	defer rows.Close()

	out := []EnvironmentLockRecord{}
	for rows.Next() {
		record := EnvironmentLockRecord{}
		var imagesJSON, resourceDefaultsJSON, resourceLimitsJSON, acceleratorsJSON, dependencyJSON []byte
		if err := rows.Scan(
			&record.Lock.LockID,
			&record.Lock.ProjectID,
			&record.Lock.EnvironmentDefinitionID,
			&record.Lock.EnvironmentDefinitionVersion,
			&imagesJSON,
			&resourceDefaultsJSON,
			&resourceLimitsJSON,
			&acceleratorsJSON,
			&record.Lock.NetworkClassRef,
			&record.Lock.SecretAccessClassRef,
			&dependencyJSON,
			&record.Lock.SBOMRef,
			&record.Lock.EnvHash,
			&record.Lock.CreatedAt,
			&record.Lock.CreatedBy,
			&record.Lock.IntegritySHA256,
			&record.IdempotencyKey,
		); err != nil {
			return nil, err
		}
		if err := hydrateLockJSON(&record.Lock, imagesJSON, resourceDefaultsJSON, resourceLimitsJSON, acceleratorsJSON, dependencyJSON); err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func hydrateDefinitionJSON(def *domain.EnvironmentDefinition, baseImagesJSON, resourceDefaultsJSON, resourceLimitsJSON, acceleratorsJSON, metadataJSON []byte) error {
	if def == nil {
		return fmt.Errorf("definition is nil")
	}
	if len(baseImagesJSON) > 0 {
		if err := json.Unmarshal(baseImagesJSON, &def.BaseImages); err != nil {
			return fmt.Errorf("decode base images: %w", err)
		}
	}
	if len(resourceDefaultsJSON) > 0 {
		if err := json.Unmarshal(resourceDefaultsJSON, &def.ResourceDefaults); err != nil {
			return fmt.Errorf("decode resource defaults: %w", err)
		}
	}
	if len(resourceLimitsJSON) > 0 {
		if err := json.Unmarshal(resourceLimitsJSON, &def.ResourceLimits); err != nil {
			return fmt.Errorf("decode resource limits: %w", err)
		}
	}
	if len(acceleratorsJSON) > 0 {
		if err := json.Unmarshal(acceleratorsJSON, &def.AllowedAccelerators); err != nil {
			return fmt.Errorf("decode accelerators: %w", err)
		}
	}
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &def.Metadata); err != nil {
			return fmt.Errorf("decode metadata: %w", err)
		}
	}
	return nil
}

func hydrateLockJSON(lock *domain.EnvLock, imagesJSON, resourceDefaultsJSON, resourceLimitsJSON, acceleratorsJSON, dependencyJSON []byte) error {
	if lock == nil {
		return fmt.Errorf("lock is nil")
	}
	if len(imagesJSON) > 0 {
		if err := json.Unmarshal(imagesJSON, &lock.Images); err != nil {
			return fmt.Errorf("decode images: %w", err)
		}
	}
	if len(resourceDefaultsJSON) > 0 {
		if err := json.Unmarshal(resourceDefaultsJSON, &lock.ResourceDefaults); err != nil {
			return fmt.Errorf("decode resource defaults: %w", err)
		}
	}
	if len(resourceLimitsJSON) > 0 {
		if err := json.Unmarshal(resourceLimitsJSON, &lock.ResourceLimits); err != nil {
			return fmt.Errorf("decode resource limits: %w", err)
		}
	}
	if len(acceleratorsJSON) > 0 {
		if err := json.Unmarshal(acceleratorsJSON, &lock.AllowedAccelerators); err != nil {
			return fmt.Errorf("decode accelerators: %w", err)
		}
	}
	if len(dependencyJSON) > 0 {
		if err := json.Unmarshal(dependencyJSON, &lock.DependencyChecksums); err != nil {
			return fmt.Errorf("decode dependency checksums: %w", err)
		}
	}
	return nil
}
