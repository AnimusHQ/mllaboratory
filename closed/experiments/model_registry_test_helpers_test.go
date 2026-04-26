package main

import (
	"context"
	"errors"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/lineageevent"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

type stubModelStore struct {
	models      map[string]domain.Model
	idempotency map[string]string
	createCalls int
	updateCalls int
}

func newStubModelStore() *stubModelStore {
	return &stubModelStore{
		models:      map[string]domain.Model{},
		idempotency: map[string]string{},
	}
}

func (s *stubModelStore) CreateModel(ctx context.Context, model domain.Model, idempotencyKey string) (domain.Model, bool, error) {
	s.createCalls++
	key := model.ProjectID + ":" + idempotencyKey
	if existingID, ok := s.idempotency[key]; ok {
		record, ok := s.models[existingID]
		if !ok {
			return domain.Model{}, false, errors.New("idempotency record missing")
		}
		return record, false, nil
	}
	s.models[model.ID] = model
	s.idempotency[key] = model.ID
	return model, true, nil
}

func (s *stubModelStore) GetModel(ctx context.Context, projectID, id string) (domain.Model, error) {
	model, ok := s.models[id]
	if !ok || model.ProjectID != projectID {
		return domain.Model{}, repo.ErrNotFound
	}
	return model, nil
}

func (s *stubModelStore) ListModels(ctx context.Context, filter repo.ModelFilter) ([]domain.Model, error) {
	out := make([]domain.Model, 0)
	for _, model := range s.models {
		if model.ProjectID != filter.ProjectID {
			continue
		}
		out = append(out, model)
	}
	return out, nil
}

func (s *stubModelStore) UpdateModelStatus(ctx context.Context, projectID, id string, status domain.ModelStatus) error {
	s.updateCalls++
	model, ok := s.models[id]
	if !ok || model.ProjectID != projectID {
		return repo.ErrNotFound
	}
	model.Status = status
	s.models[id] = model
	return nil
}

type stubModelVersionStore struct {
	versions    map[string]domain.ModelVersion
	idempotency map[string]string
	createCalls int
	updateCalls int
}

func newStubModelVersionStore() *stubModelVersionStore {
	return &stubModelVersionStore{
		versions:    map[string]domain.ModelVersion{},
		idempotency: map[string]string{},
	}
}

func (s *stubModelVersionStore) Create(ctx context.Context, version domain.ModelVersion, idempotencyKey string) (domain.ModelVersion, bool, error) {
	s.createCalls++
	key := version.ProjectID + ":" + idempotencyKey
	if existingID, ok := s.idempotency[key]; ok {
		record, ok := s.versions[existingID]
		if !ok {
			return domain.ModelVersion{}, false, errors.New("idempotency record missing")
		}
		return record, false, nil
	}
	s.versions[version.ID] = version
	s.idempotency[key] = version.ID
	return version, true, nil
}

func (s *stubModelVersionStore) Get(ctx context.Context, projectID, versionID string) (domain.ModelVersion, error) {
	version, ok := s.versions[versionID]
	if !ok || version.ProjectID != projectID {
		return domain.ModelVersion{}, repo.ErrNotFound
	}
	return version, nil
}

func (s *stubModelVersionStore) List(ctx context.Context, filter repo.ModelVersionFilter) ([]domain.ModelVersion, error) {
	out := make([]domain.ModelVersion, 0)
	for _, version := range s.versions {
		if version.ProjectID != filter.ProjectID {
			continue
		}
		if filter.ModelID != "" && version.ModelID != filter.ModelID {
			continue
		}
		out = append(out, version)
	}
	return out, nil
}

func (s *stubModelVersionStore) UpdateStatus(ctx context.Context, projectID, versionID string, status domain.ModelStatus) error {
	s.updateCalls++
	version, ok := s.versions[versionID]
	if !ok || version.ProjectID != projectID {
		return repo.ErrNotFound
	}
	version.Status = status
	s.versions[versionID] = version
	return nil
}

type stubModelVersionTransitionStore struct {
	transitions []domain.ModelVersionTransition
}

func (s *stubModelVersionTransitionStore) Insert(ctx context.Context, transition domain.ModelVersionTransition) error {
	s.transitions = append(s.transitions, transition)
	return nil
}

type stubModelVersionProvenanceStore struct {
	artifactIDs []string
	datasetIDs  []string
	calls       int
	err         error
}

func (s *stubModelVersionProvenanceStore) InsertArtifacts(ctx context.Context, projectID, modelVersionID string, artifactIDs []string, createdAt time.Time) error {
	s.calls++
	s.artifactIDs = append(s.artifactIDs, artifactIDs...)
	return s.err
}

func (s *stubModelVersionProvenanceStore) InsertDatasets(ctx context.Context, projectID, modelVersionID string, datasetVersionIDs []string, createdAt time.Time) error {
	s.calls++
	s.datasetIDs = append(s.datasetIDs, datasetVersionIDs...)
	return s.err
}

type stubRunBindingsStore struct {
	codeRef   domain.CodeRef
	envLock   domain.EnvLock
	policySHA string
	codeErr   error
	envErr    error
	policyErr error
}

func (s stubRunBindingsStore) GetCodeRef(ctx context.Context, projectID, runID string) (domain.CodeRef, error) {
	if s.codeErr != nil {
		return domain.CodeRef{}, s.codeErr
	}
	return s.codeRef, nil
}

func (s stubRunBindingsStore) GetEnvLock(ctx context.Context, projectID, runID string) (domain.EnvLock, error) {
	if s.envErr != nil {
		return domain.EnvLock{}, s.envErr
	}
	return s.envLock, nil
}

func (s stubRunBindingsStore) PolicySnapshotSHA(ctx context.Context, projectID, runID string) (string, error) {
	if s.policyErr != nil {
		return "", s.policyErr
	}
	return s.policySHA, nil
}

type captureLineage struct {
	events []lineageevent.Event
}

func (c *captureLineage) Append(ctx context.Context, event lineageevent.Event) error {
	c.events = append(c.events, event)
	return nil
}

type stubModelExportStore struct {
	exports     map[string]domain.ModelExport
	idempotency map[string]string
	createCalls int
}

func newStubModelExportStore() *stubModelExportStore {
	return &stubModelExportStore{
		exports:     map[string]domain.ModelExport{},
		idempotency: map[string]string{},
	}
}

func (s *stubModelExportStore) Create(ctx context.Context, export domain.ModelExport, idempotencyKey string) (domain.ModelExport, bool, error) {
	s.createCalls++
	key := export.ProjectID + ":" + idempotencyKey
	if existingID, ok := s.idempotency[key]; ok {
		record, ok := s.exports[existingID]
		if !ok {
			return domain.ModelExport{}, false, errors.New("idempotency record missing")
		}
		return record, false, nil
	}
	s.exports[export.ExportID] = export
	s.idempotency[key] = export.ExportID
	return export, true, nil
}
