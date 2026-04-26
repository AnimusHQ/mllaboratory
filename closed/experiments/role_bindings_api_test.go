package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

type stubRoleBindingStore struct {
	records map[string]repo.RoleBindingRecord
}

func (s *stubRoleBindingStore) Upsert(ctx context.Context, record repo.RoleBindingRecord) (repo.RoleBindingRecord, bool, error) {
	if s.records == nil {
		s.records = map[string]repo.RoleBindingRecord{}
	}
	if record.BindingID == "" {
		record.BindingID = "binding-1"
	}
	s.records[record.BindingID] = record
	return record, true, nil
}

func (s *stubRoleBindingStore) ListByProject(ctx context.Context, projectID string) ([]repo.RoleBindingRecord, error) {
	out := make([]repo.RoleBindingRecord, 0, len(s.records))
	for _, record := range s.records {
		if record.ProjectID != projectID {
			continue
		}
		out = append(out, record)
	}
	return out, nil
}

func (s *stubRoleBindingStore) ListBySubjects(ctx context.Context, projectID string, subjects []repo.RoleBindingSubject) ([]repo.RoleBindingRecord, error) {
	for _, subject := range subjects {
		for _, record := range s.records {
			if record.ProjectID == projectID && record.SubjectType == subject.Type && record.Subject == subject.Value {
				return []repo.RoleBindingRecord{record}, nil
			}
		}
	}
	return nil, nil
}

func (s *stubRoleBindingStore) GetByID(ctx context.Context, projectID, bindingID string) (repo.RoleBindingRecord, error) {
	record, ok := s.records[bindingID]
	if !ok || record.ProjectID != projectID {
		return repo.RoleBindingRecord{}, repo.ErrNotFound
	}
	return record, nil
}

func (s *stubRoleBindingStore) Delete(ctx context.Context, projectID, bindingID string) error {
	delete(s.records, bindingID)
	return nil
}

type stubAuditAppender struct {
	events []domain.AuditEvent
}

func (s *stubAuditAppender) Append(ctx context.Context, event domain.AuditEvent) (int64, error) {
	s.events = append(s.events, event)
	return int64(len(s.events)), nil
}

func TestRoleBindingUpsertEmitsAudit(t *testing.T) {
	store := &stubRoleBindingStore{}
	audit := &stubAuditAppender{}
	api := &experimentsAPI{
		roleBindingStoreOverride: store,
		roleBindingAuditOverride: audit,
	}

	body := `{"subject_type":"group","subject":"ml-team","role":"viewer"}`
	req := httptest.NewRequest(http.MethodPost, "/projects/proj-1/role-bindings", bytes.NewBufferString(body))
	req = req.WithContext(auth.ContextWithIdentity(req.Context(), auth.Identity{Subject: "admin-1"}))
	req = req.WithContext(auth.ContextWithProjectID(req.Context(), "proj-1"))
	resp := httptest.NewRecorder()

	api.handleUpsertRoleBinding(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", resp.Code)
	}
	if len(audit.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(audit.events))
	}
	if audit.events[0].Action != "rbac.role_binding_created" {
		t.Fatalf("unexpected audit action: %s", audit.events[0].Action)
	}
}

func TestRoleBindingDeleteEmitsAudit(t *testing.T) {
	store := &stubRoleBindingStore{records: map[string]repo.RoleBindingRecord{
		"binding-1": {
			BindingID:   "binding-1",
			ProjectID:   "proj-1",
			SubjectType: "group",
			Subject:     "ml-team",
			Role:        "viewer",
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		},
	}}
	audit := &stubAuditAppender{}
	api := &experimentsAPI{
		roleBindingStoreOverride: store,
		roleBindingAuditOverride: audit,
	}

	req := httptest.NewRequest(http.MethodPost, "/projects/proj-1/role-bindings/binding-1:delete", nil)
	req.SetPathValue("binding_id", "binding-1")
	req = req.WithContext(auth.ContextWithIdentity(req.Context(), auth.Identity{Subject: "admin-1"}))
	req = req.WithContext(auth.ContextWithProjectID(req.Context(), "proj-1"))
	resp := httptest.NewRecorder()

	api.handleDeleteRoleBinding(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", resp.Code)
	}
	if len(audit.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(audit.events))
	}
	if audit.events[0].Action != "rbac.role_binding_deleted" {
		t.Fatalf("unexpected audit action: %s", audit.events[0].Action)
	}
}
