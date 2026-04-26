package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/dataplane"
	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/repo"
	"github.com/animus-labs/animus-go/closed/internal/repo/postgres"
)

type stubDevEnvStore struct {
	records       map[string]postgres.DevEnvironmentRecord
	idempotency   map[string]string
	updateStates  []string
	lastAccess    map[string]time.Time
	listExpired   []postgres.DevEnvironmentRecord
	createCalls   int
	getCalls      int
	updateCalls   int
	lastAccessCnt int
}

func newStubDevEnvStore() *stubDevEnvStore {
	return &stubDevEnvStore{
		records:     map[string]postgres.DevEnvironmentRecord{},
		idempotency: map[string]string{},
		lastAccess:  map[string]time.Time{},
	}
}

func (s *stubDevEnvStore) Create(ctx context.Context, env domain.DevEnvironment, idempotencyKey string) (postgres.DevEnvironmentRecord, bool, error) {
	s.createCalls++
	if s.records == nil {
		s.records = map[string]postgres.DevEnvironmentRecord{}
	}
	if s.idempotency == nil {
		s.idempotency = map[string]string{}
	}
	key := env.ProjectID + ":" + idempotencyKey
	if existingID, ok := s.idempotency[key]; ok {
		record, ok := s.records[existingID]
		if !ok {
			return postgres.DevEnvironmentRecord{}, false, errors.New("idempotency record missing")
		}
		return record, false, nil
	}
	record := postgres.DevEnvironmentRecord{Environment: env, IdempotencyKey: idempotencyKey}
	s.records[env.ID] = record
	s.idempotency[key] = env.ID
	return record, true, nil
}

func (s *stubDevEnvStore) Get(ctx context.Context, projectID, devEnvID string) (postgres.DevEnvironmentRecord, error) {
	s.getCalls++
	record, ok := s.records[devEnvID]
	if !ok || record.Environment.ProjectID != projectID {
		return postgres.DevEnvironmentRecord{}, repo.ErrNotFound
	}
	return record, nil
}

func (s *stubDevEnvStore) List(ctx context.Context, projectID, state string, limit int) ([]postgres.DevEnvironmentRecord, error) {
	out := make([]postgres.DevEnvironmentRecord, 0, len(s.records))
	for _, record := range s.records {
		if record.Environment.ProjectID != projectID {
			continue
		}
		if state != "" && record.Environment.State != state {
			continue
		}
		out = append(out, record)
	}
	return out, nil
}

func (s *stubDevEnvStore) UpdateState(ctx context.Context, projectID, devEnvID, state, dpJobName, dpNamespace string) (bool, error) {
	s.updateCalls++
	record, ok := s.records[devEnvID]
	if !ok || record.Environment.ProjectID != projectID {
		return false, repo.ErrNotFound
	}
	record.Environment.State = state
	record.Environment.DPJobName = dpJobName
	record.Environment.DPNamespace = dpNamespace
	s.records[devEnvID] = record
	s.updateStates = append(s.updateStates, state)
	return true, nil
}

func (s *stubDevEnvStore) UpdateLastAccess(ctx context.Context, projectID, devEnvID string, accessedAt time.Time, minInterval time.Duration) (bool, error) {
	s.lastAccessCnt++
	record, ok := s.records[devEnvID]
	if !ok || record.Environment.ProjectID != projectID {
		return false, repo.ErrNotFound
	}
	record.Environment.LastAccessAt = &accessedAt
	s.records[devEnvID] = record
	s.lastAccess[devEnvID] = accessedAt
	return true, nil
}

func (s *stubDevEnvStore) ListExpired(ctx context.Context, projectID string, now time.Time, limit int) ([]postgres.DevEnvironmentRecord, error) {
	if s.listExpired != nil {
		return s.listExpired, nil
	}
	return nil, nil
}

type stubDevEnvPolicyStore struct {
	inserts []domain.PolicySnapshot
}

func (s *stubDevEnvPolicyStore) Insert(ctx context.Context, devEnvID, projectID string, snapshot domain.PolicySnapshot, snapshotJSON []byte, createdAt time.Time, createdBy, integritySHA string) error {
	s.inserts = append(s.inserts, snapshot)
	return nil
}

func (s *stubDevEnvPolicyStore) Get(ctx context.Context, projectID, devEnvID string) (domain.PolicySnapshot, error) {
	return domain.PolicySnapshot{}, repo.ErrNotFound
}

func (s *stubDevEnvPolicyStore) GetSHA(ctx context.Context, projectID, devEnvID string) (string, error) {
	return "", repo.ErrNotFound
}

type stubDevEnvSessionStore struct {
	sessions []domain.DevEnvAccessSession
	byID     map[string]domain.DevEnvAccessSession
}

func (s *stubDevEnvSessionStore) Insert(ctx context.Context, session domain.DevEnvAccessSession, integritySHA string) error {
	s.sessions = append(s.sessions, session)
	if s.byID == nil {
		s.byID = make(map[string]domain.DevEnvAccessSession)
	}
	s.byID[session.SessionID] = session
	return nil
}

func (s *stubDevEnvSessionStore) GetBySessionID(ctx context.Context, projectID, sessionID string) (domain.DevEnvAccessSession, error) {
	if s.byID == nil {
		return domain.DevEnvAccessSession{}, repo.ErrNotFound
	}
	session, ok := s.byID[sessionID]
	if !ok || session.ProjectID != projectID {
		return domain.DevEnvAccessSession{}, repo.ErrNotFound
	}
	return session, nil
}

type stubDevEnvDPClient struct {
	provisionResp  dataplane.DevEnvProvisionResponse
	accessResp     dataplane.DevEnvAccessResponse
	deleteResp     dataplane.DevEnvDeleteResponse
	provisionErr   error
	accessErr      error
	deleteErr      error
	provisionCode  int
	accessCode     int
	deleteCode     int
	provisionCalls int
	accessCalls    int
	deleteCalls    int
}

func (s *stubDevEnvDPClient) ProvisionDevEnv(ctx context.Context, req dataplane.DevEnvProvisionRequest, requestID string) (dataplane.DevEnvProvisionResponse, int, error) {
	s.provisionCalls++
	code := s.provisionCode
	if code == 0 {
		code = http.StatusOK
	}
	return s.provisionResp, code, s.provisionErr
}

func (s *stubDevEnvDPClient) DeleteDevEnv(ctx context.Context, req dataplane.DevEnvDeleteRequest, requestID string) (dataplane.DevEnvDeleteResponse, int, error) {
	s.deleteCalls++
	code := s.deleteCode
	if code == 0 {
		code = http.StatusOK
	}
	return s.deleteResp, code, s.deleteErr
}

func (s *stubDevEnvDPClient) AccessDevEnv(ctx context.Context, req dataplane.DevEnvAccessRequest, requestID string) (dataplane.DevEnvAccessResponse, int, error) {
	s.accessCalls++
	code := s.accessCode
	if code == 0 {
		code = http.StatusOK
	}
	return s.accessResp, code, s.accessErr
}

type stubEnvDefinitionStore struct {
	record postgres.EnvironmentDefinitionRecord
	err    error
}

func (s stubEnvDefinitionStore) GetDefinition(ctx context.Context, projectID, definitionID string) (postgres.EnvironmentDefinitionRecord, error) {
	if s.err != nil {
		return postgres.EnvironmentDefinitionRecord{}, s.err
	}
	return s.record, nil
}

type captureAudit struct {
	events []auditlog.Event
}

func (c *captureAudit) Append(ctx context.Context, event auditlog.Event) error {
	c.events = append(c.events, event)
	return nil
}

func (c *captureAudit) count(action string) int {
	count := 0
	for _, event := range c.events {
		if event.Action == action {
			count++
		}
	}
	return count
}

func testEnvDefinition() domain.EnvironmentDefinition {
	return domain.EnvironmentDefinition{
		ID:                   "tmpl-1",
		ProjectID:            "proj-1",
		Name:                 "dev",
		Version:              3,
		Status:               "active",
		CreatedAt:            time.Now().UTC(),
		CreatedBy:            "system",
		IntegritySHA256:      "def-sha",
		NetworkClassRef:      "net-1",
		SecretAccessClassRef: "secret-1",
		BaseImages: []domain.EnvironmentBaseImage{
			{Name: "base", Ref: "ghcr.io/acme/dev@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		},
		ResourceDefaults: domain.EnvironmentResources{CPU: "1", Memory: "1Gi"},
		ResourceLimits:   domain.EnvironmentResources{CPU: "2", Memory: "2Gi"},
	}
}

func testPolicySnapshot() domain.PolicySnapshot {
	return domain.PolicySnapshot{
		SnapshotVersion: "1.0",
		CapturedAt:      time.Now().UTC(),
		CapturedBy:      "user-1",
		RBAC: domain.PolicySnapshotRBAC{
			Subject:   "user-1",
			Roles:     []string{"admin"},
			ProjectID: "proj-1",
			Decision:  "allow",
		},
		SnapshotSHA256: "snapshot-sha",
	}
}
