package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/repo"
	repopg "github.com/animus-labs/animus-go/closed/internal/repo/postgres"
)

type roleBinding struct {
	BindingID   string    `json:"binding_id"`
	ProjectID   string    `json:"project_id"`
	SubjectType string    `json:"subject_type"`
	Subject     string    `json:"subject"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   string    `json:"created_by"`
	UpdatedAt   time.Time `json:"updated_at"`
	UpdatedBy   string    `json:"updated_by"`
}

type roleBindingRequest struct {
	SubjectType string `json:"subject_type"`
	Subject     string `json:"subject"`
	Role        string `json:"role"`
}

type roleBindingResponse struct {
	Binding roleBinding `json:"binding"`
	Created bool        `json:"created"`
}

type roleBindingListResponse struct {
	Bindings []roleBinding `json:"bindings"`
}

type roleBindingStore interface {
	Upsert(ctx context.Context, record repo.RoleBindingRecord) (repo.RoleBindingRecord, bool, error)
	ListByProject(ctx context.Context, projectID string) ([]repo.RoleBindingRecord, error)
	ListBySubjects(ctx context.Context, projectID string, subjects []repo.RoleBindingSubject) ([]repo.RoleBindingRecord, error)
	GetByID(ctx context.Context, projectID, bindingID string) (repo.RoleBindingRecord, error)
	Delete(ctx context.Context, projectID, bindingID string) error
}

func (api *experimentsAPI) roleBindingStore() roleBindingStore {
	if api == nil {
		return nil
	}
	if api.roleBindingStoreOverride != nil {
		return api.roleBindingStoreOverride
	}
	return repopg.NewRoleBindingStore(api.db)
}

func (api *experimentsAPI) roleBindingAuditAppender() repo.AuditEventAppender {
	if api == nil {
		return nil
	}
	if api.roleBindingAuditOverride != nil {
		return api.roleBindingAuditOverride
	}
	if api.db == nil {
		return nil
	}
	return repopg.NewAuditAppender(api.db, nil)
}

func (api *experimentsAPI) handleUpsertRoleBinding(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	projectID, ok := auth.ProjectIDFromContext(r.Context())
	if !ok || strings.TrimSpace(projectID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}

	var req roleBindingRequest
	if err := decodeJSON(r, &req); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}

	subjectType := strings.ToLower(strings.TrimSpace(req.SubjectType))
	subject := strings.TrimSpace(req.Subject)
	role := strings.ToLower(strings.TrimSpace(req.Role))
	if subjectType == "" || subject == "" || role == "" {
		api.writeError(w, r, http.StatusBadRequest, "binding_fields_required")
		return
	}
	switch subjectType {
	case "subject", "email", "group", "service":
		// ok
	default:
		api.writeError(w, r, http.StatusBadRequest, "subject_type_invalid")
		return
	}
	if subjectType == "service" && !strings.HasPrefix(strings.ToLower(subject), "service:") {
		api.writeError(w, r, http.StatusBadRequest, "subject_invalid")
		return
	}
	switch role {
	case auth.RoleViewer, auth.RoleEditor, auth.RoleAdmin:
		// ok
	default:
		api.writeError(w, r, http.StatusBadRequest, "role_invalid")
		return
	}

	store := api.roleBindingStore()
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	var existing *repo.RoleBindingRecord
	current, err := store.ListBySubjects(r.Context(), projectID, []repo.RoleBindingSubject{{Type: subjectType, Value: subject}})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if len(current) > 0 {
		existing = &current[0]
	}

	now := time.Now().UTC()
	createdAt := now
	createdBy := identity.Subject
	bindingID := ""
	if existing != nil {
		createdAt = existing.CreatedAt
		createdBy = existing.CreatedBy
		bindingID = existing.BindingID
	}

	integrity, err := integritySHA256(struct {
		BindingID   string    `json:"binding_id"`
		ProjectID   string    `json:"project_id"`
		SubjectType string    `json:"subject_type"`
		Subject     string    `json:"subject"`
		Role        string    `json:"role"`
		CreatedAt   time.Time `json:"created_at"`
		CreatedBy   string    `json:"created_by"`
		UpdatedAt   time.Time `json:"updated_at"`
		UpdatedBy   string    `json:"updated_by"`
	}{
		BindingID:   bindingID,
		ProjectID:   projectID,
		SubjectType: subjectType,
		Subject:     subject,
		Role:        role,
		CreatedAt:   createdAt,
		CreatedBy:   createdBy,
		UpdatedAt:   now,
		UpdatedBy:   identity.Subject,
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	record, created, err := store.Upsert(r.Context(), repo.RoleBindingRecord{
		BindingID:    bindingID,
		ProjectID:    projectID,
		SubjectType:  subjectType,
		Subject:      subject,
		Role:         role,
		CreatedAt:    createdAt,
		CreatedBy:    createdBy,
		UpdatedAt:    now,
		UpdatedBy:    identity.Subject,
		IntegritySHA: integrity,
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "binding_upsert_failed")
		return
	}

	api.appendAuditEvent(r, "rbac.role_binding_"+boolToAction(created), record, identity.Subject)

	w.Header().Set("Location", "/projects/"+projectID+"/role-bindings/"+record.BindingID)
	api.writeJSON(w, http.StatusOK, roleBindingResponse{Binding: roleBindingFromRecord(record), Created: created})
}

func (api *experimentsAPI) handleListRoleBindings(w http.ResponseWriter, r *http.Request) {
	projectID, ok := auth.ProjectIDFromContext(r.Context())
	if !ok || strings.TrimSpace(projectID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}

	store := api.roleBindingStore()
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	bindings, err := store.ListByProject(r.Context(), projectID)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	out := make([]roleBinding, 0, len(bindings))
	for _, b := range bindings {
		out = append(out, roleBindingFromRecord(b))
	}
	api.writeJSON(w, http.StatusOK, roleBindingListResponse{Bindings: out})
}

func (api *experimentsAPI) handleDeleteRoleBinding(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	projectID, ok := auth.ProjectIDFromContext(r.Context())
	if !ok || strings.TrimSpace(projectID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	bindingID := strings.TrimSpace(r.PathValue("binding_id"))
	if bindingID == "" {
		api.writeError(w, r, http.StatusBadRequest, "binding_id_required")
		return
	}

	store := api.roleBindingStore()
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	binding, err := store.GetByID(r.Context(), projectID, bindingID)
	if err != nil {
		api.writeError(w, r, http.StatusNotFound, "not_found")
		return
	}
	if err := store.Delete(r.Context(), projectID, bindingID); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "binding_delete_failed")
		return
	}

	api.appendAuditEvent(r, "rbac.role_binding_deleted", binding, identity.Subject)
	api.writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (api *experimentsAPI) appendAuditEvent(r *http.Request, action string, record repo.RoleBindingRecord, actor string) {
	if api == nil {
		return
	}
	appender := api.roleBindingAuditAppender()
	if appender == nil {
		return
	}
	payload := map[string]any{
		"project_id":   record.ProjectID,
		"binding_id":   record.BindingID,
		"subject_type": record.SubjectType,
		"subject":      record.Subject,
		"role":         record.Role,
	}

	_, _ = appender.Append(r.Context(), domain.AuditEvent{
		OccurredAt:   time.Now().UTC(),
		Actor:        strings.TrimSpace(actor),
		Action:       action,
		ResourceType: "role_binding",
		ResourceID:   record.BindingID,
		RequestID:    r.Header.Get("X-Request-Id"),
		Payload:      payload,
	})
}

func roleBindingFromRecord(record repo.RoleBindingRecord) roleBinding {
	return roleBinding{
		BindingID:   record.BindingID,
		ProjectID:   record.ProjectID,
		SubjectType: record.SubjectType,
		Subject:     record.Subject,
		Role:        record.Role,
		CreatedAt:   record.CreatedAt,
		CreatedBy:   record.CreatedBy,
		UpdatedAt:   record.UpdatedAt,
		UpdatedBy:   record.UpdatedBy,
	}
}

func boolToAction(created bool) string {
	if created {
		return "created"
	}
	return "updated"
}
