package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type modelImage struct {
	ImageDigest string          `json:"image_digest"`
	Repo        string          `json:"repo"`
	CommitSHA   string          `json:"commit_sha"`
	PipelineID  string          `json:"pipeline_id"`
	Provider    string          `json:"provider,omitempty"`
	ReceivedAt  time.Time       `json:"received_at"`
	ReceivedBy  string          `json:"received_by"`
	Payload     json.RawMessage `json:"payload,omitempty"`
}

func (api *experimentsAPI) handleListModelImages(w http.ResponseWriter, r *http.Request) {
	limit := clampInt(parseIntQuery(r, "limit", 100), 1, 500)
	repo := strings.TrimSpace(r.URL.Query().Get("repo"))
	commitSHA := strings.TrimSpace(r.URL.Query().Get("commit_sha"))

	rows, err := api.db.QueryContext(
		r.Context(),
		`SELECT image_digest,
				repo,
				commit_sha,
				pipeline_id,
				provider,
				received_at,
				received_by
		 FROM model_images
		 WHERE ($1 = '' OR repo = $1)
		   AND ($2 = '' OR commit_sha = $2)
		 ORDER BY received_at DESC
		 LIMIT $3`,
		repo,
		commitSHA,
		limit,
	)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer rows.Close()

	out := make([]modelImage, 0, limit)
	for rows.Next() {
		var (
			imageDigest string
			repoValue   string
			commitValue string
			pipelineID  string
			provider    sql.NullString
			receivedAt  time.Time
			receivedBy  string
		)
		if err := rows.Scan(&imageDigest, &repoValue, &commitValue, &pipelineID, &provider, &receivedAt, &receivedBy); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		out = append(out, modelImage{
			ImageDigest: imageDigest,
			Repo:        repoValue,
			CommitSHA:   commitValue,
			PipelineID:  pipelineID,
			Provider:    provider.String,
			ReceivedAt:  receivedAt,
			ReceivedBy:  receivedBy,
		})
	}
	if err := rows.Err(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, map[string]any{
		"model_images": out,
	})
}

func (api *experimentsAPI) handleGetModelImage(w http.ResponseWriter, r *http.Request) {
	imageDigest := strings.ToLower(strings.TrimSpace(r.PathValue("image_digest")))
	if imageDigest == "" {
		api.writeError(w, r, http.StatusBadRequest, "image_digest_required")
		return
	}
	if !isSHA256Digest(imageDigest) {
		api.writeError(w, r, http.StatusBadRequest, "image_digest_invalid")
		return
	}

	var (
		repoValue   string
		commitValue string
		pipelineID  string
		provider    sql.NullString
		receivedAt  time.Time
		receivedBy  string
		payload     []byte
	)
	err := api.db.QueryRowContext(
		r.Context(),
		`SELECT repo,
				commit_sha,
				pipeline_id,
				provider,
				received_at,
				received_by,
				payload
		 FROM model_images
		 WHERE image_digest = $1`,
		imageDigest,
	).Scan(&repoValue, &commitValue, &pipelineID, &provider, &receivedAt, &receivedBy, &payload)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, modelImage{
		ImageDigest: imageDigest,
		Repo:        repoValue,
		CommitSHA:   commitValue,
		PipelineID:  pipelineID,
		Provider:    provider.String,
		ReceivedAt:  receivedAt,
		ReceivedBy:  receivedBy,
		Payload:     normalizeJSON(payload),
	})
}
