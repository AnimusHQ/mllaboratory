package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/integrations/webhooks"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/platform/redaction"
	"github.com/animus-labs/animus-go/closed/internal/repo"
	repopg "github.com/animus-labs/animus-go/closed/internal/repo/postgres"
	"github.com/google/uuid"
)

const (
	auditWebhookSubscriptionCreated  = "webhook.subscription.created"
	auditWebhookSubscriptionUpdated  = "webhook.subscription.updated"
	auditWebhookSubscriptionEnabled  = "webhook.subscription.enabled"
	auditWebhookSubscriptionDisabled = "webhook.subscription.disabled"
	auditWebhookDeliveryEnqueued     = "webhook.delivery.enqueued"
	auditWebhookDeliveryReplay       = "webhook.delivery.replay_requested"
)

type webhookSubscriptionRequest struct {
	Name       string            `json:"name"`
	TargetURL  string            `json:"target_url"`
	Enabled    *bool             `json:"enabled,omitempty"`
	EventTypes []string          `json:"event_types"`
	SecretRef  string            `json:"secret_ref,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
}

type webhookSubscriptionUpdateRequest struct {
	Name       *string            `json:"name,omitempty"`
	TargetURL  *string            `json:"target_url,omitempty"`
	Enabled    *bool              `json:"enabled,omitempty"`
	EventTypes *[]string          `json:"event_types,omitempty"`
	SecretRef  *string            `json:"secret_ref,omitempty"`
	Headers    *map[string]string `json:"headers,omitempty"`
}

type webhookSubscriptionResponse struct {
	ID         string            `json:"id"`
	ProjectID  string            `json:"project_id"`
	Name       string            `json:"name"`
	TargetURL  string            `json:"target_url"`
	Enabled    bool              `json:"enabled"`
	EventTypes []string          `json:"event_types"`
	SecretRef  string            `json:"secret_ref,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

type webhookDeliveryResponse struct {
	ID             string    `json:"id"`
	ProjectID      string    `json:"project_id"`
	SubscriptionID string    `json:"subscription_id"`
	EventID        string    `json:"event_id"`
	EventType      string    `json:"event_type"`
	Status         string    `json:"status"`
	NextAttemptAt  time.Time `json:"next_attempt_at"`
	AttemptCount   int       `json:"attempt_count"`
	LastError      string    `json:"last_error,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type webhookDeliveryAttemptResponse struct {
	ID          int64     `json:"id"`
	DeliveryID  string    `json:"delivery_id"`
	AttemptedAt time.Time `json:"attempted_at"`
	StatusCode  *int      `json:"status_code,omitempty"`
	Outcome     string    `json:"outcome"`
	Error       string    `json:"error,omitempty"`
	LatencyMs   int       `json:"latency_ms,omitempty"`
	RequestID   string    `json:"request_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type webhookReplayRequest struct {
	ReplayToken string `json:"replay_token"`
}

func (api *experimentsAPI) handleCreateWebhookSubscription(w http.ResponseWriter, r *http.Request) {
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

	var req webhookSubscriptionRequest
	if err := decodeJSON(r, &req); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}

	name := strings.TrimSpace(req.Name)
	targetURL := strings.TrimSpace(req.TargetURL)
	if name == "" || targetURL == "" {
		api.writeError(w, r, http.StatusBadRequest, "name_target_url_required")
		return
	}

	eventTypes, err := webhooks.NormalizeEventTypes(req.EventTypes)
	if err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_event_types")
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	now := time.Now().UTC()
	record := webhooks.Subscription{
		ID:         uuid.NewString(),
		ProjectID:  projectID,
		Name:       name,
		TargetURL:  targetURL,
		Enabled:    enabled,
		EventTypes: eventTypes,
		SecretRef:  strings.TrimSpace(req.SecretRef),
		Headers:    normalizeWebhookHeaders(req.Headers),
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	store := repopg.NewWebhookSubscriptionStore(api.db)
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	created, err := store.Create(r.Context(), record)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "subscription_create_failed")
		return
	}

	api.appendWebhookAudit(r, auditWebhookSubscriptionCreated, identity.Subject, map[string]any{
		"project_id":      created.ProjectID,
		"subscription_id": created.ID,
		"enabled":         created.Enabled,
		"event_types":     eventTypesToStrings(created.EventTypes),
	})
	if created.Enabled {
		api.appendWebhookAudit(r, auditWebhookSubscriptionEnabled, identity.Subject, map[string]any{
			"project_id":      created.ProjectID,
			"subscription_id": created.ID,
		})
	}

	api.writeJSON(w, http.StatusCreated, webhookSubscriptionResponseFromRecord(created))
}

func (api *experimentsAPI) handleListWebhookSubscriptions(w http.ResponseWriter, r *http.Request) {
	projectID, ok := auth.ProjectIDFromContext(r.Context())
	if !ok || strings.TrimSpace(projectID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}

	limit := clampInt(parseIntQuery(r, "limit", 100), 1, 500)
	store := repopg.NewWebhookSubscriptionStore(api.db)
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	records, err := store.List(r.Context(), projectID, limit)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "subscription_list_failed")
		return
	}

	out := make([]webhookSubscriptionResponse, 0, len(records))
	for _, record := range records {
		out = append(out, webhookSubscriptionResponseFromRecord(record))
	}
	api.writeJSON(w, http.StatusOK, map[string]any{"subscriptions": out})
}

func (api *experimentsAPI) handleUpdateWebhookSubscription(w http.ResponseWriter, r *http.Request) {
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
	subscriptionID := strings.TrimSpace(r.PathValue("subscription_id"))
	if subscriptionID == "" {
		api.writeError(w, r, http.StatusBadRequest, "subscription_id_required")
		return
	}

	var req webhookSubscriptionUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}

	store := repopg.NewWebhookSubscriptionStore(api.db)
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	current, err := store.Get(r.Context(), projectID, subscriptionID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "subscription_lookup_failed")
		return
	}

	updated := current
	if req.Name != nil {
		updated.Name = strings.TrimSpace(*req.Name)
	}
	if req.TargetURL != nil {
		updated.TargetURL = strings.TrimSpace(*req.TargetURL)
	}
	if req.EventTypes != nil {
		values, err := webhooks.NormalizeEventTypes(*req.EventTypes)
		if err != nil {
			api.writeError(w, r, http.StatusBadRequest, "invalid_event_types")
			return
		}
		updated.EventTypes = values
	}
	if req.SecretRef != nil {
		updated.SecretRef = strings.TrimSpace(*req.SecretRef)
	}
	if req.Headers != nil {
		updated.Headers = normalizeWebhookHeaders(*req.Headers)
	}
	if req.Enabled != nil {
		updated.Enabled = *req.Enabled
	}
	if strings.TrimSpace(updated.Name) == "" || strings.TrimSpace(updated.TargetURL) == "" {
		api.writeError(w, r, http.StatusBadRequest, "name_target_url_required")
		return
	}

	updated.UpdatedAt = time.Now().UTC()
	record, err := store.Update(r.Context(), updated)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "subscription_update_failed")
		return
	}

	api.appendWebhookAudit(r, auditWebhookSubscriptionUpdated, identity.Subject, map[string]any{
		"project_id":      record.ProjectID,
		"subscription_id": record.ID,
		"enabled":         record.Enabled,
		"event_types":     eventTypesToStrings(record.EventTypes),
	})
	if current.Enabled != record.Enabled {
		action := auditWebhookSubscriptionDisabled
		if record.Enabled {
			action = auditWebhookSubscriptionEnabled
		}
		api.appendWebhookAudit(r, action, identity.Subject, map[string]any{
			"project_id":      record.ProjectID,
			"subscription_id": record.ID,
		})
	}

	api.writeJSON(w, http.StatusOK, webhookSubscriptionResponseFromRecord(record))
}

func (api *experimentsAPI) handleListWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	projectID, ok := auth.ProjectIDFromContext(r.Context())
	if !ok || strings.TrimSpace(projectID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}

	var eventType webhooks.EventType
	if raw := strings.TrimSpace(r.URL.Query().Get("event_type")); raw != "" {
		eventType = webhooks.EventType(raw)
		if !eventType.Valid() {
			api.writeError(w, r, http.StatusBadRequest, "invalid_event_type")
			return
		}
	}

	var status webhooks.DeliveryStatus
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		status = webhooks.DeliveryStatus(raw)
	}

	limit := clampInt(parseIntQuery(r, "limit", 200), 1, 500)
	store := repopg.NewWebhookDeliveryStore(api.db)
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	records, err := store.List(r.Context(), projectID, eventType, status, limit)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "delivery_list_failed")
		return
	}

	out := make([]webhookDeliveryResponse, 0, len(records))
	for _, record := range records {
		out = append(out, webhookDeliveryResponseFromRecord(record))
	}
	api.writeJSON(w, http.StatusOK, map[string]any{"deliveries": out})
}

func (api *experimentsAPI) handleListWebhookDeliveryAttempts(w http.ResponseWriter, r *http.Request) {
	projectID, ok := auth.ProjectIDFromContext(r.Context())
	if !ok || strings.TrimSpace(projectID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	deliveryID := strings.TrimSpace(r.PathValue("delivery_id"))
	if deliveryID == "" {
		api.writeError(w, r, http.StatusBadRequest, "delivery_id_required")
		return
	}

	deliveryStore := repopg.NewWebhookDeliveryStore(api.db)
	if deliveryStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	record, err := deliveryStore.Get(r.Context(), projectID, deliveryID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "delivery_lookup_failed")
		return
	}

	limit := clampInt(parseIntQuery(r, "limit", 200), 1, 500)
	attemptStore := repopg.NewWebhookDeliveryAttemptStore(api.db)
	if attemptStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	attempts, err := attemptStore.List(r.Context(), record.ID, limit)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "attempt_list_failed")
		return
	}

	out := make([]webhookDeliveryAttemptResponse, 0, len(attempts))
	for _, attempt := range attempts {
		out = append(out, webhookDeliveryAttemptResponseFromRecord(attempt))
	}
	api.writeJSON(w, http.StatusOK, map[string]any{"attempts": out})
}

func (api *experimentsAPI) handleReplayWebhookDelivery(w http.ResponseWriter, r *http.Request) {
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
	deliveryID := strings.TrimSpace(r.PathValue("delivery_id"))
	if deliveryID == "" {
		api.writeError(w, r, http.StatusBadRequest, "delivery_id_required")
		return
	}

	var req webhookReplayRequest
	if err := decodeJSON(r, &req); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}
	replayToken := strings.TrimSpace(req.ReplayToken)
	if replayToken == "" {
		replayToken = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	}
	if replayToken == "" {
		api.writeError(w, r, http.StatusBadRequest, "replay_token_required")
		return
	}

	deliveryStore := repopg.NewWebhookDeliveryStore(api.db)
	if deliveryStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	record, err := deliveryStore.Get(r.Context(), projectID, deliveryID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "delivery_lookup_failed")
		return
	}

	replayStore := repopg.NewWebhookDeliveryReplayStore(api.db)
	if replayStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	now := time.Now().UTC()
	inserted, err := replayStore.Insert(r.Context(), record.ID, replayToken, now)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "replay_insert_failed")
		return
	}

	scheduled := false
	if inserted {
		record.Status = webhooks.DeliveryStatusPending
		record.NextAttemptAt = now
		record.LastError = ""
		record.UpdatedAt = now
		if _, err := deliveryStore.Update(r.Context(), record); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "replay_schedule_failed")
			return
		}
		scheduled = true
	}

	api.appendWebhookAudit(r, auditWebhookDeliveryReplay, identity.Subject, map[string]any{
		"project_id":      record.ProjectID,
		"delivery_id":     record.ID,
		"event_id":        record.EventID,
		"subscription_id": record.SubscriptionID,
		"scheduled":       scheduled,
	})

	api.writeJSON(w, http.StatusOK, map[string]any{
		"delivery_id": record.ID,
		"scheduled":   scheduled,
	})
}

func (api *experimentsAPI) appendWebhookAudit(r *http.Request, action, actor string, payload map[string]any) {
	if r == nil {
		return
	}
	api.appendWebhookAuditWithContext(r.Context(), r.Header.Get("X-Request-Id"), action, actor, payload)
}

func (api *experimentsAPI) appendWebhookAuditWithContext(ctx context.Context, requestID, action, actor string, payload map[string]any) {
	if api == nil || api.db == nil {
		return
	}
	auditAppender := repopg.NewAuditAppender(api.db, nil)
	if auditAppender == nil {
		return
	}
	_, _ = auditAppender.Append(ctx, domain.AuditEvent{
		OccurredAt:   time.Now().UTC(),
		Actor:        strings.TrimSpace(actor),
		Action:       action,
		ResourceType: "webhook",
		ResourceID:   webhookAuditResourceID(payload),
		RequestID:    strings.TrimSpace(requestID),
		Payload:      payload,
	})
}

func webhookAuditResourceID(payload map[string]any) string {
	if payload == nil {
		return "webhook"
	}
	if value, ok := payload["subscription_id"]; ok {
		if id, ok := value.(string); ok && strings.TrimSpace(id) != "" {
			return id
		}
	}
	if value, ok := payload["delivery_id"]; ok {
		if id, ok := value.(string); ok && strings.TrimSpace(id) != "" {
			return id
		}
	}
	return "webhook"
}

func webhookSubscriptionResponseFromRecord(record webhooks.Subscription) webhookSubscriptionResponse {
	return webhookSubscriptionResponse{
		ID:         record.ID,
		ProjectID:  record.ProjectID,
		Name:       record.Name,
		TargetURL:  record.TargetURL,
		Enabled:    record.Enabled,
		EventTypes: eventTypesToStrings(record.EventTypes),
		SecretRef:  record.SecretRef,
		Headers:    redaction.RedactMapString(record.Headers),
		CreatedAt:  record.CreatedAt,
		UpdatedAt:  record.UpdatedAt,
	}
}

func webhookDeliveryResponseFromRecord(record webhooks.Delivery) webhookDeliveryResponse {
	return webhookDeliveryResponse{
		ID:             record.ID,
		ProjectID:      record.ProjectID,
		SubscriptionID: record.SubscriptionID,
		EventID:        record.EventID,
		EventType:      record.EventType.String(),
		Status:         string(record.Status),
		NextAttemptAt:  record.NextAttemptAt,
		AttemptCount:   record.AttemptCount,
		LastError:      sanitizeWebhookError(record.LastError),
		CreatedAt:      record.CreatedAt,
		UpdatedAt:      record.UpdatedAt,
	}
}

func webhookDeliveryAttemptResponseFromRecord(record webhooks.Attempt) webhookDeliveryAttemptResponse {
	return webhookDeliveryAttemptResponse{
		ID:          record.ID,
		DeliveryID:  record.DeliveryID,
		AttemptedAt: record.AttemptedAt,
		StatusCode:  record.StatusCode,
		Outcome:     string(record.Outcome),
		Error:       sanitizeWebhookError(record.Error),
		LatencyMs:   record.LatencyMs,
		RequestID:   record.RequestID,
		CreatedAt:   record.CreatedAt,
	}
}

func eventTypesToStrings(input []webhooks.EventType) []string {
	if len(input) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(input))
	for _, value := range input {
		if !value.Valid() {
			continue
		}
		out = append(out, value.String())
	}
	return out
}

func normalizeWebhookHeaders(headers map[string]string) map[string]string {
	if headers == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(headers))
	for key, value := range headers {
		k := strings.TrimSpace(key)
		if k == "" {
			continue
		}
		out[k] = strings.TrimSpace(value)
	}
	return out
}

func sanitizeWebhookError(value string) string {
	clean := redaction.RedactString(strings.TrimSpace(value))
	if len(clean) > 500 {
		return clean[:500]
	}
	return clean
}
