package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
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
)

func (api *experimentsAPI) handleCIReport(w http.ResponseWriter, r *http.Request) {
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
		api.auditCIReportReject(r.Context(), identity, r, "", "missing_signature_headers")
		api.writeError(w, r, http.StatusUnauthorized, "ci_signature_required")
		return
	}
	if err := auth.VerifyInternalAuthTimestamp(ts, time.Now().UTC(), api.ciWebhookMaxSkew); err != nil {
		api.auditCIReportReject(r.Context(), identity, r, "", "invalid_signature_timestamp")
		api.writeError(w, r, http.StatusUnauthorized, "ci_signature_invalid")
		return
	}
	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		api.auditCIReportReject(r.Context(), identity, r, "", "invalid_signature_timestamp")
		api.writeError(w, r, http.StatusUnauthorized, "ci_signature_invalid")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		api.auditCIReportReject(r.Context(), identity, r, "", "body_read_failed")
		api.writeError(w, r, http.StatusBadRequest, "invalid_body")
		return
	}

	if err := verifyCIWebhookSignature(api.ciWebhookSecret, ts, r.Method, body, sig); err != nil {
		api.auditCIReportReject(r.Context(), identity, r, "", "invalid_signature")
		api.writeError(w, r, http.StatusUnauthorized, "ci_signature_invalid")
		return
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		api.auditCIReportReject(r.Context(), identity, r, "", "invalid_json")
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}

	imageDigest, _ := payload["image_digest"].(string)
	imageDigest = strings.ToLower(strings.TrimSpace(imageDigest))
	if imageDigest == "" {
		api.auditCIReportReject(r.Context(), identity, r, "", "image_digest_required")
		api.writeError(w, r, http.StatusBadRequest, "image_digest_required")
		return
	}
	if !isSHA256Digest(imageDigest) {
		api.auditCIReportReject(r.Context(), identity, r, imageDigest, "image_digest_invalid")
		api.writeError(w, r, http.StatusBadRequest, "image_digest_invalid")
		return
	}

	repo, _ := payload["repo"].(string)
	repo = strings.TrimSpace(repo)
	if repo == "" {
		api.auditCIReportReject(r.Context(), identity, r, imageDigest, "repo_required")
		api.writeError(w, r, http.StatusBadRequest, "repo_required")
		return
	}

	commitSHA, _ := payload["commit_sha"].(string)
	commitSHA = strings.TrimSpace(commitSHA)
	if commitSHA == "" {
		api.auditCIReportReject(r.Context(), identity, r, imageDigest, "commit_sha_required")
		api.writeError(w, r, http.StatusBadRequest, "commit_sha_required")
		return
	}

	pipelineID, _ := payload["pipeline_id"].(string)
	pipelineID = strings.TrimSpace(pipelineID)
	if pipelineID == "" {
		api.auditCIReportReject(r.Context(), identity, r, imageDigest, "pipeline_id_required")
		api.writeError(w, r, http.StatusBadRequest, "pipeline_id_required")
		return
	}

	provider, _ := payload["provider"].(string)
	provider = strings.TrimSpace(provider)

	payload["image_digest"] = imageDigest
	payload["repo"] = repo
	payload["commit_sha"] = commitSHA
	payload["pipeline_id"] = pipelineID
	if provider != "" {
		payload["provider"] = provider
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	payloadSum := sha256.Sum256(payloadJSON)
	payloadSHA256 := hex.EncodeToString(payloadSum[:])

	now := time.Now().UTC()

	type integrityInput struct {
		ImageDigest    string          `json:"image_digest"`
		Repo           string          `json:"repo"`
		CommitSHA      string          `json:"commit_sha"`
		PipelineID     string          `json:"pipeline_id"`
		Provider       string          `json:"provider,omitempty"`
		ReceivedAt     time.Time       `json:"received_at"`
		ReceivedBy     string          `json:"received_by"`
		SignatureTS    int64           `json:"signature_ts"`
		Signature      string          `json:"signature"`
		Payload        json.RawMessage `json:"payload"`
		PayloadSHA256  string          `json:"payload_sha256"`
		SignatureScope string          `json:"signature_scope"`
	}
	integrity, err := integritySHA256(integrityInput{
		ImageDigest:    imageDigest,
		Repo:           repo,
		CommitSHA:      commitSHA,
		PipelineID:     pipelineID,
		Provider:       provider,
		ReceivedAt:     now,
		ReceivedBy:     identity.Subject,
		SignatureTS:    tsInt,
		Signature:      sig,
		Payload:        payloadJSON,
		PayloadSHA256:  payloadSHA256,
		SignatureScope: "ci.report",
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

	var insertedDigest string
	err = tx.QueryRowContext(
		r.Context(),
		`INSERT INTO model_images (
			image_digest,
			repo,
			commit_sha,
			pipeline_id,
			provider,
			received_at,
			received_by,
			signature_ts,
			signature,
			payload,
			payload_sha256,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (image_digest) DO NOTHING
		RETURNING image_digest`,
		imageDigest,
		repo,
		commitSHA,
		pipelineID,
		nullString(provider),
		now,
		identity.Subject,
		tsInt,
		sig,
		payloadJSON,
		payloadSHA256,
		integrity,
	).Scan(&insertedDigest)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_, auditErr := auditlog.Insert(r.Context(), tx, auditlog.Event{
				OccurredAt:   now,
				Actor:        identity.Subject,
				Action:       "ci_report.duplicate",
				ResourceType: "model_image",
				ResourceID:   imageDigest,
				RequestID:    r.Header.Get("X-Request-Id"),
				IP:           requestIP(r.RemoteAddr),
				UserAgent:    r.UserAgent(),
				Payload: map[string]any{
					"service":        "experiments",
					"image_digest":   imageDigest,
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
				"image_digest":   imageDigest,
				"payload_sha256": payloadSHA256,
			})
			return
		}
		if isUniqueViolation(err) {
			api.writeError(w, r, http.StatusConflict, "model_image_exists")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	_, err = auditlog.Insert(r.Context(), tx, auditlog.Event{
		OccurredAt:   now,
		Actor:        identity.Subject,
		Action:       "ci_report.create",
		ResourceType: "model_image",
		ResourceID:   insertedDigest,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":        "experiments",
			"image_digest":   insertedDigest,
			"repo":           repo,
			"commit_sha":     commitSHA,
			"pipeline_id":    pipelineID,
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

	w.Header().Set("Location", "/model-images/"+insertedDigest)
	api.writeJSON(w, http.StatusCreated, map[string]any{
		"status":         "created",
		"image_digest":   insertedDigest,
		"repo":           repo,
		"commit_sha":     commitSHA,
		"pipeline_id":    pipelineID,
		"provider":       provider,
		"payload_sha256": payloadSHA256,
		"received_at":    now,
		"received_by":    identity.Subject,
	})
}

func (api *experimentsAPI) auditCIReportReject(ctx context.Context, identity auth.Identity, r *http.Request, imageDigest string, reason string) {
	payload := map[string]any{
		"service": "experiments",
		"reason":  reason,
	}
	imageDigest = strings.TrimSpace(imageDigest)
	if imageDigest != "" {
		payload["image_digest"] = imageDigest
	}

	now := time.Now().UTC()
	_, _ = auditlog.Insert(ctx, api.db, auditlog.Event{
		OccurredAt:   now,
		Actor:        identity.Subject,
		Action:       "ci_report.reject",
		ResourceType: "ci_report",
		ResourceID:   r.Header.Get("X-Request-Id"),
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload:      payload,
	})
}
