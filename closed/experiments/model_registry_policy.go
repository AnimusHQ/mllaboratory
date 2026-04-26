package main

import (
	"context"
	"errors"
	"strings"

	"github.com/animus-labs/animus-go/closed/internal/platform/policy"
)

type modelExportPolicyDecision struct {
	Allowed bool
	Code    string
}

const (
	modelExportPolicyDecisionRequired = "policy_decision_required"
	modelExportPolicyDenied           = "policy_denied"
	modelExportPolicyApprovalRequired = "policy_approval_required"
	modelExportPolicyApprovalDenied   = "policy_approval_denied"
)

const (
	selectModelExportPolicyDecisionsQuery = `SELECT d.decision
		FROM policy_decisions d
		JOIN experiment_runs r ON r.run_id = d.run_id
		WHERE d.run_id = $1 AND r.project_id = $2`
	selectModelExportPolicyApprovalsQuery = `SELECT a.status
		FROM policy_approvals a
		JOIN experiment_runs r ON r.run_id = a.run_id
		WHERE a.run_id = $1 AND r.project_id = $2`
)

func (api *experimentsAPI) checkModelExportPolicy(ctx context.Context, projectID, runID string) (modelExportPolicyDecision, error) {
	if api == nil {
		return modelExportPolicyDecision{}, errors.New("api not initialized")
	}
	if api.modelExportPolicyOverride != nil {
		return api.modelExportPolicyOverride(ctx, projectID, runID)
	}
	if api.db == nil {
		return modelExportPolicyDecision{}, errors.New("db not configured")
	}
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	if projectID == "" {
		return modelExportPolicyDecision{}, errors.New("project id is required")
	}
	if runID == "" {
		return modelExportPolicyDecision{}, errors.New("run id is required")
	}

	rows, err := api.db.QueryContext(ctx, selectModelExportPolicyDecisionsQuery, runID, projectID)
	if err != nil {
		return modelExportPolicyDecision{}, err
	}
	defer rows.Close()

	hasDecision := false
	requiresApproval := false
	for rows.Next() {
		var decision string
		if err := rows.Scan(&decision); err != nil {
			return modelExportPolicyDecision{}, err
		}
		hasDecision = true
		switch strings.ToLower(strings.TrimSpace(decision)) {
		case policy.EffectDeny:
			return modelExportPolicyDecision{Allowed: false, Code: modelExportPolicyDenied}, nil
		case policy.EffectRequireApproval:
			requiresApproval = true
		case policy.EffectAllow:
		default:
			return modelExportPolicyDecision{Allowed: false, Code: modelExportPolicyDenied}, nil
		}
	}
	if err := rows.Err(); err != nil {
		return modelExportPolicyDecision{}, err
	}
	if !hasDecision {
		return modelExportPolicyDecision{Allowed: false, Code: modelExportPolicyDecisionRequired}, nil
	}
	if !requiresApproval {
		return modelExportPolicyDecision{Allowed: true}, nil
	}

	approvalRows, err := api.db.QueryContext(ctx, selectModelExportPolicyApprovalsQuery, runID, projectID)
	if err != nil {
		return modelExportPolicyDecision{}, err
	}
	defer approvalRows.Close()

	hasApproval := false
	pending := false
	for approvalRows.Next() {
		var status string
		if err := approvalRows.Scan(&status); err != nil {
			return modelExportPolicyDecision{}, err
		}
		hasApproval = true
		switch strings.ToLower(strings.TrimSpace(status)) {
		case approvalStatusApproved:
		case approvalStatusDenied:
			return modelExportPolicyDecision{Allowed: false, Code: modelExportPolicyApprovalDenied}, nil
		case approvalStatusPending:
			pending = true
		default:
			pending = true
		}
	}
	if err := approvalRows.Err(); err != nil {
		return modelExportPolicyDecision{}, err
	}
	if !hasApproval || pending {
		return modelExportPolicyDecision{Allowed: false, Code: modelExportPolicyApprovalRequired}, nil
	}
	return modelExportPolicyDecision{Allowed: true}, nil
}
