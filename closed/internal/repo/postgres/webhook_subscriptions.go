package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/integrations/webhooks"
)

type WebhookSubscriptionStore struct {
	db DB
}

const (
	insertWebhookSubscriptionQuery = `INSERT INTO webhook_subscriptions (
			id,
			project_id,
			name,
			target_url,
			enabled,
			event_types,
			secret_ref,
			headers_jsonb,
			created_at,
			updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, project_id, name, target_url, enabled, event_types, secret_ref, headers_jsonb, created_at, updated_at`
	updateWebhookSubscriptionQuery = `UPDATE webhook_subscriptions
		SET name = $3,
			target_url = $4,
			enabled = $5,
			event_types = $6,
			secret_ref = $7,
			headers_jsonb = $8,
			updated_at = $9
		WHERE project_id = $1 AND id = $2
		RETURNING id, project_id, name, target_url, enabled, event_types, secret_ref, headers_jsonb, created_at, updated_at`
	selectWebhookSubscriptionQuery = `SELECT id, project_id, name, target_url, enabled, event_types, secret_ref, headers_jsonb, created_at, updated_at
		FROM webhook_subscriptions
		WHERE project_id = $1 AND id = $2`
	listWebhookSubscriptionsQuery = `SELECT id, project_id, name, target_url, enabled, event_types, secret_ref, headers_jsonb, created_at, updated_at
		FROM webhook_subscriptions
		WHERE project_id = $1
		ORDER BY created_at ASC, id ASC
		LIMIT $2`
	listWebhookSubscriptionsForEventQuery = `SELECT id, project_id, name, target_url, enabled, event_types, secret_ref, headers_jsonb, created_at, updated_at
		FROM webhook_subscriptions
		WHERE project_id = $1 AND enabled = true AND $2 = ANY(event_types)
		ORDER BY created_at ASC, id ASC`
)

func NewWebhookSubscriptionStore(db DB) *WebhookSubscriptionStore {
	if db == nil {
		return nil
	}
	return &WebhookSubscriptionStore{db: db}
}

func (s *WebhookSubscriptionStore) Create(ctx context.Context, record webhooks.Subscription) (webhooks.Subscription, error) {
	if s == nil || s.db == nil {
		return webhooks.Subscription{}, fmt.Errorf("webhook subscription store not initialized")
	}
	record.ID = strings.TrimSpace(record.ID)
	record.ProjectID = strings.TrimSpace(record.ProjectID)
	record.Name = strings.TrimSpace(record.Name)
	record.TargetURL = strings.TrimSpace(record.TargetURL)
	record.SecretRef = strings.TrimSpace(record.SecretRef)
	if record.ID == "" || record.ProjectID == "" || record.Name == "" || record.TargetURL == "" {
		return webhooks.Subscription{}, fmt.Errorf("id, project_id, name, target_url are required")
	}
	eventTypes := normalizeEventTypes(record.EventTypes)
	if len(eventTypes) == 0 {
		return webhooks.Subscription{}, fmt.Errorf("event_types are required")
	}
	headersJSON, err := encodeHeaders(record.Headers)
	if err != nil {
		return webhooks.Subscription{}, err
	}
	createdAt := normalizeTime(record.CreatedAt)
	updatedAt := normalizeTime(record.UpdatedAt)
	row := s.db.QueryRowContext(
		ctx,
		insertWebhookSubscriptionQuery,
		record.ID,
		record.ProjectID,
		record.Name,
		record.TargetURL,
		record.Enabled,
		encodeEventTypes(eventTypes),
		nullString(record.SecretRef),
		headersJSON,
		createdAt,
		updatedAt,
	)
	return scanWebhookSubscription(row)
}

func (s *WebhookSubscriptionStore) Update(ctx context.Context, record webhooks.Subscription) (webhooks.Subscription, error) {
	if s == nil || s.db == nil {
		return webhooks.Subscription{}, fmt.Errorf("webhook subscription store not initialized")
	}
	record.ID = strings.TrimSpace(record.ID)
	record.ProjectID = strings.TrimSpace(record.ProjectID)
	record.Name = strings.TrimSpace(record.Name)
	record.TargetURL = strings.TrimSpace(record.TargetURL)
	record.SecretRef = strings.TrimSpace(record.SecretRef)
	if record.ID == "" || record.ProjectID == "" || record.Name == "" || record.TargetURL == "" {
		return webhooks.Subscription{}, fmt.Errorf("id, project_id, name, target_url are required")
	}
	eventTypes := normalizeEventTypes(record.EventTypes)
	if len(eventTypes) == 0 {
		return webhooks.Subscription{}, fmt.Errorf("event_types are required")
	}
	headersJSON, err := encodeHeaders(record.Headers)
	if err != nil {
		return webhooks.Subscription{}, err
	}
	updatedAt := normalizeTime(record.UpdatedAt)
	row := s.db.QueryRowContext(
		ctx,
		updateWebhookSubscriptionQuery,
		record.ProjectID,
		record.ID,
		record.Name,
		record.TargetURL,
		record.Enabled,
		encodeEventTypes(eventTypes),
		nullString(record.SecretRef),
		headersJSON,
		updatedAt,
	)
	return scanWebhookSubscription(row)
}

func (s *WebhookSubscriptionStore) Get(ctx context.Context, projectID, subscriptionID string) (webhooks.Subscription, error) {
	if s == nil || s.db == nil {
		return webhooks.Subscription{}, fmt.Errorf("webhook subscription store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	subscriptionID = strings.TrimSpace(subscriptionID)
	if projectID == "" || subscriptionID == "" {
		return webhooks.Subscription{}, fmt.Errorf("project_id and subscription_id are required")
	}
	row := s.db.QueryRowContext(ctx, selectWebhookSubscriptionQuery, projectID, subscriptionID)
	record, err := scanWebhookSubscription(row)
	if err != nil {
		return webhooks.Subscription{}, handleNotFound(err)
	}
	return record, nil
}

func (s *WebhookSubscriptionStore) List(ctx context.Context, projectID string, limit int) ([]webhooks.Subscription, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("webhook subscription store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, listWebhookSubscriptionsQuery, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]webhooks.Subscription, 0)
	for rows.Next() {
		record, err := scanWebhookSubscription(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *WebhookSubscriptionStore) ListEnabledByEvent(ctx context.Context, projectID string, eventType webhooks.EventType) ([]webhooks.Subscription, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("webhook subscription store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	if !eventType.Valid() {
		return nil, fmt.Errorf("event_type is required")
	}
	rows, err := s.db.QueryContext(ctx, listWebhookSubscriptionsForEventQuery, projectID, eventType.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]webhooks.Subscription, 0)
	for rows.Next() {
		record, err := scanWebhookSubscription(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func normalizeEventTypes(input []webhooks.EventType) []webhooks.EventType {
	if len(input) == 0 {
		return nil
	}
	seen := map[webhooks.EventType]struct{}{}
	out := make([]webhooks.EventType, 0, len(input))
	for _, value := range input {
		if !value.Valid() {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sortEventTypes(out)
	return out
}

func sortEventTypes(input []webhooks.EventType) {
	if len(input) == 0 {
		return
	}
	sort.Slice(input, func(i, j int) bool { return input[i] < input[j] })
}

func encodeEventTypes(input []webhooks.EventType) []string {
	out := make([]string, 0, len(input))
	for _, value := range input {
		if !value.Valid() {
			continue
		}
		out = append(out, value.String())
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func decodeEventTypes(arr []string) []webhooks.EventType {
	if len(arr) == 0 {
		return nil
	}
	out := make([]webhooks.EventType, 0, len(arr))
	for _, elem := range arr {
		et := webhooks.EventType(strings.TrimSpace(elem))
		if !et.Valid() {
			continue
		}
		out = append(out, et)
	}
	sortEventTypes(out)
	return out
}

func encodeHeaders(headers map[string]string) ([]byte, error) {
	if headers == nil {
		headers = map[string]string{}
	}
	return json.Marshal(headers)
}

func decodeHeaders(raw []byte) (map[string]string, error) {
	if len(raw) == 0 {
		return map[string]string{}, nil
	}
	var out map[string]string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]string{}
	}
	return out, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanWebhookSubscription(row rowScanner) (webhooks.Subscription, error) {
	var (
		record     webhooks.Subscription
		eventTypes []string
		secretRef  sql.NullString
		headersRaw []byte
		createdAt  time.Time
		updatedAt  time.Time
	)
	if err := row.Scan(&record.ID, &record.ProjectID, &record.Name, &record.TargetURL, &record.Enabled, &eventTypes, &secretRef, &headersRaw, &createdAt, &updatedAt); err != nil {
		return webhooks.Subscription{}, err
	}
	record.EventTypes = decodeEventTypes(eventTypes)
	record.SecretRef = strings.TrimSpace(secretRef.String)
	headers, err := decodeHeaders(headersRaw)
	if err != nil {
		return webhooks.Subscription{}, err
	}
	record.Headers = headers
	record.CreatedAt = createdAt.UTC()
	record.UpdatedAt = updatedAt.UTC()
	return record, nil
}
