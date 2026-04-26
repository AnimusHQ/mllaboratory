package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/platform/lineageevent"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

const (
	evidenceBundleSchemaV1 = "animus.evidence_bundle.v1"
	evidenceSignatureAlg   = "hmac-sha256"
)

type evidenceBundle struct {
	BundleID        string    `json:"bundle_id"`
	RunID           string    `json:"run_id"`
	BundleObjectKey string    `json:"bundle_object_key"`
	ReportObjectKey string    `json:"report_object_key"`
	BundleSHA256    string    `json:"bundle_sha256"`
	BundleSizeBytes int64     `json:"bundle_size_bytes"`
	ReportSHA256    string    `json:"report_sha256"`
	ReportSizeBytes int64     `json:"report_size_bytes"`
	Signature       string    `json:"signature"`
	SignatureAlg    string    `json:"signature_alg"`
	CreatedAt       time.Time `json:"created_at"`
	CreatedBy       string    `json:"created_by"`
}

type evidenceBundleListResponse struct {
	Bundles []evidenceBundle `json:"bundles"`
}

type createEvidenceBundleResponse struct {
	Bundle evidenceBundle `json:"bundle"`
}

type evidenceLineageEvent struct {
	EventID     int64           `json:"event_id"`
	OccurredAt  time.Time       `json:"occurred_at"`
	Actor       string          `json:"actor"`
	RequestID   string          `json:"request_id,omitempty"`
	SubjectType string          `json:"subject_type"`
	SubjectID   string          `json:"subject_id"`
	Predicate   string          `json:"predicate"`
	ObjectType  string          `json:"object_type"`
	ObjectID    string          `json:"object_id"`
	Metadata    json.RawMessage `json:"metadata"`
}

type evidenceAuditEvent struct {
	EventID         int64           `json:"event_id"`
	OccurredAt      time.Time       `json:"occurred_at"`
	Actor           string          `json:"actor"`
	Action          string          `json:"action"`
	ResourceType    string          `json:"resource_type"`
	ResourceID      string          `json:"resource_id"`
	RequestID       string          `json:"request_id,omitempty"`
	IP              string          `json:"ip,omitempty"`
	UserAgent       string          `json:"user_agent,omitempty"`
	Payload         json.RawMessage `json:"payload"`
	IntegritySHA256 string          `json:"integrity_sha256"`
}

type evidencePolicySnapshot struct {
	Decisions []policyDecisionDetail  `json:"decisions"`
	Approvals []policyApprovalDetail  `json:"approvals"`
	Versions  []evidencePolicyVersion `json:"versions"`
}

type evidencePolicyVersion struct {
	PolicyVersionID   string          `json:"policy_version_id"`
	PolicyID          string          `json:"policy_id"`
	PolicyName        string          `json:"policy_name"`
	PolicyDescription string          `json:"policy_description,omitempty"`
	Version           int             `json:"version"`
	Status            string          `json:"status"`
	SpecYAML          string          `json:"spec_yaml"`
	Spec              json.RawMessage `json:"spec"`
	SpecSHA256        string          `json:"spec_sha256"`
	CreatedAt         time.Time       `json:"created_at"`
	CreatedBy         string          `json:"created_by"`
}

type evidenceBundleManifest struct {
	Schema    string                 `json:"schema"`
	BundleID  string                 `json:"bundle_id"`
	RunID     string                 `json:"run_id"`
	CreatedAt time.Time              `json:"created_at"`
	CreatedBy string                 `json:"created_by"`
	Files     []evidenceManifestFile `json:"files"`
}

type evidenceManifestFile struct {
	Name        string `json:"name"`
	SHA256      string `json:"sha256"`
	SizeBytes   int64  `json:"size_bytes"`
	ContentType string `json:"content_type"`
}

type evidenceBundleFile struct {
	Name        string
	ContentType string
	Data        []byte
}

type evidenceBundleIntegrityInput struct {
	BundleID        string    `json:"bundle_id"`
	RunID           string    `json:"run_id"`
	BundleObjectKey string    `json:"bundle_object_key"`
	ReportObjectKey string    `json:"report_object_key"`
	BundleSHA256    string    `json:"bundle_sha256"`
	BundleSizeBytes int64     `json:"bundle_size_bytes"`
	ReportSHA256    string    `json:"report_sha256"`
	ReportSizeBytes int64     `json:"report_size_bytes"`
	Signature       string    `json:"signature"`
	SignatureAlg    string    `json:"signature_alg"`
	CreatedAt       time.Time `json:"created_at"`
	CreatedBy       string    `json:"created_by"`
}

func (api *experimentsAPI) handleCreateEvidenceBundle(w http.ResponseWriter, r *http.Request) {
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

	bundle, err := api.createEvidenceBundle(r.Context(), runID, identity, r)
	if err != nil {
		if errors.Is(err, errEvidenceStoreFailed) {
			api.writeError(w, r, http.StatusBadGateway, "object_store_error")
			return
		}
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, errEvidenceLedgerMissing) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.logger.Error("evidence bundle create failed", "error", err, "run_id", runID)
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusCreated, createEvidenceBundleResponse{Bundle: bundle})
}

func (api *experimentsAPI) handleListEvidenceBundles(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(r.PathValue("run_id"))
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}
	limit := clampInt(parseIntQuery(r, "limit", 100), 1, 500)

	rows, err := api.db.QueryContext(
		r.Context(),
		`SELECT bundle_id,
				bundle_object_key,
				report_object_key,
				bundle_sha256,
				bundle_size_bytes,
				report_sha256,
				report_size_bytes,
				signature,
				signature_alg,
				created_at,
				created_by
		 FROM experiment_run_evidence_bundles
		 WHERE run_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`,
		runID,
		limit,
	)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer rows.Close()

	out := make([]evidenceBundle, 0, limit)
	for rows.Next() {
		var bundle evidenceBundle
		if err := rows.Scan(
			&bundle.BundleID,
			&bundle.BundleObjectKey,
			&bundle.ReportObjectKey,
			&bundle.BundleSHA256,
			&bundle.BundleSizeBytes,
			&bundle.ReportSHA256,
			&bundle.ReportSizeBytes,
			&bundle.Signature,
			&bundle.SignatureAlg,
			&bundle.CreatedAt,
			&bundle.CreatedBy,
		); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		bundle.RunID = runID
		out = append(out, bundle)
	}
	if err := rows.Err(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, evidenceBundleListResponse{Bundles: out})
}

func (api *experimentsAPI) handleGetEvidenceBundle(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(r.PathValue("run_id"))
	bundleID := strings.TrimSpace(r.PathValue("bundle_id"))
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}
	if bundleID == "" {
		api.writeError(w, r, http.StatusBadRequest, "bundle_id_required")
		return
	}

	bundle, err := api.getEvidenceBundle(r.Context(), runID, bundleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, bundle)
}

func (api *experimentsAPI) handleDownloadEvidenceBundle(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(r.PathValue("run_id"))
	bundleID := strings.TrimSpace(r.PathValue("bundle_id"))
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}
	if bundleID == "" {
		api.writeError(w, r, http.StatusBadRequest, "bundle_id_required")
		return
	}

	bundle, err := api.getEvidenceBundle(r.Context(), runID, bundleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	obj, err := api.store.GetObject(r.Context(), api.storeCfg.BucketArtifacts, bundle.BundleObjectKey, minio.GetObjectOptions{})
	if err != nil {
		api.writeError(w, r, http.StatusBadGateway, "object_store_error")
		return
	}
	defer obj.Close()

	if _, err := obj.Stat(); err != nil {
		api.writeError(w, r, http.StatusNotFound, "not_found")
		return
	}

	filename := fmt.Sprintf("evidence-%s.zip", bundle.RunID)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	if bundle.BundleSizeBytes > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(bundle.BundleSizeBytes, 10))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, obj)
}

func (api *experimentsAPI) handleDownloadEvidenceReport(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(r.PathValue("run_id"))
	bundleID := strings.TrimSpace(r.PathValue("bundle_id"))
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}
	if bundleID == "" {
		api.writeError(w, r, http.StatusBadRequest, "bundle_id_required")
		return
	}

	bundle, err := api.getEvidenceBundle(r.Context(), runID, bundleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	obj, err := api.store.GetObject(r.Context(), api.storeCfg.BucketArtifacts, bundle.ReportObjectKey, minio.GetObjectOptions{})
	if err != nil {
		api.writeError(w, r, http.StatusBadGateway, "object_store_error")
		return
	}
	defer obj.Close()

	if _, err := obj.Stat(); err != nil {
		api.writeError(w, r, http.StatusNotFound, "not_found")
		return
	}

	filename := fmt.Sprintf("evidence-report-%s.pdf", bundle.RunID)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	if bundle.ReportSizeBytes > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(bundle.ReportSizeBytes, 10))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, obj)
}

func (api *experimentsAPI) getEvidenceBundle(ctx context.Context, runID string, bundleID string) (evidenceBundle, error) {
	var bundle evidenceBundle
	err := api.db.QueryRowContext(
		ctx,
		`SELECT bundle_id,
				bundle_object_key,
				report_object_key,
				bundle_sha256,
				bundle_size_bytes,
				report_sha256,
				report_size_bytes,
				signature,
				signature_alg,
				created_at,
				created_by
		 FROM experiment_run_evidence_bundles
		 WHERE run_id = $1 AND bundle_id = $2`,
		runID,
		bundleID,
	).Scan(
		&bundle.BundleID,
		&bundle.BundleObjectKey,
		&bundle.ReportObjectKey,
		&bundle.BundleSHA256,
		&bundle.BundleSizeBytes,
		&bundle.ReportSHA256,
		&bundle.ReportSizeBytes,
		&bundle.Signature,
		&bundle.SignatureAlg,
		&bundle.CreatedAt,
		&bundle.CreatedBy,
	)
	if err != nil {
		return evidenceBundle{}, err
	}
	bundle.RunID = runID
	return bundle, nil
}

var errEvidenceLedgerMissing = errors.New("execution ledger missing")
var errEvidenceStoreFailed = errors.New("evidence object store failure")

func (api *experimentsAPI) createEvidenceBundle(ctx context.Context, runID string, identity auth.Identity, r *http.Request) (evidenceBundle, error) {
	prefix, err := api.getRunArtifactPrefix(ctx, runID)
	if err != nil {
		return evidenceBundle{}, err
	}

	ledgerRecord, ledgerEntry, err := api.fetchEvidenceLedger(ctx, runID)
	if err != nil {
		return evidenceBundle{}, err
	}

	lineageEvents, err := fetchEvidenceLineage(ctx, api.db, runID)
	if err != nil {
		return evidenceBundle{}, err
	}

	policySnapshot, decisionIDs, approvalIDs, err := fetchEvidencePolicies(ctx, api.db, runID)
	if err != nil {
		return evidenceBundle{}, err
	}

	auditEvents, err := fetchEvidenceAudit(ctx, api.db, evidenceAuditInput{
		RunID:            runID,
		ExecutionID:      ledgerEntry.ExecutionID,
		DatasetVersionID: ledgerEntry.Dataset.VersionID,
		DecisionIDs:      decisionIDs,
		ApprovalIDs:      approvalIDs,
	})
	if err != nil {
		return evidenceBundle{}, err
	}

	ledgerPayload, err := json.MarshalIndent(ledgerRecord, "", "  ")
	if err != nil {
		return evidenceBundle{}, err
	}
	lineagePayload, err := json.MarshalIndent(map[string]any{"events": lineageEvents}, "", "  ")
	if err != nil {
		return evidenceBundle{}, err
	}
	auditPayload, err := json.MarshalIndent(map[string]any{"events": auditEvents}, "", "  ")
	if err != nil {
		return evidenceBundle{}, err
	}
	policyPayload, err := json.MarshalIndent(policySnapshot, "", "  ")
	if err != nil {
		return evidenceBundle{}, err
	}

	bundleID := uuid.NewString()
	createdAt := time.Now().UTC()

	executionHash := ""
	if len(ledgerRecord.Entries) > 0 {
		executionHash = ledgerRecord.Entries[0].ExecutionHash
	}

	reportInput := evidenceReportInput{
		RunID:            runID,
		ExecutionID:      ledgerEntry.ExecutionID,
		ExperimentID:     ledgerEntry.ExperimentID,
		DatasetID:        ledgerEntry.Dataset.DatasetID,
		DatasetVersionID: ledgerEntry.Dataset.VersionID,
		DatasetSHA256:    ledgerEntry.Dataset.SHA256,
		GitRepo:          ledgerEntry.Git.Repo,
		GitCommit:        ledgerEntry.Git.Commit,
		GitRef:           ledgerEntry.Git.Ref,
		ImageRef:         ledgerEntry.Image.Ref,
		ImageDigest:      ledgerEntry.Image.Digest,
		ExecutionHash:    executionHash,
		PolicyDecisions:  policySnapshot.Decisions,
		PolicyApprovals:  policySnapshot.Approvals,
		GeneratedAt:      createdAt,
		GeneratedBy:      strings.TrimSpace(identity.Subject),
	}
	reportPDF, err := buildComplianceReportPDF(reportInput)
	if err != nil {
		return evidenceBundle{}, err
	}

	files := []evidenceBundleFile{
		{Name: "ledger.json", ContentType: "application/json", Data: ledgerPayload},
		{Name: "lineage.json", ContentType: "application/json", Data: lineagePayload},
		{Name: "audit.json", ContentType: "application/json", Data: auditPayload},
		{Name: "policies.json", ContentType: "application/json", Data: policyPayload},
		{Name: "report.pdf", ContentType: "application/pdf", Data: reportPDF},
	}

	manifestPayload, err := buildEvidenceManifest(bundleID, runID, createdAt, identity.Subject, files)
	if err != nil {
		return evidenceBundle{}, err
	}
	files = append(files, evidenceBundleFile{Name: "manifest.json", ContentType: "application/json", Data: manifestPayload})

	bundleData, bundleSHA, bundleSize, err := buildEvidenceZip(createdAt, files)
	if err != nil {
		return evidenceBundle{}, err
	}

	signature, err := computeEvidenceSignature(api.evidenceSigningSecret, bundleSHA)
	if err != nil {
		return evidenceBundle{}, err
	}

	reportSHA := sha256HexBytes(reportPDF)
	reportSize := int64(len(reportPDF))

	bundlePrefix := fmt.Sprintf("%s/evidence/%s", prefix, bundleID)
	bundleObjectKey := fmt.Sprintf("%s/bundle.zip", bundlePrefix)
	reportObjectKey := fmt.Sprintf("%s/report.pdf", bundlePrefix)

	putCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	_, err = api.store.PutObject(
		putCtx,
		api.storeCfg.BucketArtifacts,
		bundleObjectKey,
		bytes.NewReader(bundleData),
		bundleSize,
		minio.PutObjectOptions{ContentType: "application/zip"},
	)
	cancel()
	if err != nil {
		return evidenceBundle{}, fmt.Errorf("%w: %s", errEvidenceStoreFailed, err)
	}

	putCtx, cancel = context.WithTimeout(ctx, 2*time.Minute)
	_, err = api.store.PutObject(
		putCtx,
		api.storeCfg.BucketArtifacts,
		reportObjectKey,
		bytes.NewReader(reportPDF),
		reportSize,
		minio.PutObjectOptions{ContentType: "application/pdf"},
	)
	cancel()
	if err != nil {
		_ = api.store.RemoveObject(ctx, api.storeCfg.BucketArtifacts, bundleObjectKey, minio.RemoveObjectOptions{})
		return evidenceBundle{}, fmt.Errorf("%w: %s", errEvidenceStoreFailed, err)
	}

	integrity, err := integritySHA256(evidenceBundleIntegrityInput{
		BundleID:        bundleID,
		RunID:           runID,
		BundleObjectKey: bundleObjectKey,
		ReportObjectKey: reportObjectKey,
		BundleSHA256:    bundleSHA,
		BundleSizeBytes: bundleSize,
		ReportSHA256:    reportSHA,
		ReportSizeBytes: reportSize,
		Signature:       signature,
		SignatureAlg:    evidenceSignatureAlg,
		CreatedAt:       createdAt,
		CreatedBy:       strings.TrimSpace(identity.Subject),
	})
	if err != nil {
		_ = api.store.RemoveObject(ctx, api.storeCfg.BucketArtifacts, bundleObjectKey, minio.RemoveObjectOptions{})
		_ = api.store.RemoveObject(ctx, api.storeCfg.BucketArtifacts, reportObjectKey, minio.RemoveObjectOptions{})
		return evidenceBundle{}, err
	}

	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		_ = api.store.RemoveObject(ctx, api.storeCfg.BucketArtifacts, bundleObjectKey, minio.RemoveObjectOptions{})
		_ = api.store.RemoveObject(ctx, api.storeCfg.BucketArtifacts, reportObjectKey, minio.RemoveObjectOptions{})
		return evidenceBundle{}, err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO experiment_run_evidence_bundles (
			bundle_id,
			run_id,
			bundle_object_key,
			report_object_key,
			bundle_sha256,
			bundle_size_bytes,
			report_sha256,
			report_size_bytes,
			signature,
			signature_alg,
			created_at,
			created_by,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		bundleID,
		runID,
		bundleObjectKey,
		reportObjectKey,
		bundleSHA,
		bundleSize,
		reportSHA,
		reportSize,
		signature,
		evidenceSignatureAlg,
		createdAt,
		strings.TrimSpace(identity.Subject),
		integrity,
	)
	if err != nil {
		_ = api.store.RemoveObject(ctx, api.storeCfg.BucketArtifacts, bundleObjectKey, minio.RemoveObjectOptions{})
		_ = api.store.RemoveObject(ctx, api.storeCfg.BucketArtifacts, reportObjectKey, minio.RemoveObjectOptions{})
		return evidenceBundle{}, err
	}

	_, err = lineageevent.Insert(ctx, tx, lineageevent.Event{
		OccurredAt:  createdAt,
		Actor:       identity.Subject,
		RequestID:   r.Header.Get("X-Request-Id"),
		SubjectType: "experiment_run",
		SubjectID:   runID,
		Predicate:   "produced",
		ObjectType:  "evidence_bundle",
		ObjectID:    bundleID,
		Metadata: map[string]any{
			"bundle_sha256": bundleSHA,
			"bundle_size":   bundleSize,
			"signature_alg": evidenceSignatureAlg,
		},
	})
	if err != nil {
		_ = api.store.RemoveObject(ctx, api.storeCfg.BucketArtifacts, bundleObjectKey, minio.RemoveObjectOptions{})
		_ = api.store.RemoveObject(ctx, api.storeCfg.BucketArtifacts, reportObjectKey, minio.RemoveObjectOptions{})
		return evidenceBundle{}, err
	}

	_, err = auditlog.Insert(ctx, tx, auditlog.Event{
		OccurredAt:   createdAt,
		Actor:        identity.Subject,
		Action:       "evidence_bundle.create",
		ResourceType: "evidence_bundle",
		ResourceID:   bundleID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":           "experiments",
			"run_id":            runID,
			"bundle_object_key": bundleObjectKey,
			"report_object_key": reportObjectKey,
			"bundle_sha256":     bundleSHA,
			"bundle_size_bytes": bundleSize,
			"report_sha256":     reportSHA,
			"report_size_bytes": reportSize,
			"signature_alg":     evidenceSignatureAlg,
		},
	})
	if err != nil {
		_ = api.store.RemoveObject(ctx, api.storeCfg.BucketArtifacts, bundleObjectKey, minio.RemoveObjectOptions{})
		_ = api.store.RemoveObject(ctx, api.storeCfg.BucketArtifacts, reportObjectKey, minio.RemoveObjectOptions{})
		return evidenceBundle{}, err
	}

	if err := tx.Commit(); err != nil {
		_ = api.store.RemoveObject(ctx, api.storeCfg.BucketArtifacts, bundleObjectKey, minio.RemoveObjectOptions{})
		_ = api.store.RemoveObject(ctx, api.storeCfg.BucketArtifacts, reportObjectKey, minio.RemoveObjectOptions{})
		return evidenceBundle{}, err
	}

	return evidenceBundle{
		BundleID:        bundleID,
		RunID:           runID,
		BundleObjectKey: bundleObjectKey,
		ReportObjectKey: reportObjectKey,
		BundleSHA256:    bundleSHA,
		BundleSizeBytes: bundleSize,
		ReportSHA256:    reportSHA,
		ReportSizeBytes: reportSize,
		Signature:       signature,
		SignatureAlg:    evidenceSignatureAlg,
		CreatedAt:       createdAt,
		CreatedBy:       strings.TrimSpace(identity.Subject),
	}, nil
}

func (api *experimentsAPI) fetchEvidenceLedger(ctx context.Context, runID string) (executionLedgerExportResponse, executionLedgerEntry, error) {
	var (
		record    executionLedgerRecord
		entryRaw  []byte
		replayRaw []byte
	)
	err := api.db.QueryRowContext(
		ctx,
		`SELECT ledger_id,
				run_id,
				execution_id,
				execution_hash,
				entry,
				entry_sha256,
				replay_bundle,
				created_at,
				created_by
		 FROM execution_ledger_entries
		 WHERE run_id = $1`,
		runID,
	).Scan(
		&record.LedgerID,
		&record.RunID,
		&record.ExecutionID,
		&record.ExecutionHash,
		&entryRaw,
		&record.EntrySHA256,
		&replayRaw,
		&record.CreatedAt,
		&record.CreatedBy,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return executionLedgerExportResponse{}, executionLedgerEntry{}, errEvidenceLedgerMissing
		}
		return executionLedgerExportResponse{}, executionLedgerEntry{}, err
	}

	record.Entry = normalizeJSON(entryRaw)
	record.ReplayBundle = normalizeJSON(replayRaw)

	checksum, err := executionLedgerChecksum([]executionLedgerRecord{record})
	if err != nil {
		return executionLedgerExportResponse{}, executionLedgerEntry{}, err
	}
	recordChecksum := executionLedgerExportResponse{
		Entries:  []executionLedgerRecord{record},
		Checksum: checksum,
	}

	var entry executionLedgerEntry
	if err := json.Unmarshal(record.Entry, &entry); err != nil {
		return executionLedgerExportResponse{}, executionLedgerEntry{}, err
	}

	return recordChecksum, entry, nil
}

func fetchEvidenceLineage(ctx context.Context, db *sql.DB, runID string) ([]evidenceLineageEvent, error) {
	rows, err := db.QueryContext(
		ctx,
		`SELECT event_id, occurred_at, actor, request_id, subject_type, subject_id, predicate, object_type, object_id, metadata
		 FROM lineage_events
		 WHERE (subject_type = 'experiment_run' AND subject_id = $1)
		    OR (object_type = 'experiment_run' AND object_id = $1)
		 ORDER BY event_id ASC`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]evidenceLineageEvent, 0)
	for rows.Next() {
		var (
			ev        evidenceLineageEvent
			requestID sql.NullString
			metadata  []byte
		)
		if err := rows.Scan(
			&ev.EventID,
			&ev.OccurredAt,
			&ev.Actor,
			&requestID,
			&ev.SubjectType,
			&ev.SubjectID,
			&ev.Predicate,
			&ev.ObjectType,
			&ev.ObjectID,
			&metadata,
		); err != nil {
			return nil, err
		}
		ev.RequestID = strings.TrimSpace(requestID.String)
		ev.Metadata = normalizeJSON(metadata)
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

type evidenceAuditInput struct {
	RunID            string
	ExecutionID      string
	DatasetVersionID string
	DecisionIDs      []string
	ApprovalIDs      []string
}

func fetchEvidenceAudit(ctx context.Context, db *sql.DB, input evidenceAuditInput) ([]evidenceAuditEvent, error) {
	ids := []string{input.RunID, input.ExecutionID, input.DatasetVersionID}
	ids = append(ids, input.DecisionIDs...)
	ids = append(ids, input.ApprovalIDs...)
	ids = uniqueNonEmpty(ids)
	if len(ids) == 0 {
		return []evidenceAuditEvent{}, nil
	}

	resourceTypes := []string{
		"experiment_run",
		"experiment_run_execution",
		"dataset_version",
		"policy_decision",
		"policy_approval",
		"evidence_bundle",
	}

	args := []any{}
	idClause := buildInClause(len(ids), len(args)+1)
	for _, id := range ids {
		args = append(args, id)
	}
	typeClause := buildInClause(len(resourceTypes), len(args)+1)
	for _, rt := range resourceTypes {
		args = append(args, rt)
	}

	query := fmt.Sprintf(`SELECT event_id, occurred_at, actor, action, resource_type, resource_id, request_id, ip, user_agent, payload, integrity_sha256
		FROM audit_events
		WHERE resource_id IN (%s) AND resource_type IN (%s)
		ORDER BY event_id DESC`, idClause, typeClause)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]evidenceAuditEvent, 0)
	for rows.Next() {
		var (
			ev         evidenceAuditEvent
			requestID  sql.NullString
			ip         sql.NullString
			userAgent  sql.NullString
			payloadRaw []byte
		)
		if err := rows.Scan(
			&ev.EventID,
			&ev.OccurredAt,
			&ev.Actor,
			&ev.Action,
			&ev.ResourceType,
			&ev.ResourceID,
			&requestID,
			&ip,
			&userAgent,
			&payloadRaw,
			&ev.IntegritySHA256,
		); err != nil {
			return nil, err
		}
		ev.RequestID = strings.TrimSpace(requestID.String)
		ev.IP = strings.TrimSpace(ip.String)
		ev.UserAgent = strings.TrimSpace(userAgent.String)
		ev.Payload = normalizeJSON(payloadRaw)
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func fetchEvidencePolicies(ctx context.Context, db *sql.DB, runID string) (evidencePolicySnapshot, []string, []string, error) {
	decisions, decisionIDs, err := fetchEvidenceDecisions(ctx, db, runID)
	if err != nil {
		return evidencePolicySnapshot{}, nil, nil, err
	}
	approvals, approvalIDs, err := fetchEvidenceApprovals(ctx, db, runID)
	if err != nil {
		return evidencePolicySnapshot{}, nil, nil, err
	}
	versions, err := fetchEvidencePolicyVersions(ctx, db, decisions)
	if err != nil {
		return evidencePolicySnapshot{}, nil, nil, err
	}
	return evidencePolicySnapshot{
		Decisions: decisions,
		Approvals: approvals,
		Versions:  versions,
	}, decisionIDs, approvalIDs, nil
}

func fetchEvidenceDecisions(ctx context.Context, db *sql.DB, runID string) ([]policyDecisionDetail, []string, error) {
	rows, err := db.QueryContext(
		ctx,
		`SELECT d.decision_id,
				d.run_id,
				d.policy_id,
				p.name,
				d.policy_version_id,
				d.policy_sha256,
				d.context_sha256,
				d.context,
				d.decision,
				d.rule_id,
				d.reason,
				d.created_at,
				d.created_by
		 FROM policy_decisions d
		 JOIN policies p ON p.policy_id = d.policy_id
		 WHERE d.run_id = $1
		 ORDER BY d.created_at DESC`,
		runID,
	)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	out := []policyDecisionDetail{}
	ids := []string{}
	for rows.Next() {
		var (
			decisionID string
			runIDVal   sql.NullString
			policyID   string
			policyName string
			versionID  string
			policySHA  string
			contextSHA string
			contextRaw []byte
			decision   string
			ruleID     sql.NullString
			reason     sql.NullString
			createdAt  time.Time
			createdBy  string
		)
		if err := rows.Scan(
			&decisionID,
			&runIDVal,
			&policyID,
			&policyName,
			&versionID,
			&policySHA,
			&contextSHA,
			&contextRaw,
			&decision,
			&ruleID,
			&reason,
			&createdAt,
			&createdBy,
		); err != nil {
			return nil, nil, err
		}
		out = append(out, policyDecisionDetail{
			policyDecisionSummary: policyDecisionSummary{
				DecisionID:      decisionID,
				RunID:           strings.TrimSpace(runIDVal.String),
				PolicyID:        policyID,
				PolicyName:      policyName,
				PolicyVersionID: versionID,
				PolicySHA256:    policySHA,
				ContextSHA256:   contextSHA,
				Decision:        decision,
				RuleID:          strings.TrimSpace(ruleID.String),
				Reason:          strings.TrimSpace(reason.String),
				CreatedAt:       createdAt,
				CreatedBy:       createdBy,
			},
			Context: normalizeJSON(contextRaw),
		})
		ids = append(ids, decisionID)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return out, uniqueNonEmpty(ids), nil
}

func fetchEvidenceApprovals(ctx context.Context, db *sql.DB, runID string) ([]policyApprovalDetail, []string, error) {
	rows, err := db.QueryContext(
		ctx,
		`SELECT a.approval_id,
				a.decision_id,
				a.run_id,
				a.status,
				a.requested_at,
				a.requested_by,
				a.decided_at,
				a.decided_by,
				a.reason,
				d.policy_id,
				p.name,
				d.policy_version_id,
				d.decision,
				d.rule_id,
				d.context
		 FROM policy_approvals a
		 JOIN policy_decisions d ON d.decision_id = a.decision_id
		 JOIN policies p ON p.policy_id = d.policy_id
		 WHERE a.run_id = $1
		 ORDER BY a.requested_at DESC`,
		runID,
	)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	out := []policyApprovalDetail{}
	ids := []string{}
	for rows.Next() {
		var (
			approvalID  string
			decisionID  string
			runIDVal    sql.NullString
			status      string
			requestedAt time.Time
			requestedBy string
			decidedAt   sql.NullTime
			decidedBy   sql.NullString
			reason      sql.NullString
			policyID    string
			policyName  string
			versionID   string
			decision    string
			ruleID      sql.NullString
			contextRaw  []byte
		)
		if err := rows.Scan(
			&approvalID,
			&decisionID,
			&runIDVal,
			&status,
			&requestedAt,
			&requestedBy,
			&decidedAt,
			&decidedBy,
			&reason,
			&policyID,
			&policyName,
			&versionID,
			&decision,
			&ruleID,
			&contextRaw,
		); err != nil {
			return nil, nil, err
		}

		var decidedAtPtr *time.Time
		if decidedAt.Valid && !decidedAt.Time.IsZero() {
			t := decidedAt.Time.UTC()
			decidedAtPtr = &t
		}

		out = append(out, policyApprovalDetail{
			policyApprovalSummary: policyApprovalSummary{
				ApprovalID:      approvalID,
				DecisionID:      decisionID,
				RunID:           strings.TrimSpace(runIDVal.String),
				Status:          strings.TrimSpace(status),
				RequestedAt:     requestedAt,
				RequestedBy:     requestedBy,
				DecidedAt:       decidedAtPtr,
				DecidedBy:       strings.TrimSpace(decidedBy.String),
				Reason:          strings.TrimSpace(reason.String),
				PolicyID:        policyID,
				PolicyName:      policyName,
				PolicyVersionID: versionID,
				Decision:        decision,
				RuleID:          strings.TrimSpace(ruleID.String),
			},
			DecisionContext: normalizeJSON(contextRaw),
		})
		ids = append(ids, approvalID)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return out, uniqueNonEmpty(ids), nil
}

func fetchEvidencePolicyVersions(ctx context.Context, db *sql.DB, decisions []policyDecisionDetail) ([]evidencePolicyVersion, error) {
	versionIDs := []string{}
	seen := map[string]struct{}{}
	for _, decision := range decisions {
		id := strings.TrimSpace(decision.PolicyVersionID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		versionIDs = append(versionIDs, id)
	}
	if len(versionIDs) == 0 {
		return []evidencePolicyVersion{}, nil
	}

	sort.Strings(versionIDs)
	args := []any{}
	idClause := buildInClause(len(versionIDs), len(args)+1)
	for _, id := range versionIDs {
		args = append(args, id)
	}

	query := fmt.Sprintf(`SELECT v.policy_version_id,
			v.policy_id,
			p.name,
			p.description,
			v.version,
			v.status,
			v.spec_yaml,
			v.spec_json,
			v.spec_sha256,
			v.created_at,
			v.created_by
		FROM policy_versions v
		JOIN policies p ON p.policy_id = v.policy_id
		WHERE v.policy_version_id IN (%s)
		ORDER BY v.created_at DESC`, idClause)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []evidencePolicyVersion{}
	for rows.Next() {
		var (
			version evidencePolicyVersion
			desc    sql.NullString
			specRaw []byte
		)
		if err := rows.Scan(
			&version.PolicyVersionID,
			&version.PolicyID,
			&version.PolicyName,
			&desc,
			&version.Version,
			&version.Status,
			&version.SpecYAML,
			&specRaw,
			&version.SpecSHA256,
			&version.CreatedAt,
			&version.CreatedBy,
		); err != nil {
			return nil, err
		}
		version.PolicyDescription = strings.TrimSpace(desc.String)
		version.Spec = normalizeJSON(specRaw)
		out = append(out, version)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func buildEvidenceManifest(bundleID string, runID string, createdAt time.Time, createdBy string, files []evidenceBundleFile) ([]byte, error) {
	manifestFiles := make([]evidenceManifestFile, 0, len(files))
	for _, file := range files {
		manifestFiles = append(manifestFiles, evidenceManifestFile{
			Name:        file.Name,
			SHA256:      sha256HexBytes(file.Data),
			SizeBytes:   int64(len(file.Data)),
			ContentType: file.ContentType,
		})
	}
	sort.Slice(manifestFiles, func(i, j int) bool {
		return manifestFiles[i].Name < manifestFiles[j].Name
	})

	manifest := evidenceBundleManifest{
		Schema:    evidenceBundleSchemaV1,
		BundleID:  bundleID,
		RunID:     runID,
		CreatedAt: createdAt,
		CreatedBy: strings.TrimSpace(createdBy),
		Files:     manifestFiles,
	}
	return json.MarshalIndent(manifest, "", "  ")
}

func buildEvidenceZip(createdAt time.Time, files []evidenceBundleFile) ([]byte, string, int64, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for _, file := range files {
		header := &zip.FileHeader{
			Name:     file.Name,
			Method:   zip.Deflate,
			Modified: createdAt,
		}
		writer, err := zw.CreateHeader(header)
		if err != nil {
			_ = zw.Close()
			return nil, "", 0, err
		}
		if _, err := writer.Write(file.Data); err != nil {
			_ = zw.Close()
			return nil, "", 0, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, "", 0, err
	}

	bundleData := buf.Bytes()
	bundleSHA := sha256HexBytes(bundleData)
	return bundleData, bundleSHA, int64(len(bundleData)), nil
}

func computeEvidenceSignature(secret string, bundleSHA string) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", errors.New("evidence signing secret required")
	}
	if strings.TrimSpace(bundleSHA) == "" {
		return "", errors.New("bundle sha required")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(strings.TrimSpace(bundleSHA))); err != nil {
		return "", fmt.Errorf("signature hmac: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func buildInClause(count int, startIndex int) string {
	parts := make([]string, 0, count)
	for i := 0; i < count; i++ {
		parts = append(parts, "$"+strconv.Itoa(startIndex+i))
	}
	return strings.Join(parts, ",")
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
