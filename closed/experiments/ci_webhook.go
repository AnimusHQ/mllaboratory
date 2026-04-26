package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/google/uuid"
)

const (
	ciWebhookHeaderTimestamp = "X-Animus-CI-Ts"
	ciWebhookHeaderSignature = "X-Animus-CI-Sig"
)

func (api *experimentsAPI) handleCIWebhook(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	if strings.TrimSpace(api.ciWebhookSecret) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	ts := strings.TrimSpace(r.Header.Get(ciWebhookHeaderTimestamp))
	sig := strings.TrimSpace(r.Header.Get(ciWebhookHeaderSignature))
	if ts == "" || sig == "" {
		api.auditCIWebhookReject(r.Context(), identity, r, "", "missing_signature_headers")
		api.writeError(w, r, http.StatusUnauthorized, "ci_signature_required")
		return
	}

	if err := auth.VerifyInternalAuthTimestamp(ts, time.Now().UTC(), api.ciWebhookMaxSkew); err != nil {
		api.auditCIWebhookReject(r.Context(), identity, r, "", "invalid_signature_timestamp")
		api.writeError(w, r, http.StatusUnauthorized, "ci_signature_invalid")
		return
	}
	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		api.auditCIWebhookReject(r.Context(), identity, r, "", "invalid_signature_timestamp")
		api.writeError(w, r, http.StatusUnauthorized, "ci_signature_invalid")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		api.auditCIWebhookReject(r.Context(), identity, r, "", "body_read_failed")
		api.writeError(w, r, http.StatusBadRequest, "invalid_body")
		return
	}

	if err := verifyCIWebhookSignature(api.ciWebhookSecret, ts, r.Method, body, sig); err != nil {
		api.auditCIWebhookReject(r.Context(), identity, r, "", "invalid_signature")
		api.writeError(w, r, http.StatusUnauthorized, "ci_signature_invalid")
		return
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		api.auditCIWebhookReject(r.Context(), identity, r, "", "invalid_json")
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}

	runID, _ := payload["run_id"].(string)
	runID = strings.TrimSpace(runID)
	if runID == "" {
		api.auditCIWebhookReject(r.Context(), identity, r, "", "run_id_required")
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}

	provider, _ := payload["provider"].(string)
	provider = strings.TrimSpace(provider)

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	payloadSum := sha256.Sum256(payloadJSON)
	payloadSHA256 := hex.EncodeToString(payloadSum[:])

	runExists, err := api.runExists(r.Context(), runID)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if !runExists {
		api.auditCIWebhookReject(r.Context(), identity, r, runID, "run_not_found")
		api.writeError(w, r, http.StatusNotFound, "not_found")
		return
	}

	now := time.Now().UTC()
	contextID := uuid.NewString()

	type integrityInput struct {
		ContextID     string          `json:"context_id"`
		RunID         string          `json:"run_id"`
		ReceivedAt    time.Time       `json:"received_at"`
		ReceivedBy    string          `json:"received_by"`
		Provider      string          `json:"provider,omitempty"`
		SignatureTS   int64           `json:"signature_ts"`
		Signature     string          `json:"signature"`
		Payload       json.RawMessage `json:"payload"`
		PayloadSHA256 string          `json:"payload_sha256"`
	}
	integrity, err := integritySHA256(integrityInput{
		ContextID:     contextID,
		RunID:         runID,
		ReceivedAt:    now,
		ReceivedBy:    identity.Subject,
		Provider:      provider,
		SignatureTS:   tsInt,
		Signature:     sig,
		Payload:       payloadJSON,
		PayloadSHA256: payloadSHA256,
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	tx, err := api.db.BeginTx(r.Context(), nil)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback() }()

	var insertedID string
	err = tx.QueryRowContext(
		r.Context(),
		`INSERT INTO experiment_run_contexts (
			context_id,
			run_id,
			received_at,
			received_by,
			provider,
			signature_ts,
			signature,
			payload,
			payload_sha256,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (run_id, payload_sha256) DO NOTHING
		RETURNING context_id`,
		contextID,
		runID,
		now,
		identity.Subject,
		nullString(provider),
		tsInt,
		sig,
		payloadJSON,
		payloadSHA256,
		integrity,
	).Scan(&insertedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			existingID, fetchErr := api.getExistingRunContextID(r.Context(), tx, runID, payloadSHA256)
			if fetchErr != nil {
				api.writeError(w, r, http.StatusInternalServerError, "internal_error")
				return
			}

			_, auditErr := auditlog.Insert(r.Context(), tx, auditlog.Event{
				OccurredAt:   now,
				Actor:        identity.Subject,
				Action:       "ci_webhook.duplicate",
				ResourceType: "experiment_run_context",
				ResourceID:   existingID,
				RequestID:    r.Header.Get("X-Request-Id"),
				IP:           requestIP(r.RemoteAddr),
				UserAgent:    r.UserAgent(),
				Payload: map[string]any{
					"service":        "experiments",
					"run_id":         runID,
					"provider":       provider,
					"payload_sha256": payloadSHA256,
				},
			})
			if auditErr != nil {
				api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
				return
			}

			if err := tx.Commit(); err != nil {
				api.writeError(w, r, http.StatusInternalServerError, "internal_error")
				return
			}

			api.writeJSON(w, http.StatusOK, map[string]any{
				"status":         "duplicate",
				"context_id":     existingID,
				"run_id":         runID,
				"provider":       provider,
				"payload_sha256": payloadSHA256,
			})
			return
		}

		if isForeignKeyViolation(err) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	_, err = auditlog.Insert(r.Context(), tx, auditlog.Event{
		OccurredAt:   now,
		Actor:        identity.Subject,
		Action:       "experiment_run_context.create",
		ResourceType: "experiment_run_context",
		ResourceID:   insertedID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":        "experiments",
			"context_id":     insertedID,
			"run_id":         runID,
			"provider":       provider,
			"payload_sha256": payloadSHA256,
			"signature_ts":   tsInt,
		},
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
		return
	}

	if err := tx.Commit(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	w.Header().Set("Location", "/experiment-run-contexts/"+insertedID)
	api.writeJSON(w, http.StatusCreated, map[string]any{
		"context_id":     insertedID,
		"run_id":         runID,
		"provider":       provider,
		"payload_sha256": payloadSHA256,
		"received_at":    now,
		"received_by":    identity.Subject,
		"payload":        json.RawMessage(payloadJSON),
	})
}

func verifyCIWebhookSignature(secret string, ts string, method string, body []byte, signature string) error {
	expected, err := computeCIWebhookMAC(secret, ts, method, body)
	if err != nil {
		return err
	}
	got, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(signature))
	if err != nil {
		return errors.New("invalid signature encoding")
	}
	if !hmac.Equal(expected, got) {
		return errors.New("invalid signature")
	}
	return nil
}

func computeCIWebhookMAC(secret string, ts string, method string, body []byte) ([]byte, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, errors.New("webhook secret is required")
	}
	ts = strings.TrimSpace(ts)
	if ts == "" {
		return nil, errors.New("timestamp is required")
	}

	sum := sha256.Sum256(body)
	msg := strings.Join([]string{
		ts,
		strings.ToUpper(strings.TrimSpace(method)),
		hex.EncodeToString(sum[:]),
	}, "\n")

	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(msg)); err != nil {
		return nil, err
	}
	return mac.Sum(nil), nil
}

func (api *experimentsAPI) auditCIWebhookReject(ctx context.Context, identity auth.Identity, r *http.Request, runID string, reason string) {
	payload := map[string]any{
		"service": "experiments",
		"reason":  reason,
	}
	if strings.TrimSpace(runID) != "" {
		payload["run_id"] = runID
	}

	now := time.Now().UTC()
	_, _ = auditlog.Insert(ctx, api.db, auditlog.Event{
		OccurredAt:   now,
		Actor:        identity.Subject,
		Action:       "ci_webhook.reject",
		ResourceType: "ci_webhook",
		ResourceID:   r.Header.Get("X-Request-Id"),
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload:      payload,
	})
}

func (api *experimentsAPI) runExists(ctx context.Context, runID string) (bool, error) {
	var one int
	err := api.db.QueryRowContext(ctx, `SELECT 1 FROM experiment_runs WHERE run_id = $1`, runID).Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (api *experimentsAPI) getExistingRunContextID(ctx context.Context, q auditlog.QueryRower, runID string, payloadSHA256 string) (string, error) {
	var contextID string
	err := q.QueryRowContext(
		ctx,
		`SELECT context_id
		 FROM experiment_run_contexts
		 WHERE run_id = $1 AND payload_sha256 = $2
		 LIMIT 1`,
		runID,
		payloadSHA256,
	).Scan(&contextID)
	if err != nil {
		return "", err
	}
	return contextID, nil
}
