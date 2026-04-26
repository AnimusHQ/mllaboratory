package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/platform/lineageevent"
	"github.com/animus-labs/animus-go/closed/internal/platform/redaction"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

type experimentRunArtifact struct {
	ArtifactID  string          `json:"artifact_id"`
	RunID       string          `json:"run_id"`
	Kind        string          `json:"kind"`
	Name        string          `json:"name,omitempty"`
	Filename    string          `json:"filename,omitempty"`
	ContentType string          `json:"content_type,omitempty"`
	ObjectKey   string          `json:"object_key"`
	SHA256      string          `json:"sha256"`
	SizeBytes   int64           `json:"size_bytes"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   time.Time       `json:"created_at"`
	CreatedBy   string          `json:"created_by"`
}

type createExperimentRunArtifactResponse struct {
	Artifact experimentRunArtifact `json:"artifact"`
}

func isAllowedArtifactKind(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "model", "preview", "log", "file":
		return true
	default:
		return false
	}
}

func sanitizeFilename(name string) string {
	base := path.Base(strings.TrimSpace(name))
	if base == "" || base == "." || base == "/" {
		return "artifact.bin"
	}
	return base
}

func (api *experimentsAPI) getRunArtifactPrefix(ctx context.Context, runID string) (prefix string, err error) {
	var experimentID string
	var artifactsPrefix sql.NullString
	err = api.db.QueryRowContext(
		ctx,
		`SELECT experiment_id, artifacts_prefix
		 FROM experiment_runs
		 WHERE run_id = $1`,
		runID,
	).Scan(&experimentID, &artifactsPrefix)
	if err != nil {
		return "", err
	}
	prefix = strings.TrimSpace(artifactsPrefix.String)
	if prefix == "" {
		prefix = fmt.Sprintf("experiments/%s/runs/%s", strings.TrimSpace(experimentID), strings.TrimSpace(runID))
	}
	prefix = strings.Trim(prefix, "/")
	return prefix, nil
}

func (api *experimentsAPI) handleCreateExperimentRunArtifact(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	runID := strings.TrimSpace(r.PathValue("run_id"))
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}

	prefix, err := api.getRunArtifactPrefix(r.Context(), runID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_multipart")
		return
	}
	if r.MultipartForm != nil {
		defer func() { _ = r.MultipartForm.RemoveAll() }()
	}

	kind := strings.TrimSpace(r.FormValue("kind"))
	if kind == "" {
		api.writeError(w, r, http.StatusBadRequest, "artifact_kind_required")
		return
	}
	if !isAllowedArtifactKind(kind) {
		api.writeError(w, r, http.StatusBadRequest, "artifact_kind_invalid")
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))

	metadata := map[string]any{}
	metadataRaw := strings.TrimSpace(r.FormValue("metadata"))
	if metadataRaw != "" {
		if err := json.Unmarshal([]byte(metadataRaw), &metadata); err != nil {
			api.writeError(w, r, http.StatusBadRequest, "invalid_metadata")
			return
		}
	}
	metadata = redaction.RedactMetadata(metadata)
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		api.writeError(w, r, http.StatusBadRequest, "file_required")
		return
	}
	defer file.Close()

	filename := sanitizeFilename(header.Filename)
	contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	artifactID := uuid.NewString()
	objectKey := fmt.Sprintf("%s/artifacts/%s/%s/%s", prefix, strings.ToLower(kind), artifactID, filename)

	hasher := sha256.New()
	counter := &countingWriter{}
	reader := io.TeeReader(file, io.MultiWriter(hasher, counter))

	size := header.Size
	if size <= 0 {
		size = -1
	}

	putCtx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	_, err = api.store.PutObject(
		putCtx,
		api.storeCfg.BucketArtifacts,
		objectKey,
		reader,
		size,
		minio.PutObjectOptions{ContentType: contentType},
	)
	cancel()
	if err != nil {
		api.writeError(w, r, http.StatusBadGateway, "artifact_store_failed")
		return
	}

	sha256Hex := hex.EncodeToString(hasher.Sum(nil))
	sizeBytes := counter.n
	if sizeBytes <= 0 && header.Size > 0 {
		sizeBytes = header.Size
	}

	now := time.Now().UTC()

	type integrityInput struct {
		ArtifactID  string          `json:"artifact_id"`
		RunID       string          `json:"run_id"`
		Kind        string          `json:"kind"`
		Name        string          `json:"name,omitempty"`
		Filename    string          `json:"filename,omitempty"`
		ContentType string          `json:"content_type,omitempty"`
		ObjectKey   string          `json:"object_key"`
		SHA256      string          `json:"sha256"`
		SizeBytes   int64           `json:"size_bytes"`
		Metadata    json.RawMessage `json:"metadata"`
		CreatedAt   time.Time       `json:"created_at"`
		CreatedBy   string          `json:"created_by"`
	}
	integrity, err := integritySHA256(integrityInput{
		ArtifactID:  artifactID,
		RunID:       runID,
		Kind:        kind,
		Name:        name,
		Filename:    filename,
		ContentType: contentType,
		ObjectKey:   objectKey,
		SHA256:      sha256Hex,
		SizeBytes:   sizeBytes,
		Metadata:    metadataJSON,
		CreatedAt:   now,
		CreatedBy:   identity.Subject,
	})
	if err != nil {
		_ = api.store.RemoveObject(r.Context(), api.storeCfg.BucketArtifacts, objectKey, minio.RemoveObjectOptions{})
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	tx, err := api.db.BeginTx(r.Context(), nil)
	if err != nil {
		_ = api.store.RemoveObject(r.Context(), api.storeCfg.BucketArtifacts, objectKey, minio.RemoveObjectOptions{})
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(
		r.Context(),
		`INSERT INTO experiment_run_artifacts (
			artifact_id,
			run_id,
			kind,
			name,
			filename,
			content_type,
			object_key,
			sha256,
			size_bytes,
			metadata,
			created_at,
			created_by,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		artifactID,
		runID,
		strings.ToLower(strings.TrimSpace(kind)),
		nullString(name),
		nullString(filename),
		nullString(contentType),
		objectKey,
		sha256Hex,
		sizeBytes,
		metadataJSON,
		now,
		identity.Subject,
		integrity,
	)
	if err != nil {
		_ = api.store.RemoveObject(r.Context(), api.storeCfg.BucketArtifacts, objectKey, minio.RemoveObjectOptions{})
		if isForeignKeyViolation(err) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	_, err = lineageevent.Insert(r.Context(), tx, lineageevent.Event{
		OccurredAt:  now,
		Actor:       identity.Subject,
		RequestID:   r.Header.Get("X-Request-Id"),
		SubjectType: "experiment_run",
		SubjectID:   runID,
		Predicate:   "produced",
		ObjectType:  "artifact",
		ObjectID:    artifactID,
		Metadata: map[string]any{
			"kind":         strings.ToLower(strings.TrimSpace(kind)),
			"name":         name,
			"filename":     filename,
			"content_type": contentType,
			"object_key":   objectKey,
			"sha256":       sha256Hex,
			"size_bytes":   sizeBytes,
			"metadata":     metadata,
		},
	})
	if err != nil {
		_ = api.store.RemoveObject(r.Context(), api.storeCfg.BucketArtifacts, objectKey, minio.RemoveObjectOptions{})
		api.writeError(w, r, http.StatusInternalServerError, "lineage_write_failed")
		return
	}

	_, err = auditlog.Insert(r.Context(), tx, auditlog.Event{
		OccurredAt:   now,
		Actor:        identity.Subject,
		Action:       "experiment_run_artifact.create",
		ResourceType: "experiment_run_artifact",
		ResourceID:   artifactID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":      "experiments",
			"artifact_id":  artifactID,
			"run_id":       runID,
			"kind":         strings.ToLower(strings.TrimSpace(kind)),
			"name":         name,
			"filename":     filename,
			"content_type": contentType,
			"object_key":   objectKey,
			"sha256":       sha256Hex,
			"size_bytes":   sizeBytes,
			"metadata":     metadata,
		},
	})
	if err != nil {
		_ = api.store.RemoveObject(r.Context(), api.storeCfg.BucketArtifacts, objectKey, minio.RemoveObjectOptions{})
		api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
		return
	}

	if err := tx.Commit(); err != nil {
		_ = api.store.RemoveObject(r.Context(), api.storeCfg.BucketArtifacts, objectKey, minio.RemoveObjectOptions{})
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/experiment-runs/%s/artifacts/%s", runID, artifactID))
	api.writeJSON(w, http.StatusCreated, createExperimentRunArtifactResponse{
		Artifact: experimentRunArtifact{
			ArtifactID:  artifactID,
			RunID:       runID,
			Kind:        strings.ToLower(strings.TrimSpace(kind)),
			Name:        name,
			Filename:    filename,
			ContentType: contentType,
			ObjectKey:   objectKey,
			SHA256:      sha256Hex,
			SizeBytes:   sizeBytes,
			Metadata:    normalizeJSON(metadataJSON),
			CreatedAt:   now,
			CreatedBy:   identity.Subject,
		},
	})
}

func (api *experimentsAPI) handleListExperimentRunArtifacts(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(r.PathValue("run_id"))
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}

	limit := clampInt(parseIntQuery(r, "limit", 100), 1, 500)
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	kindFilter := strings.ToLower(strings.TrimSpace(kind))
	if kindFilter != "" && !isAllowedArtifactKind(kindFilter) {
		api.writeError(w, r, http.StatusBadRequest, "artifact_kind_invalid")
		return
	}

	query := `SELECT artifact_id, kind, name, filename, content_type, object_key, sha256, size_bytes, metadata, created_at, created_by
		FROM experiment_run_artifacts
		WHERE run_id = $1`
	args := []any{runID}
	if kindFilter != "" {
		query += ` AND kind = $2`
		args = append(args, kindFilter)
	}
	query += ` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, limit)

	rows, err := api.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer rows.Close()

	out := make([]experimentRunArtifact, 0, limit)
	for rows.Next() {
		var (
			artifactID  string
			kind        string
			name        sql.NullString
			filename    sql.NullString
			contentType sql.NullString
			objectKey   string
			sha256Sum   string
			sizeBytes   int64
			metadata    []byte
			createdAt   time.Time
			createdBy   string
		)
		if err := rows.Scan(&artifactID, &kind, &name, &filename, &contentType, &objectKey, &sha256Sum, &sizeBytes, &metadata, &createdAt, &createdBy); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		out = append(out, experimentRunArtifact{
			ArtifactID:  artifactID,
			RunID:       runID,
			Kind:        kind,
			Name:        strings.TrimSpace(name.String),
			Filename:    strings.TrimSpace(filename.String),
			ContentType: strings.TrimSpace(contentType.String),
			ObjectKey:   objectKey,
			SHA256:      sha256Sum,
			SizeBytes:   sizeBytes,
			Metadata:    normalizeJSON(metadata),
			CreatedAt:   createdAt,
			CreatedBy:   createdBy,
		})
	}
	if err := rows.Err(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, map[string]any{"artifacts": out})
}

func (api *experimentsAPI) handleGetExperimentRunArtifact(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(r.PathValue("run_id"))
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}
	artifactID := strings.TrimSpace(r.PathValue("artifact_id"))
	if artifactID == "" {
		api.writeError(w, r, http.StatusBadRequest, "artifact_id_required")
		return
	}

	var (
		kind        string
		name        sql.NullString
		filename    sql.NullString
		contentType sql.NullString
		objectKey   string
		sha256Sum   string
		sizeBytes   int64
		metadata    []byte
		createdAt   time.Time
		createdBy   string
	)
	err := api.db.QueryRowContext(
		r.Context(),
		`SELECT kind, name, filename, content_type, object_key, sha256, size_bytes, metadata, created_at, created_by
		 FROM experiment_run_artifacts
		 WHERE run_id = $1 AND artifact_id = $2`,
		runID,
		artifactID,
	).Scan(&kind, &name, &filename, &contentType, &objectKey, &sha256Sum, &sizeBytes, &metadata, &createdAt, &createdBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, experimentRunArtifact{
		ArtifactID:  artifactID,
		RunID:       runID,
		Kind:        kind,
		Name:        strings.TrimSpace(name.String),
		Filename:    strings.TrimSpace(filename.String),
		ContentType: strings.TrimSpace(contentType.String),
		ObjectKey:   objectKey,
		SHA256:      sha256Sum,
		SizeBytes:   sizeBytes,
		Metadata:    normalizeJSON(metadata),
		CreatedAt:   createdAt,
		CreatedBy:   createdBy,
	})
}

func (api *experimentsAPI) handleDownloadExperimentRunArtifact(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(r.PathValue("run_id"))
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}
	artifactID := strings.TrimSpace(r.PathValue("artifact_id"))
	if artifactID == "" {
		api.writeError(w, r, http.StatusBadRequest, "artifact_id_required")
		return
	}

	var (
		filename    sql.NullString
		contentType sql.NullString
		objectKey   string
		sizeBytes   int64
	)
	err := api.db.QueryRowContext(
		r.Context(),
		`SELECT filename, content_type, object_key, size_bytes
		 FROM experiment_run_artifacts
		 WHERE run_id = $1 AND artifact_id = $2`,
		runID,
		artifactID,
	).Scan(&filename, &contentType, &objectKey, &sizeBytes)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	name := strings.TrimSpace(filename.String)
	if name == "" {
		name = "artifact.bin"
	}
	ct := strings.TrimSpace(contentType.String)
	if ct == "" {
		ct = "application/octet-stream"
	}

	obj, err := api.store.GetObject(r.Context(), api.storeCfg.BucketArtifacts, objectKey, minio.GetObjectOptions{})
	if err != nil {
		api.writeError(w, r, http.StatusBadGateway, "object_store_error")
		return
	}
	defer obj.Close()

	if _, err := obj.Stat(); err != nil {
		api.writeError(w, r, http.StatusNotFound, "not_found")
		return
	}

	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	if sizeBytes > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(sizeBytes, 10))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, obj)
}

type countingWriter struct {
	n int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.n += int64(len(p))
	return len(p), nil
}
