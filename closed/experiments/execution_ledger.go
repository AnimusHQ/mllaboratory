package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const executionLedgerSchemaV1 = "animus.execution_ledger.v1"
const executionReplaySchemaV1 = "animus.execution_replay.v1"

type executionLedgerEntry struct {
	Schema       string                  `json:"schema"`
	LedgerID     string                  `json:"ledger_id"`
	RunID        string                  `json:"run_id"`
	ExecutionID  string                  `json:"execution_id"`
	ExperimentID string                  `json:"experiment_id"`
	Dataset      executionLedgerDataset  `json:"dataset"`
	Git          executionLedgerGit      `json:"git"`
	Image        executionLedgerImage    `json:"image"`
	Executor     executionLedgerExecutor `json:"executor"`
	Params       json.RawMessage         `json:"params"`
	Resources    json.RawMessage         `json:"resources"`
	Policy       executionLedgerPolicy   `json:"policy"`
	CreatedAt    time.Time               `json:"created_at"`
	CreatedBy    string                  `json:"created_by"`
}

type executionLedgerDataset struct {
	DatasetID string `json:"dataset_id"`
	VersionID string `json:"version_id"`
	SHA256    string `json:"sha256"`
}

type executionLedgerGit struct {
	Repo   string `json:"repo,omitempty"`
	Commit string `json:"commit,omitempty"`
	Ref    string `json:"ref,omitempty"`
}

type executionLedgerImage struct {
	Ref    string `json:"ref"`
	Digest string `json:"digest"`
}

type executionLedgerExecutor struct {
	Kind              string `json:"kind"`
	K8sNamespace      string `json:"k8s_namespace,omitempty"`
	K8sJobName        string `json:"k8s_job_name,omitempty"`
	DockerContainerID string `json:"docker_container_id,omitempty"`
	DatapilotURL      string `json:"datapilot_url"`
}

type executionLedgerPolicy struct {
	Decisions []executionLedgerDecision `json:"decisions"`
	Approvals []executionLedgerApproval `json:"approvals,omitempty"`
}

type executionLedgerDecision struct {
	DecisionID      string    `json:"decision_id"`
	PolicyID        string    `json:"policy_id"`
	PolicyVersionID string    `json:"policy_version_id"`
	PolicySHA256    string    `json:"policy_sha256"`
	ContextSHA256   string    `json:"context_sha256"`
	Decision        string    `json:"decision"`
	RuleID          string    `json:"rule_id,omitempty"`
	Reason          string    `json:"reason,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	CreatedBy       string    `json:"created_by"`
}

type executionLedgerApproval struct {
	ApprovalID  string     `json:"approval_id"`
	DecisionID  string     `json:"decision_id"`
	Status      string     `json:"status"`
	RequestedAt time.Time  `json:"requested_at"`
	RequestedBy string     `json:"requested_by"`
	DecidedAt   *time.Time `json:"decided_at,omitempty"`
	DecidedBy   string     `json:"decided_by,omitempty"`
	Reason      string     `json:"reason,omitempty"`
}

type executionReplayBundle struct {
	Schema           string          `json:"schema"`
	RunID            string          `json:"run_id"`
	ExperimentID     string          `json:"experiment_id"`
	DatasetID        string          `json:"dataset_id"`
	DatasetVersionID string          `json:"dataset_version_id"`
	DatasetSHA256    string          `json:"dataset_sha256"`
	GitRepo          string          `json:"git_repo,omitempty"`
	GitCommit        string          `json:"git_commit,omitempty"`
	GitRef           string          `json:"git_ref,omitempty"`
	ImageRef         string          `json:"image_ref"`
	ImageDigest      string          `json:"image_digest"`
	Executor         string          `json:"executor"`
	Resources        json.RawMessage `json:"resources"`
	Params           json.RawMessage `json:"params"`
}

func (api *experimentsAPI) insertExecutionLedgerEntry(ctx context.Context, tx *sql.Tx, runID string, executionID string) (string, bool, error) {
	if tx == nil {
		return "", false, errors.New("tx is required")
	}
	runID = strings.TrimSpace(runID)
	executionID = strings.TrimSpace(executionID)
	if runID == "" || executionID == "" {
		return "", false, errors.New("run_id and execution_id are required")
	}

	var (
		experimentID     string
		datasetVersionID string
		gitRepo          string
		gitCommit        string
		gitRef           string
		paramsRaw        []byte
		executorKind     string
		imageRef         string
		imageDigest      string
		resourcesRaw     []byte
		k8sNamespace     string
		k8sJobName       string
		dockerContainer  string
		datapilotURL     string
		createdAt        time.Time
		createdBy        string
		datasetID        sql.NullString
		datasetSHA       sql.NullString
	)

	err := tx.QueryRowContext(
		ctx,
		`SELECT r.experiment_id,
				r.dataset_version_id,
				COALESCE(r.git_repo,''),
				COALESCE(r.git_commit,''),
				COALESCE(r.git_ref,''),
				r.params,
				e.executor,
				e.image_ref,
				e.image_digest,
				e.resources,
				COALESCE(e.k8s_namespace,''),
				COALESCE(e.k8s_job_name,''),
				COALESCE(e.docker_container_id,''),
				e.datapilot_url,
				e.created_at,
				e.created_by,
				v.dataset_id,
				v.content_sha256
		 FROM experiment_runs r
		 JOIN experiment_run_executions e ON e.run_id = r.run_id
		 LEFT JOIN dataset_versions v ON v.version_id = r.dataset_version_id
		 WHERE r.run_id = $1 AND e.execution_id = $2`,
		runID,
		executionID,
	).Scan(
		&experimentID,
		&datasetVersionID,
		&gitRepo,
		&gitCommit,
		&gitRef,
		&paramsRaw,
		&executorKind,
		&imageRef,
		&imageDigest,
		&resourcesRaw,
		&k8sNamespace,
		&k8sJobName,
		&dockerContainer,
		&datapilotURL,
		&createdAt,
		&createdBy,
		&datasetID,
		&datasetSHA,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, errors.New("execution record not found")
		}
		return "", false, err
	}

	experimentID = strings.TrimSpace(experimentID)
	datasetVersionID = strings.TrimSpace(datasetVersionID)
	datasetIDValue := strings.TrimSpace(datasetID.String)
	datasetSHAValue := strings.TrimSpace(datasetSHA.String)
	gitRepo = strings.TrimSpace(gitRepo)
	gitCommit = strings.TrimSpace(gitCommit)
	gitRef = strings.TrimSpace(gitRef)
	executorKind = strings.TrimSpace(executorKind)
	imageRef = strings.TrimSpace(imageRef)
	imageDigest = strings.TrimSpace(imageDigest)
	k8sNamespace = strings.TrimSpace(k8sNamespace)
	k8sJobName = strings.TrimSpace(k8sJobName)
	dockerContainer = strings.TrimSpace(dockerContainer)
	datapilotURL = strings.TrimSpace(datapilotURL)
	createdBy = strings.TrimSpace(createdBy)

	if datasetVersionID == "" || datasetSHAValue == "" {
		return "", false, errors.New("dataset metadata missing")
	}
	if imageDigest == "" {
		return "", false, errors.New("image digest missing")
	}

	decisions, err := fetchExecutionLedgerDecisions(ctx, tx, runID)
	if err != nil {
		return "", false, err
	}
	approvals, err := fetchExecutionLedgerApprovals(ctx, tx, runID)
	if err != nil {
		return "", false, err
	}

	paramsJSON := normalizeJSON(paramsRaw)
	resourcesJSON := normalizeJSON(resourcesRaw)

	ledgerID := uuid.NewString()
	entry := executionLedgerEntry{
		Schema:       executionLedgerSchemaV1,
		LedgerID:     ledgerID,
		RunID:        runID,
		ExecutionID:  executionID,
		ExperimentID: experimentID,
		Dataset: executionLedgerDataset{
			DatasetID: datasetIDValue,
			VersionID: datasetVersionID,
			SHA256:    datasetSHAValue,
		},
		Git: executionLedgerGit{
			Repo:   gitRepo,
			Commit: gitCommit,
			Ref:    gitRef,
		},
		Image: executionLedgerImage{
			Ref:    imageRef,
			Digest: imageDigest,
		},
		Executor: executionLedgerExecutor{
			Kind:              executorKind,
			K8sNamespace:      k8sNamespace,
			K8sJobName:        k8sJobName,
			DockerContainerID: dockerContainer,
			DatapilotURL:      datapilotURL,
		},
		Params:    paramsJSON,
		Resources: resourcesJSON,
		Policy: executionLedgerPolicy{
			Decisions: decisions,
			Approvals: approvals,
		},
		CreatedAt: createdAt.UTC(),
		CreatedBy: createdBy,
	}
	entryJSON, err := json.Marshal(entry)
	if err != nil {
		return "", false, err
	}
	entrySHA := sha256HexBytes(entryJSON)

	replay := executionReplayBundle{
		Schema:           executionReplaySchemaV1,
		RunID:            runID,
		ExperimentID:     experimentID,
		DatasetID:        datasetIDValue,
		DatasetVersionID: datasetVersionID,
		DatasetSHA256:    datasetSHAValue,
		GitRepo:          gitRepo,
		GitCommit:        gitCommit,
		GitRef:           gitRef,
		ImageRef:         imageRef,
		ImageDigest:      imageDigest,
		Executor:         executorKind,
		Resources:        resourcesJSON,
		Params:           paramsJSON,
	}
	replayJSON, err := json.Marshal(replay)
	if err != nil {
		return "", false, err
	}
	executionHash := sha256HexBytes(replayJSON)

	type ledgerIntegrityInput struct {
		LedgerID      string          `json:"ledger_id"`
		RunID         string          `json:"run_id"`
		ExecutionID   string          `json:"execution_id"`
		Entry         json.RawMessage `json:"entry"`
		EntrySHA256   string          `json:"entry_sha256"`
		ExecutionHash string          `json:"execution_hash"`
		ReplayBundle  json.RawMessage `json:"replay_bundle"`
		CreatedAt     time.Time       `json:"created_at"`
		CreatedBy     string          `json:"created_by"`
	}
	integrity, err := integritySHA256(ledgerIntegrityInput{
		LedgerID:      ledgerID,
		RunID:         runID,
		ExecutionID:   executionID,
		Entry:         entryJSON,
		EntrySHA256:   entrySHA,
		ExecutionHash: executionHash,
		ReplayBundle:  replayJSON,
		CreatedAt:     createdAt.UTC(),
		CreatedBy:     createdBy,
	})
	if err != nil {
		return "", false, err
	}

	res, err := tx.ExecContext(
		ctx,
		`INSERT INTO execution_ledger_entries (
			ledger_id,
			run_id,
			execution_id,
			entry,
			entry_sha256,
			execution_hash,
			replay_bundle,
			created_at,
			created_by,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (run_id) DO NOTHING`,
		ledgerID,
		runID,
		executionID,
		entryJSON,
		entrySHA,
		executionHash,
		replayJSON,
		createdAt.UTC(),
		createdBy,
		integrity,
	)
	if err != nil {
		return "", false, err
	}
	rows, _ := res.RowsAffected()
	return ledgerID, rows > 0, nil
}

func fetchExecutionLedgerDecisions(ctx context.Context, tx *sql.Tx, runID string) ([]executionLedgerDecision, error) {
	rows, err := tx.QueryContext(
		ctx,
		`SELECT decision_id,
				policy_id,
				policy_version_id,
				policy_sha256,
				context_sha256,
				decision,
				rule_id,
				reason,
				created_at,
				created_by
		 FROM policy_decisions
		 WHERE run_id = $1
		 ORDER BY created_at ASC`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []executionLedgerDecision{}
	for rows.Next() {
		var (
			entry    executionLedgerDecision
			ruleID   sql.NullString
			reason   sql.NullString
			decision string
		)
		if err := rows.Scan(
			&entry.DecisionID,
			&entry.PolicyID,
			&entry.PolicyVersionID,
			&entry.PolicySHA256,
			&entry.ContextSHA256,
			&decision,
			&ruleID,
			&reason,
			&entry.CreatedAt,
			&entry.CreatedBy,
		); err != nil {
			return nil, err
		}
		entry.Decision = strings.TrimSpace(decision)
		entry.RuleID = strings.TrimSpace(ruleID.String)
		entry.Reason = strings.TrimSpace(reason.String)
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func fetchExecutionLedgerApprovals(ctx context.Context, tx *sql.Tx, runID string) ([]executionLedgerApproval, error) {
	rows, err := tx.QueryContext(
		ctx,
		`SELECT approval_id,
				decision_id,
				status,
				requested_at,
				requested_by,
				decided_at,
				decided_by,
				reason
		 FROM policy_approvals
		 WHERE run_id = $1
		 ORDER BY requested_at ASC`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []executionLedgerApproval{}
	for rows.Next() {
		var (
			entry     executionLedgerApproval
			decidedAt sql.NullTime
			decidedBy sql.NullString
			reason    sql.NullString
		)
		if err := rows.Scan(
			&entry.ApprovalID,
			&entry.DecisionID,
			&entry.Status,
			&entry.RequestedAt,
			&entry.RequestedBy,
			&decidedAt,
			&decidedBy,
			&reason,
		); err != nil {
			return nil, err
		}
		if decidedAt.Valid && !decidedAt.Time.IsZero() {
			t := decidedAt.Time.UTC()
			entry.DecidedAt = &t
		}
		entry.DecidedBy = strings.TrimSpace(decidedBy.String)
		entry.Reason = strings.TrimSpace(reason.String)
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
