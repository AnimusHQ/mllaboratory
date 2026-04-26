package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/google/uuid"
)

const (
	gitlabHeaderToken = "X-Gitlab-Token"
	gitlabHeaderEvent = "X-Gitlab-Event"
)

var (
	errGitlabRepoRequired   = errors.New("repo_required")
	errGitlabCommitRequired = errors.New("commit_sha_required")
)

type gitlabGovernanceRecord struct {
	GovernanceID             string    `json:"governance_id"`
	Repo                     string    `json:"repo"`
	CommitSHA                string    `json:"commit_sha"`
	Ref                      string    `json:"ref,omitempty"`
	RefProtected             *bool     `json:"ref_protected,omitempty"`
	ProjectID                *int64    `json:"project_id,omitempty"`
	ProjectPath              string    `json:"project_path,omitempty"`
	PipelineID               *int64    `json:"pipeline_id,omitempty"`
	PipelineURL              string    `json:"pipeline_url,omitempty"`
	PipelineStatus           string    `json:"pipeline_status,omitempty"`
	PipelineSource           string    `json:"pipeline_source,omitempty"`
	MergeRequestID           *int64    `json:"merge_request_id,omitempty"`
	MergeRequestIID          *int64    `json:"merge_request_iid,omitempty"`
	MergeRequestURL          string    `json:"merge_request_url,omitempty"`
	MergeRequestSourceBranch string    `json:"merge_request_source_branch,omitempty"`
	MergeRequestTargetBranch string    `json:"merge_request_target_branch,omitempty"`
	ApprovalsRequired        *int      `json:"approvals_required,omitempty"`
	ApprovalsReceived        *int      `json:"approvals_received,omitempty"`
	ApprovalsApproved        *bool     `json:"approvals_approved,omitempty"`
	EventType                string    `json:"event_type"`
	ReceivedAt               time.Time `json:"received_at"`
	ReceivedBy               string    `json:"received_by"`
	PayloadSHA256            string    `json:"payload_sha256"`
	Signature                string    `json:"signature"`
}

type gitlabWebhookResponse struct {
	Status string                 `json:"status"`
	Record gitlabGovernanceRecord `json:"record"`
}

type gitlabGovernanceInput struct {
	Repo                     string
	CommitSHA                string
	Ref                      string
	RefProtected             *bool
	ProjectID                *int64
	ProjectPath              string
	PipelineID               *int64
	PipelineURL              string
	PipelineStatus           string
	PipelineSource           string
	MergeRequestID           *int64
	MergeRequestIID          *int64
	MergeRequestURL          string
	MergeRequestSourceBranch string
	MergeRequestTargetBranch string
	ApprovalsRequired        *int
	ApprovalsReceived        *int
	ApprovalsApproved        *bool
	EventType                string
}

type gitlabGovernanceIntegrityInput struct {
	GovernanceID             string          `json:"governance_id"`
	Repo                     string          `json:"repo"`
	CommitSHA                string          `json:"commit_sha"`
	Ref                      string          `json:"ref,omitempty"`
	RefProtected             *bool           `json:"ref_protected,omitempty"`
	ProjectID                *int64          `json:"project_id,omitempty"`
	ProjectPath              string          `json:"project_path,omitempty"`
	PipelineID               *int64          `json:"pipeline_id,omitempty"`
	PipelineURL              string          `json:"pipeline_url,omitempty"`
	PipelineStatus           string          `json:"pipeline_status,omitempty"`
	PipelineSource           string          `json:"pipeline_source,omitempty"`
	MergeRequestID           *int64          `json:"merge_request_id,omitempty"`
	MergeRequestIID          *int64          `json:"merge_request_iid,omitempty"`
	MergeRequestURL          string          `json:"merge_request_url,omitempty"`
	MergeRequestSourceBranch string          `json:"merge_request_source_branch,omitempty"`
	MergeRequestTargetBranch string          `json:"merge_request_target_branch,omitempty"`
	ApprovalsRequired        *int            `json:"approvals_required,omitempty"`
	ApprovalsReceived        *int            `json:"approvals_received,omitempty"`
	ApprovalsApproved        *bool           `json:"approvals_approved,omitempty"`
	EventType                string          `json:"event_type"`
	Payload                  json.RawMessage `json:"payload"`
	PayloadSHA256            string          `json:"payload_sha256"`
	ReceivedAt               time.Time       `json:"received_at"`
	ReceivedBy               string          `json:"received_by"`
	Signature                string          `json:"signature"`
}

func (api *experimentsAPI) handleGitlabWebhook(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	if strings.TrimSpace(api.gitlabWebhookSecret) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	token := strings.TrimSpace(r.Header.Get(gitlabHeaderToken))
	if token == "" {
		api.auditGitlabWebhookReject(r.Context(), identity, r, "", "missing_token")
		api.writeError(w, r, http.StatusUnauthorized, "gitlab_token_required")
		return
	}
	if token != api.gitlabWebhookSecret {
		api.auditGitlabWebhookReject(r.Context(), identity, r, "", "invalid_token")
		api.writeError(w, r, http.StatusUnauthorized, "gitlab_token_invalid")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		api.auditGitlabWebhookReject(r.Context(), identity, r, "", "body_read_failed")
		api.writeError(w, r, http.StatusBadRequest, "invalid_body")
		return
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		api.auditGitlabWebhookReject(r.Context(), identity, r, "", "invalid_json")
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}

	eventType := strings.TrimSpace(r.Header.Get(gitlabHeaderEvent))
	input, err := parseGitlabGovernancePayload(payload, eventType)
	if err != nil {
		api.auditGitlabWebhookReject(r.Context(), identity, r, "", err.Error())
		switch {
		case errors.Is(err, errGitlabRepoRequired):
			api.writeError(w, r, http.StatusBadRequest, "repo_required")
		case errors.Is(err, errGitlabCommitRequired):
			api.writeError(w, r, http.StatusBadRequest, "commit_sha_required")
		default:
			api.writeError(w, r, http.StatusBadRequest, "invalid_payload")
		}
		return
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	payloadSHA := sha256HexBytes(payloadJSON)

	signature := sha256Hex(token)
	now := time.Now().UTC()
	governanceID := uuid.NewString()

	integrity, err := integritySHA256(gitlabGovernanceIntegrityInput{
		GovernanceID:             governanceID,
		Repo:                     input.Repo,
		CommitSHA:                input.CommitSHA,
		Ref:                      input.Ref,
		RefProtected:             input.RefProtected,
		ProjectID:                input.ProjectID,
		ProjectPath:              input.ProjectPath,
		PipelineID:               input.PipelineID,
		PipelineURL:              input.PipelineURL,
		PipelineStatus:           input.PipelineStatus,
		PipelineSource:           input.PipelineSource,
		MergeRequestID:           input.MergeRequestID,
		MergeRequestIID:          input.MergeRequestIID,
		MergeRequestURL:          input.MergeRequestURL,
		MergeRequestSourceBranch: input.MergeRequestSourceBranch,
		MergeRequestTargetBranch: input.MergeRequestTargetBranch,
		ApprovalsRequired:        input.ApprovalsRequired,
		ApprovalsReceived:        input.ApprovalsReceived,
		ApprovalsApproved:        input.ApprovalsApproved,
		EventType:                input.EventType,
		Payload:                  payloadJSON,
		PayloadSHA256:            payloadSHA,
		ReceivedAt:               now,
		ReceivedBy:               identity.Subject,
		Signature:                signature,
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
		`INSERT INTO gitlab_governance_events (
			governance_id,
			repo,
			commit_sha,
			ref,
			ref_protected,
			project_id,
			project_path,
			pipeline_id,
			pipeline_url,
			pipeline_status,
			pipeline_source,
			merge_request_id,
			merge_request_iid,
			merge_request_url,
			merge_request_source_branch,
			merge_request_target_branch,
			approvals_required,
			approvals_received,
			approvals_approved,
			event_type,
			payload,
			payload_sha256,
			received_at,
			received_by,
			signature,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26)
		ON CONFLICT (payload_sha256) DO NOTHING
		RETURNING governance_id`,
		governanceID,
		input.Repo,
		input.CommitSHA,
		nullString(input.Ref),
		nullBool(input.RefProtected),
		nullInt64(input.ProjectID),
		nullString(input.ProjectPath),
		nullInt64(input.PipelineID),
		nullString(input.PipelineURL),
		nullString(input.PipelineStatus),
		nullString(input.PipelineSource),
		nullInt64(input.MergeRequestID),
		nullInt64(input.MergeRequestIID),
		nullString(input.MergeRequestURL),
		nullString(input.MergeRequestSourceBranch),
		nullString(input.MergeRequestTargetBranch),
		nullInt(input.ApprovalsRequired),
		nullInt(input.ApprovalsReceived),
		nullBool(input.ApprovalsApproved),
		input.EventType,
		payloadJSON,
		payloadSHA,
		now,
		identity.Subject,
		signature,
		integrity,
	).Scan(&insertedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			existingRecord, fetchErr := api.getGitlabGovernanceRecordByPayload(r.Context(), tx, payloadSHA)
			if fetchErr != nil {
				api.writeError(w, r, http.StatusInternalServerError, "internal_error")
				return
			}

			_, auditErr := auditlog.Insert(r.Context(), tx, auditlog.Event{
				OccurredAt:   now,
				Actor:        identity.Subject,
				Action:       "gitlab_webhook.duplicate",
				ResourceType: "gitlab_governance",
				ResourceID:   existingRecord.GovernanceID,
				RequestID:    r.Header.Get("X-Request-Id"),
				IP:           requestIP(r.RemoteAddr),
				UserAgent:    r.UserAgent(),
				Payload: map[string]any{
					"service":        "experiments",
					"repo":           input.Repo,
					"commit_sha":     input.CommitSHA,
					"payload_sha256": payloadSHA,
					"event_type":     input.EventType,
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

			api.writeJSON(w, http.StatusOK, gitlabWebhookResponse{
				Status: "duplicate",
				Record: existingRecord,
			})
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	_, err = auditlog.Insert(r.Context(), tx, auditlog.Event{
		OccurredAt:   now,
		Actor:        identity.Subject,
		Action:       "gitlab_governance.create",
		ResourceType: "gitlab_governance",
		ResourceID:   insertedID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":        "experiments",
			"repo":           input.Repo,
			"commit_sha":     input.CommitSHA,
			"ref":            input.Ref,
			"payload_sha256": payloadSHA,
			"event_type":     input.EventType,
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

	api.writeJSON(w, http.StatusCreated, gitlabWebhookResponse{
		Status: "created",
		Record: gitlabGovernanceRecord{
			GovernanceID:             insertedID,
			Repo:                     input.Repo,
			CommitSHA:                input.CommitSHA,
			Ref:                      input.Ref,
			RefProtected:             input.RefProtected,
			ProjectID:                input.ProjectID,
			ProjectPath:              input.ProjectPath,
			PipelineID:               input.PipelineID,
			PipelineURL:              input.PipelineURL,
			PipelineStatus:           input.PipelineStatus,
			PipelineSource:           input.PipelineSource,
			MergeRequestID:           input.MergeRequestID,
			MergeRequestIID:          input.MergeRequestIID,
			MergeRequestURL:          input.MergeRequestURL,
			MergeRequestSourceBranch: input.MergeRequestSourceBranch,
			MergeRequestTargetBranch: input.MergeRequestTargetBranch,
			ApprovalsRequired:        input.ApprovalsRequired,
			ApprovalsReceived:        input.ApprovalsReceived,
			ApprovalsApproved:        input.ApprovalsApproved,
			EventType:                input.EventType,
			ReceivedAt:               now,
			ReceivedBy:               identity.Subject,
			PayloadSHA256:            payloadSHA,
			Signature:                signature,
		},
	})
}

func (api *experimentsAPI) auditGitlabWebhookReject(ctx context.Context, identity auth.Identity, r *http.Request, repo string, reason string) {
	payload := map[string]any{
		"service": "experiments",
		"reason":  reason,
	}
	if strings.TrimSpace(repo) != "" {
		payload["repo"] = repo
	}

	now := time.Now().UTC()
	_, _ = auditlog.Insert(ctx, api.db, auditlog.Event{
		OccurredAt:   now,
		Actor:        identity.Subject,
		Action:       "gitlab_webhook.reject",
		ResourceType: "gitlab_webhook",
		ResourceID:   r.Header.Get("X-Request-Id"),
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload:      payload,
	})
}

func (api *experimentsAPI) getGitlabGovernanceRecordByPayload(ctx context.Context, q auditlog.QueryRower, payloadSHA string) (gitlabGovernanceRecord, error) {
	var (
		record                   gitlabGovernanceRecord
		ref                      sql.NullString
		refProtected             sql.NullBool
		projectID                sql.NullInt64
		projectPath              sql.NullString
		pipelineID               sql.NullInt64
		pipelineURL              sql.NullString
		pipelineStatus           sql.NullString
		pipelineSource           sql.NullString
		mergeRequestID           sql.NullInt64
		mergeRequestIID          sql.NullInt64
		mergeRequestURL          sql.NullString
		mergeRequestSourceBranch sql.NullString
		mergeRequestTargetBranch sql.NullString
		approvalsRequired        sql.NullInt64
		approvalsReceived        sql.NullInt64
		approvalsApproved        sql.NullBool
	)

	err := q.QueryRowContext(
		ctx,
		`SELECT governance_id,
				repo,
				commit_sha,
				ref,
				ref_protected,
				project_id,
				project_path,
				pipeline_id,
				pipeline_url,
				pipeline_status,
				pipeline_source,
				merge_request_id,
				merge_request_iid,
				merge_request_url,
				merge_request_source_branch,
				merge_request_target_branch,
				approvals_required,
				approvals_received,
				approvals_approved,
				event_type,
				received_at,
				received_by,
				payload_sha256,
				signature
		 FROM gitlab_governance_events
		 WHERE payload_sha256 = $1
		 LIMIT 1`,
		payloadSHA,
	).Scan(
		&record.GovernanceID,
		&record.Repo,
		&record.CommitSHA,
		&ref,
		&refProtected,
		&projectID,
		&projectPath,
		&pipelineID,
		&pipelineURL,
		&pipelineStatus,
		&pipelineSource,
		&mergeRequestID,
		&mergeRequestIID,
		&mergeRequestURL,
		&mergeRequestSourceBranch,
		&mergeRequestTargetBranch,
		&approvalsRequired,
		&approvalsReceived,
		&approvalsApproved,
		&record.EventType,
		&record.ReceivedAt,
		&record.ReceivedBy,
		&record.PayloadSHA256,
		&record.Signature,
	)
	if err != nil {
		return gitlabGovernanceRecord{}, err
	}

	record.Ref = strings.TrimSpace(ref.String)
	record.RefProtected = optionalBool(refProtected)
	record.ProjectID = optionalInt64(projectID)
	record.ProjectPath = strings.TrimSpace(projectPath.String)
	record.PipelineID = optionalInt64(pipelineID)
	record.PipelineURL = strings.TrimSpace(pipelineURL.String)
	record.PipelineStatus = strings.TrimSpace(pipelineStatus.String)
	record.PipelineSource = strings.TrimSpace(pipelineSource.String)
	record.MergeRequestID = optionalInt64(mergeRequestID)
	record.MergeRequestIID = optionalInt64(mergeRequestIID)
	record.MergeRequestURL = strings.TrimSpace(mergeRequestURL.String)
	record.MergeRequestSourceBranch = strings.TrimSpace(mergeRequestSourceBranch.String)
	record.MergeRequestTargetBranch = strings.TrimSpace(mergeRequestTargetBranch.String)
	record.ApprovalsRequired = optionalInt(approvalsRequired)
	record.ApprovalsReceived = optionalInt(approvalsReceived)
	record.ApprovalsApproved = optionalBool(approvalsApproved)

	return record, nil
}

type gitlabGovernanceLookup struct {
	GovernanceID             string
	Repo                     string
	CommitSHA                string
	Ref                      sql.NullString
	RefProtected             sql.NullBool
	ProjectID                sql.NullInt64
	ProjectPath              sql.NullString
	PipelineID               sql.NullInt64
	PipelineURL              sql.NullString
	PipelineStatus           sql.NullString
	PipelineSource           sql.NullString
	MergeRequestID           sql.NullInt64
	MergeRequestIID          sql.NullInt64
	MergeRequestURL          sql.NullString
	MergeRequestSourceBranch sql.NullString
	MergeRequestTargetBranch sql.NullString
	ApprovalsRequired        sql.NullInt64
	ApprovalsReceived        sql.NullInt64
	ApprovalsApproved        sql.NullBool
	EventType                string
	ReceivedAt               time.Time
}

func (api *experimentsAPI) fetchGitlabGovernance(ctx context.Context, repo string, commit string, ref string) (*gitlabGovernanceLookup, error) {
	repo = normalizeRepo(repo)
	commit = normalizeCommit(commit)
	if repo == "" || commit == "" {
		return nil, nil
	}

	ref = strings.TrimSpace(ref)
	repoCandidates := []string{repo}
	if parts := strings.SplitN(repo, "/", 2); len(parts) == 2 && parts[1] != "" {
		repoCandidates = append(repoCandidates, parts[1])
	}

	for _, candidate := range repoCandidates {
		record, found, err := api.fetchGitlabGovernanceForRepo(ctx, candidate, commit, ref)
		if err != nil {
			return nil, err
		}
		if found {
			return record, nil
		}
	}
	if ref == "" {
		return nil, nil
	}
	return api.fetchGitlabGovernance(ctx, repo, commit, "")
}

func (api *experimentsAPI) fetchGitlabGovernanceForRepo(ctx context.Context, repo string, commit string, ref string) (*gitlabGovernanceLookup, bool, error) {
	query := `SELECT governance_id,
			repo,
			commit_sha,
			ref,
			ref_protected,
			project_id,
			project_path,
			pipeline_id,
			pipeline_url,
			pipeline_status,
			pipeline_source,
			merge_request_id,
			merge_request_iid,
			merge_request_url,
			merge_request_source_branch,
			merge_request_target_branch,
			approvals_required,
			approvals_received,
			approvals_approved,
			event_type,
			received_at
		FROM gitlab_governance_events
		WHERE repo = $1 AND commit_sha = $2`
	args := []any{repo, commit}
	if ref != "" {
		query += " AND ref = $3"
		args = append(args, ref)
	}
	query += " ORDER BY received_at DESC LIMIT 1"

	var record gitlabGovernanceLookup
	err := api.db.QueryRowContext(ctx, query, args...).Scan(
		&record.GovernanceID,
		&record.Repo,
		&record.CommitSHA,
		&record.Ref,
		&record.RefProtected,
		&record.ProjectID,
		&record.ProjectPath,
		&record.PipelineID,
		&record.PipelineURL,
		&record.PipelineStatus,
		&record.PipelineSource,
		&record.MergeRequestID,
		&record.MergeRequestIID,
		&record.MergeRequestURL,
		&record.MergeRequestSourceBranch,
		&record.MergeRequestTargetBranch,
		&record.ApprovalsRequired,
		&record.ApprovalsReceived,
		&record.ApprovalsApproved,
		&record.EventType,
		&record.ReceivedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &record, true, nil
}

func (api *experimentsAPI) gitlabPolicyMeta(ctx context.Context, repo string, commit string, ref string) (map[string]any, error) {
	record, err := api.fetchGitlabGovernance(ctx, repo, commit, ref)
	if err != nil || record == nil {
		return nil, err
	}

	meta := map[string]any{
		"repo":        record.Repo,
		"commit_sha":  record.CommitSHA,
		"event_type":  record.EventType,
		"received_at": record.ReceivedAt.UTC().Format(time.RFC3339),
	}
	if record.Ref.Valid {
		meta["ref"] = strings.TrimSpace(record.Ref.String)
	}
	if record.RefProtected.Valid {
		meta["ref_protected"] = record.RefProtected.Bool
	}
	if record.ProjectID.Valid {
		meta["project_id"] = record.ProjectID.Int64
	}
	if record.ProjectPath.Valid {
		meta["project_path"] = strings.TrimSpace(record.ProjectPath.String)
	}
	if record.PipelineID.Valid {
		meta["pipeline_id"] = record.PipelineID.Int64
	}
	if record.PipelineURL.Valid {
		meta["pipeline_url"] = strings.TrimSpace(record.PipelineURL.String)
	}
	if record.PipelineStatus.Valid {
		meta["pipeline_status"] = strings.TrimSpace(record.PipelineStatus.String)
	}
	if record.PipelineSource.Valid {
		meta["pipeline_source"] = strings.TrimSpace(record.PipelineSource.String)
	}
	if record.MergeRequestID.Valid {
		meta["merge_request_id"] = record.MergeRequestID.Int64
	}
	if record.MergeRequestIID.Valid {
		meta["merge_request_iid"] = record.MergeRequestIID.Int64
	}
	if record.MergeRequestURL.Valid {
		meta["merge_request_url"] = strings.TrimSpace(record.MergeRequestURL.String)
	}
	if record.MergeRequestSourceBranch.Valid {
		meta["merge_request_source_branch"] = strings.TrimSpace(record.MergeRequestSourceBranch.String)
	}
	if record.MergeRequestTargetBranch.Valid {
		meta["merge_request_target_branch"] = strings.TrimSpace(record.MergeRequestTargetBranch.String)
	}
	if record.ApprovalsRequired.Valid {
		meta["approvals_required"] = int(record.ApprovalsRequired.Int64)
	}
	if record.ApprovalsReceived.Valid {
		meta["approvals_received"] = int(record.ApprovalsReceived.Int64)
	}
	if record.ApprovalsApproved.Valid {
		meta["approvals_approved"] = record.ApprovalsApproved.Bool
	}

	return meta, nil
}

func parseGitlabGovernancePayload(payload map[string]any, headerEvent string) (gitlabGovernanceInput, error) {
	eventType := strings.TrimSpace(headerEvent)
	if eventType == "" {
		eventType = stringFromPath(payload, "object_kind", "event_type")
	}
	eventType = strings.ToLower(strings.TrimSpace(eventType))
	if eventType == "" {
		eventType = "unknown"
	}

	projectWebURL := stringFromPath(payload, "project.web_url", "project.http_url", "project.homepage")
	projectPath := strings.TrimSpace(stringFromPath(payload, "project.path_with_namespace", "project.path", "project.namespace"))
	projectID, projectIDOK := int64FromPath(payload, "project.id")

	repoRaw := projectWebURL
	if repoRaw == "" {
		repoRaw = stringFromPath(payload, "project.http_url", "project.ssh_url", "project.git_ssh_url", "project.git_http_url")
	}
	if repoRaw == "" && projectPath != "" {
		host := hostFromURL(projectWebURL)
		if host != "" {
			repoRaw = host + "/" + strings.Trim(projectPath, "/")
		} else {
			repoRaw = projectPath
		}
	}
	repo := normalizeRepo(repoRaw)
	if repo == "" {
		return gitlabGovernanceInput{}, errGitlabRepoRequired
	}

	commit := stringFromPath(payload,
		"object_attributes.sha",
		"object_attributes.last_commit.id",
		"object_attributes.commit_sha",
		"checkout_sha",
		"commit_sha",
	)
	commit = normalizeCommit(commit)
	if commit == "" {
		return gitlabGovernanceInput{}, errGitlabCommitRequired
	}

	ref := stringFromPath(payload,
		"object_attributes.ref",
		"object_attributes.source_branch",
		"ref",
	)
	ref = strings.TrimSpace(ref)

	refProtected, refProtectedOK := boolFromPath(payload,
		"object_attributes.ref_protected",
		"ref_protected",
		"protected_ref",
	)
	var refProtectedPtr *bool
	if refProtectedOK {
		refProtectedPtr = &refProtected
	}

	pipelineID, pipelineOK := int64FromPath(payload,
		"object_attributes.id",
		"pipeline_id",
	)
	var pipelineIDPtr *int64
	if pipelineOK {
		pipelineIDPtr = &pipelineID
	}
	pipelineURL := strings.TrimSpace(stringFromPath(payload,
		"object_attributes.web_url",
		"pipeline_url",
	))
	pipelineStatus := strings.TrimSpace(stringFromPath(payload,
		"object_attributes.status",
		"pipeline_status",
	))
	pipelineSource := strings.TrimSpace(stringFromPath(payload,
		"object_attributes.source",
		"pipeline_source",
	))

	mergeRequestID, mrIDOK := int64FromPath(payload, "merge_request.id")
	var mergeRequestIDPtr *int64
	if mrIDOK {
		mergeRequestIDPtr = &mergeRequestID
	} else if strings.Contains(eventType, "merge") {
		if value, ok := int64FromPath(payload, "object_attributes.id"); ok {
			mergeRequestIDPtr = &value
		}
	}
	mergeRequestIID, mrIIDOK := int64FromPath(payload, "merge_request.iid")
	var mergeRequestIIDPtr *int64
	if mrIIDOK {
		mergeRequestIIDPtr = &mergeRequestIID
	} else if strings.Contains(eventType, "merge") {
		if value, ok := int64FromPath(payload, "object_attributes.iid"); ok {
			mergeRequestIIDPtr = &value
		}
	}
	mergeRequestURL := strings.TrimSpace(stringFromPath(payload,
		"merge_request.url",
		"merge_request.web_url",
	))
	if mergeRequestURL == "" && strings.Contains(eventType, "merge") {
		mergeRequestURL = strings.TrimSpace(stringFromPath(payload, "object_attributes.url", "object_attributes.web_url"))
	}

	mergeRequestSourceBranch := strings.TrimSpace(stringFromPath(payload, "merge_request.source_branch"))
	mergeRequestTargetBranch := strings.TrimSpace(stringFromPath(payload, "merge_request.target_branch"))
	if strings.Contains(eventType, "merge") {
		if mergeRequestSourceBranch == "" {
			mergeRequestSourceBranch = strings.TrimSpace(stringFromPath(payload, "object_attributes.source_branch"))
		}
		if mergeRequestTargetBranch == "" {
			mergeRequestTargetBranch = strings.TrimSpace(stringFromPath(payload, "object_attributes.target_branch"))
		}
	}

	approvalsRequired, approvalsRequiredOK := intFromPath(payload,
		"approvals_required",
		"approvals.required",
		"merge_request.approvals_required",
		"object_attributes.approvals_required",
		"object_attributes.approvals_before_merge",
	)
	approvalsReceived, approvalsReceivedOK := intFromPath(payload,
		"approvals_received",
		"approvals.received",
		"merge_request.approvals_received",
		"merge_request.approvals",
		"object_attributes.approvals_received",
	)
	approvalsApproved, approvalsApprovedOK := boolFromPath(payload,
		"approvals_approved",
		"approved",
		"approvals.approved",
		"merge_request.approved",
		"object_attributes.approved",
	)

	var approvalsRequiredPtr *int
	if approvalsRequiredOK {
		approvalsRequiredPtr = &approvalsRequired
	}
	var approvalsReceivedPtr *int
	if approvalsReceivedOK {
		approvalsReceivedPtr = &approvalsReceived
	}
	var approvalsApprovedPtr *bool
	if approvalsApprovedOK {
		approvalsApprovedPtr = &approvalsApproved
	}
	if approvalsApprovedPtr == nil && approvalsRequiredPtr != nil && approvalsReceivedPtr != nil && *approvalsRequiredPtr > 0 {
		approved := *approvalsReceivedPtr >= *approvalsRequiredPtr
		approvalsApprovedPtr = &approved
	}

	var projectIDPtr *int64
	if projectIDOK {
		projectIDPtr = &projectID
	}

	return gitlabGovernanceInput{
		Repo:                     repo,
		CommitSHA:                commit,
		Ref:                      ref,
		RefProtected:             refProtectedPtr,
		ProjectID:                projectIDPtr,
		ProjectPath:              strings.TrimSpace(projectPath),
		PipelineID:               pipelineIDPtr,
		PipelineURL:              pipelineURL,
		PipelineStatus:           pipelineStatus,
		PipelineSource:           pipelineSource,
		MergeRequestID:           mergeRequestIDPtr,
		MergeRequestIID:          mergeRequestIIDPtr,
		MergeRequestURL:          mergeRequestURL,
		MergeRequestSourceBranch: mergeRequestSourceBranch,
		MergeRequestTargetBranch: mergeRequestTargetBranch,
		ApprovalsRequired:        approvalsRequiredPtr,
		ApprovalsReceived:        approvalsReceivedPtr,
		ApprovalsApproved:        approvalsApprovedPtr,
		EventType:                eventType,
	}, nil
}

func normalizeRepo(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if parsed, err := url.Parse(raw); err == nil && parsed.Host != "" {
		host := strings.ToLower(strings.TrimSpace(parsed.Host))
		path := strings.Trim(parsed.Path, "/")
		if host == "" || path == "" {
			return ""
		}
		path = strings.TrimSuffix(path, ".git")
		if path == "" {
			return ""
		}
		return strings.ToLower(host + "/" + path)
	}
	if strings.Contains(raw, "@") && strings.Contains(raw, ":") {
		parts := strings.SplitN(raw, ":", 2)
		if len(parts) == 2 {
			host := strings.TrimPrefix(parts[0], "git@")
			host = strings.ToLower(strings.TrimSpace(host))
			path := strings.Trim(strings.TrimSuffix(parts[1], ".git"), "/")
			if host != "" && path != "" {
				return strings.ToLower(host + "/" + path)
			}
		}
	}
	raw = strings.Trim(strings.TrimSuffix(raw, ".git"), "/")
	return strings.ToLower(raw)
}

func normalizeCommit(commit string) string {
	return strings.ToLower(strings.TrimSpace(commit))
}

func hostFromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Host)
}

func stringFromPath(payload map[string]any, paths ...string) string {
	for _, path := range paths {
		if value, ok := resolvePayloadPath(payload, path); ok {
			if str := stringFromValue(value); str != "" {
				return str
			}
		}
	}
	return ""
}

func boolFromPath(payload map[string]any, paths ...string) (bool, bool) {
	for _, path := range paths {
		if value, ok := resolvePayloadPath(payload, path); ok {
			if parsed, ok := boolFromValue(value); ok {
				return parsed, true
			}
		}
	}
	return false, false
}

func int64FromPath(payload map[string]any, paths ...string) (int64, bool) {
	for _, path := range paths {
		if value, ok := resolvePayloadPath(payload, path); ok {
			if parsed, ok := int64FromValue(value); ok {
				return parsed, true
			}
		}
	}
	return 0, false
}

func intFromPath(payload map[string]any, paths ...string) (int, bool) {
	if value, ok := int64FromPath(payload, paths...); ok {
		if value > int64(^uint(0)>>1) {
			return 0, false
		}
		return int(value), true
	}
	return 0, false
}

func resolvePayloadPath(payload map[string]any, path string) (any, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, false
	}
	parts := strings.Split(path, ".")
	var current any = payload
	for _, part := range parts {
		key := strings.TrimSpace(part)
		if key == "" {
			return nil, false
		}
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[key]
			if !ok {
				return nil, false
			}
			current = next
		default:
			return nil, false
		}
	}
	return current, true
}

func stringFromValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []byte:
		return strings.TrimSpace(string(typed))
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func boolFromValue(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		if typed == "" {
			return false, false
		}
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		if err != nil {
			return false, false
		}
		return parsed, true
	case float64:
		if typed == 0 {
			return false, true
		}
		if typed == 1 {
			return true, true
		}
		return false, false
	default:
		return false, false
	}
}

func int64FromValue(value any) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return parsed, true
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func nullInt64(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
}

func nullInt(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}

func nullBool(value *bool) sql.NullBool {
	if value == nil {
		return sql.NullBool{}
	}
	return sql.NullBool{Bool: *value, Valid: true}
}

func optionalInt64(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	out := value.Int64
	return &out
}

func optionalInt(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	out := int(value.Int64)
	return &out
}

func optionalBool(value sql.NullBool) *bool {
	if !value.Valid {
		return nil
	}
	out := value.Bool
	return &out
}
