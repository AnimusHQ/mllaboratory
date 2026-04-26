package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/platform/policy"
	"github.com/google/uuid"
)

type policyVersionRecord struct {
	PolicyID        string
	PolicyName      string
	PolicyVersionID string
	Version         int
	Status          string
	SpecJSON        []byte
	SpecSHA256      string
}

type policyEvaluation struct {
	PolicyID        string
	PolicyName      string
	PolicyVersionID string
	PolicyVersion   int
	PolicySHA256    string
	Decision        policy.Decision
}

type executionPolicyInput struct {
	RunID             string
	Actor             policy.ActorContext
	ExperimentID      string
	DatasetID         string
	DatasetVersionID  string
	DatasetSHA256     string
	GitRepo           string
	GitCommit         string
	GitRef            string
	ImageRef          string
	ImageDigest       string
	ImageExecutionRef string
	Resources         map[string]any
}

type executionPolicyResult struct {
	Context       policy.Context
	ContextJSON   []byte
	ContextSHA256 string
	Evaluations   []policyEvaluation
}

func (api *experimentsAPI) evaluateExecutionPolicy(ctx context.Context, input executionPolicyInput) (executionPolicyResult, error) {
	policies, err := api.loadActivePolicyVersions(ctx)
	if err != nil {
		return executionPolicyResult{}, err
	}

	context := policy.Context{
		Actor: policy.ActorContext{
			Subject: strings.TrimSpace(input.Actor.Subject),
			Email:   strings.TrimSpace(input.Actor.Email),
			Roles:   input.Actor.Roles,
		},
		Dataset: policy.DatasetContext{
			DatasetID: strings.TrimSpace(input.DatasetID),
			VersionID: strings.TrimSpace(input.DatasetVersionID),
			SHA256:    strings.TrimSpace(input.DatasetSHA256),
		},
		Experiment: policy.ExperimentContext{
			ExperimentID: strings.TrimSpace(input.ExperimentID),
			RunID:        strings.TrimSpace(input.RunID),
		},
		Git: policy.GitContext{
			Repo:   strings.TrimSpace(input.GitRepo),
			Commit: strings.TrimSpace(input.GitCommit),
			Ref:    strings.TrimSpace(input.GitRef),
		},
		Image: policy.ImageContext{
			Ref:    strings.TrimSpace(input.ImageRef),
			Digest: strings.TrimSpace(input.ImageDigest),
		},
		Resources: input.Resources,
		Meta: map[string]interface{}{
			"image_execution_ref": strings.TrimSpace(input.ImageExecutionRef),
		},
	}

	gitlabMeta, err := api.gitlabPolicyMeta(ctx, input.GitRepo, input.GitCommit, input.GitRef)
	if err != nil {
		return executionPolicyResult{}, err
	}
	if len(gitlabMeta) > 0 {
		context.Meta["gitlab"] = gitlabMeta
	}

	contextJSON, err := json.Marshal(context)
	if err != nil {
		return executionPolicyResult{}, err
	}
	contextSHA := sha256HexBytes(contextJSON)

	evaluations := make([]policyEvaluation, 0, len(policies))
	for _, record := range policies {
		var spec policy.Spec
		if err := json.Unmarshal(record.SpecJSON, &spec); err != nil {
			return executionPolicyResult{}, err
		}
		decision, err := policy.Evaluate(spec, context)
		if err != nil {
			return executionPolicyResult{}, err
		}
		evaluations = append(evaluations, policyEvaluation{
			PolicyID:        record.PolicyID,
			PolicyName:      record.PolicyName,
			PolicyVersionID: record.PolicyVersionID,
			PolicyVersion:   record.Version,
			PolicySHA256:    record.SpecSHA256,
			Decision:        decision,
		})
	}

	return executionPolicyResult{
		Context:       context,
		ContextJSON:   contextJSON,
		ContextSHA256: contextSHA,
		Evaluations:   evaluations,
	}, nil
}

func (api *experimentsAPI) loadActivePolicyVersions(ctx context.Context) ([]policyVersionRecord, error) {
	rows, err := api.db.QueryContext(
		ctx,
		`SELECT p.policy_id,
				p.name,
				v.policy_version_id,
				v.version,
				v.status,
				v.spec_json,
				v.spec_sha256
		 FROM policies p
		 JOIN LATERAL (
			SELECT policy_version_id, version, status, spec_json, spec_sha256
			FROM policy_versions
			WHERE policy_id = p.policy_id
			ORDER BY version DESC
			LIMIT 1
		 ) v ON true
		 WHERE v.status = 'active'
		 ORDER BY p.created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []policyVersionRecord{}
	for rows.Next() {
		var record policyVersionRecord
		if err := rows.Scan(
			&record.PolicyID,
			&record.PolicyName,
			&record.PolicyVersionID,
			&record.Version,
			&record.Status,
			&record.SpecJSON,
			&record.SpecSHA256,
		); err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func aggregatePolicyDecision(evaluations []policyEvaluation) string {
	if len(evaluations) == 0 {
		return policy.EffectAllow
	}
	outcome := policy.EffectAllow
	for _, evaluation := range evaluations {
		switch evaluation.Decision.Effect {
		case policy.EffectDeny:
			return policy.EffectDeny
		case policy.EffectRequireApproval:
			outcome = policy.EffectRequireApproval
		}
	}
	return outcome
}

type insertedPolicyDecision struct {
	DecisionID      string
	PolicyID        string
	PolicyName      string
	PolicyVersionID string
	Effect          string
	RuleID          string
}

func (api *experimentsAPI) insertPolicyDecisions(ctx context.Context, tx *sql.Tx, runID string, actor auth.Identity, contextJSON []byte, contextSHA256 string, evaluations []policyEvaluation) ([]insertedPolicyDecision, error) {
	if tx == nil {
		return nil, errors.New("tx is required")
	}
	if len(evaluations) == 0 {
		return nil, nil
	}

	now := time.Now().UTC()
	runID = strings.TrimSpace(runID)
	var runIDValue sql.NullString
	if runID != "" {
		runIDValue = sql.NullString{String: runID, Valid: true}
	}

	out := make([]insertedPolicyDecision, 0, len(evaluations))
	for _, evaluation := range evaluations {
		decisionID := uuid.NewString()
		decision := strings.ToLower(strings.TrimSpace(evaluation.Decision.Effect))
		if decision == "" {
			return nil, errors.New("policy decision is required")
		}
		ruleID := strings.TrimSpace(evaluation.Decision.RuleID)
		reason := strings.TrimSpace(evaluation.Decision.Reason)
		if contextJSON == nil {
			contextJSON = []byte("{}")
		}
		contextSHA := strings.TrimSpace(contextSHA256)

		type integrityInput struct {
			DecisionID      string          `json:"decision_id"`
			RunID           string          `json:"run_id,omitempty"`
			PolicyID        string          `json:"policy_id"`
			PolicyVersionID string          `json:"policy_version_id"`
			PolicySHA256    string          `json:"policy_sha256"`
			Context         json.RawMessage `json:"context"`
			ContextSHA256   string          `json:"context_sha256"`
			Decision        string          `json:"decision"`
			RuleID          string          `json:"rule_id,omitempty"`
			Reason          string          `json:"reason,omitempty"`
			CreatedAt       time.Time       `json:"created_at"`
			CreatedBy       string          `json:"created_by"`
		}
		integrity, err := integritySHA256(integrityInput{
			DecisionID:      decisionID,
			RunID:           runID,
			PolicyID:        evaluation.PolicyID,
			PolicyVersionID: evaluation.PolicyVersionID,
			PolicySHA256:    evaluation.PolicySHA256,
			Context:         contextJSON,
			ContextSHA256:   contextSHA,
			Decision:        decision,
			RuleID:          ruleID,
			Reason:          reason,
			CreatedAt:       now,
			CreatedBy:       actor.Subject,
		})
		if err != nil {
			return nil, err
		}

		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO policy_decisions (
				decision_id,
				run_id,
				policy_id,
				policy_version_id,
				policy_sha256,
				context,
				context_sha256,
				decision,
				rule_id,
				reason,
				created_at,
				created_by,
				integrity_sha256
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
			decisionID,
			runIDValue,
			evaluation.PolicyID,
			evaluation.PolicyVersionID,
			evaluation.PolicySHA256,
			contextJSON,
			contextSHA,
			decision,
			nullString(ruleID),
			nullString(reason),
			now,
			actor.Subject,
			integrity,
		)
		if err != nil {
			return nil, err
		}

		out = append(out, insertedPolicyDecision{
			DecisionID:      decisionID,
			PolicyID:        evaluation.PolicyID,
			PolicyName:      evaluation.PolicyName,
			PolicyVersionID: evaluation.PolicyVersionID,
			Effect:          decision,
			RuleID:          ruleID,
		})
	}

	return out, nil
}

func (api *experimentsAPI) insertPolicyApprovals(ctx context.Context, tx *sql.Tx, runID string, actor auth.Identity, decisions []insertedPolicyDecision) ([]string, error) {
	if tx == nil {
		return nil, errors.New("tx is required")
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, errors.New("run_id is required")
	}

	now := time.Now().UTC()
	out := []string{}
	for _, decision := range decisions {
		if decision.Effect != policy.EffectRequireApproval {
			continue
		}
		approvalID := uuid.NewString()

		type integrityInput struct {
			ApprovalID  string    `json:"approval_id"`
			DecisionID  string    `json:"decision_id"`
			RunID       string    `json:"run_id"`
			Status      string    `json:"status"`
			RequestedAt time.Time `json:"requested_at"`
			RequestedBy string    `json:"requested_by"`
		}
		integrity, err := integritySHA256(integrityInput{
			ApprovalID:  approvalID,
			DecisionID:  decision.DecisionID,
			RunID:       runID,
			Status:      "pending",
			RequestedAt: now,
			RequestedBy: actor.Subject,
		})
		if err != nil {
			return nil, err
		}

		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO policy_approvals (
				approval_id,
				decision_id,
				run_id,
				status,
				requested_at,
				requested_by,
				integrity_sha256
			) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			approvalID,
			decision.DecisionID,
			runID,
			"pending",
			now,
			actor.Subject,
			integrity,
		)
		if err != nil {
			return nil, err
		}
		out = append(out, approvalID)
	}
	return out, nil
}
