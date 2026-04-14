package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/platform/policy"
	"github.com/google/uuid"
)

const (
	policyStatusActive   = "active"
	policyStatusDisabled = "disabled"
)

const (
	approvalStatusPending  = "pending"
	approvalStatusApproved = "approved"
	approvalStatusDenied   = "denied"
)

type policySummary struct {
	PolicyID      string               `json:"policy_id"`
	Name          string               `json:"name"`
	Description   string               `json:"description,omitempty"`
	CreatedAt     time.Time            `json:"created_at"`
	CreatedBy     string               `json:"created_by"`
	LatestVersion policyVersionSummary `json:"latest_version,omitempty"`
}

type policyVersionSummary struct {
	PolicyVersionID string    `json:"policy_version_id"`
	Version         int       `json:"version"`
	Status          string    `json:"status"`
	SpecSHA256      string    `json:"spec_sha256"`
	CreatedAt       time.Time `json:"created_at"`
	CreatedBy       string    `json:"created_by"`
}

type policyListResponse struct {
	Policies []policySummary `json:"policies"`
}

type policyVersion struct {
	PolicyVersionID string          `json:"policy_version_id"`
	PolicyID        string          `json:"policy_id"`
	Version         int             `json:"version"`
	Status          string          `json:"status"`
	SpecYAML        string          `json:"spec_yaml"`
	Spec            json.RawMessage `json:"spec"`
	SpecSHA256      string          `json:"spec_sha256"`
	CreatedAt       time.Time       `json:"created_at"`
	CreatedBy       string          `json:"created_by"`
}

type policyVersionListResponse struct {
	PolicyID string          `json:"policy_id"`
	Versions []policyVersion `json:"versions"`
}

type createPolicyRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Spec        string `json:"spec"`
	Status      string `json:"status,omitempty"`
}

type createPolicyVersionRequest struct {
	Spec   string `json:"spec"`
	Status string `json:"status,omitempty"`
}

type policyDecisionSummary struct {
	DecisionID      string    `json:"decision_id"`
	RunID           string    `json:"run_id,omitempty"`
	PolicyID        string    `json:"policy_id"`
	PolicyName      string    `json:"policy_name,omitempty"`
	PolicyVersionID string    `json:"policy_version_id"`
	PolicySHA256    string    `json:"policy_sha256"`
	ContextSHA256   string    `json:"context_sha256"`
	Decision        string    `json:"decision"`
	RuleID          string    `json:"rule_id,omitempty"`
	Reason          string    `json:"reason,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	CreatedBy       string    `json:"created_by"`
}

type policyDecisionDetail struct {
	policyDecisionSummary
	Context json.RawMessage `json:"context"`
}

type policyDecisionListResponse struct {
	Decisions []policyDecisionSummary `json:"decisions"`
}

type policyApprovalSummary struct {
	ApprovalID      string     `json:"approval_id"`
	DecisionID      string     `json:"decision_id"`
	RunID           string     `json:"run_id,omitempty"`
	Status          string     `json:"status"`
	RequestedAt     time.Time  `json:"requested_at"`
	RequestedBy     string     `json:"requested_by"`
	DecidedAt       *time.Time `json:"decided_at,omitempty"`
	DecidedBy       string     `json:"decided_by,omitempty"`
	Reason          string     `json:"reason,omitempty"`
	PolicyID        string     `json:"policy_id"`
	PolicyName      string     `json:"policy_name"`
	PolicyVersionID string     `json:"policy_version_id"`
	Decision        string     `json:"decision"`
	RuleID          string     `json:"rule_id,omitempty"`
}

type policyApprovalDetail struct {
	policyApprovalSummary
	DecisionContext json.RawMessage `json:"decision_context,omitempty"`
}

type policyApprovalListResponse struct {
	Approvals []policyApprovalSummary `json:"approvals"`
}

type policyApprovalActionRequest struct {
	Reason string `json:"reason,omitempty"`
}

func (api *experimentsAPI) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	limit := clampInt(parseIntQuery(r, "limit", 100), 1, 500)

	rows, err := api.db.QueryContext(
		r.Context(),
		`SELECT p.policy_id,
				p.name,
				p.description,
				p.created_at,
				p.created_by,
				v.policy_version_id,
				v.version,
				v.status,
				v.spec_sha256,
				v.created_at,
				v.created_by
		 FROM policies p
		 LEFT JOIN LATERAL (
			SELECT policy_version_id, version, status, spec_sha256, created_at, created_by
			FROM policy_versions
			WHERE policy_id = p.policy_id
			ORDER BY version DESC
			LIMIT 1
		 ) v ON true
		 ORDER BY p.created_at DESC
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer rows.Close()

	out := make([]policySummary, 0, limit)
	for rows.Next() {
		var (
			policyID         string
			name             string
			desc             sql.NullString
			createdAt        time.Time
			createdBy        string
			versionID        sql.NullString
			versionNum       sql.NullInt64
			versionStatus    sql.NullString
			specSHA          sql.NullString
			versionCreatedAt sql.NullTime
			versionCreatedBy sql.NullString
		)
		if err := rows.Scan(
			&policyID,
			&name,
			&desc,
			&createdAt,
			&createdBy,
			&versionID,
			&versionNum,
			&versionStatus,
			&specSHA,
			&versionCreatedAt,
			&versionCreatedBy,
		); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}

		summary := policySummary{
			PolicyID:    policyID,
			Name:        name,
			Description: strings.TrimSpace(desc.String),
			CreatedAt:   createdAt,
			CreatedBy:   createdBy,
		}
		if versionID.Valid && versionNum.Valid && versionStatus.Valid && specSHA.Valid && versionCreatedAt.Valid && versionCreatedBy.Valid {
			summary.LatestVersion = policyVersionSummary{
				PolicyVersionID: versionID.String,
				Version:         int(versionNum.Int64),
				Status:          strings.TrimSpace(versionStatus.String),
				SpecSHA256:      strings.TrimSpace(specSHA.String),
				CreatedAt:       versionCreatedAt.Time.UTC(),
				CreatedBy:       strings.TrimSpace(versionCreatedBy.String),
			}
		}
		out = append(out, summary)
	}
	if err := rows.Err(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, policyListResponse{Policies: out})
}

func (api *experimentsAPI) handleGetPolicy(w http.ResponseWriter, r *http.Request) {
	policyID := strings.TrimSpace(r.PathValue("policy_id"))
	if policyID == "" {
		api.writeError(w, r, http.StatusBadRequest, "policy_id_required")
		return
	}

	var (
		name             string
		desc             sql.NullString
		createdAt        time.Time
		createdBy        string
		versionID        sql.NullString
		versionNum       sql.NullInt64
		versionStatus    sql.NullString
		specSHA          sql.NullString
		versionCreatedAt sql.NullTime
		versionCreatedBy sql.NullString
	)
	err := api.db.QueryRowContext(
		r.Context(),
		`SELECT p.name,
				p.description,
				p.created_at,
				p.created_by,
				v.policy_version_id,
				v.version,
				v.status,
				v.spec_sha256,
				v.created_at,
				v.created_by
		 FROM policies p
		 LEFT JOIN LATERAL (
			SELECT policy_version_id, version, status, spec_sha256, created_at, created_by
			FROM policy_versions
			WHERE policy_id = p.policy_id
			ORDER BY version DESC
			LIMIT 1
		 ) v ON true
		 WHERE p.policy_id = $1`,
		policyID,
	).Scan(
		&name,
		&desc,
		&createdAt,
		&createdBy,
		&versionID,
		&versionNum,
		&versionStatus,
		&specSHA,
		&versionCreatedAt,
		&versionCreatedBy,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	out := policySummary{
		PolicyID:    policyID,
		Name:        name,
		Description: strings.TrimSpace(desc.String),
		CreatedAt:   createdAt,
		CreatedBy:   createdBy,
	}
	if versionID.Valid && versionNum.Valid && versionStatus.Valid && specSHA.Valid && versionCreatedAt.Valid && versionCreatedBy.Valid {
		out.LatestVersion = policyVersionSummary{
			PolicyVersionID: versionID.String,
			Version:         int(versionNum.Int64),
			Status:          strings.TrimSpace(versionStatus.String),
			SpecSHA256:      strings.TrimSpace(specSHA.String),
			CreatedAt:       versionCreatedAt.Time.UTC(),
			CreatedBy:       strings.TrimSpace(versionCreatedBy.String),
		}
	}

	api.writeJSON(w, http.StatusOK, out)
}

func (api *experimentsAPI) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	var req createPolicyRequest
	if err := decodeJSON(r, &req); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		api.writeError(w, r, http.StatusBadRequest, "name_required")
		return
	}
	description := strings.TrimSpace(req.Description)
	specRaw := strings.TrimSpace(req.Spec)
	if specRaw == "" {
		api.writeError(w, r, http.StatusBadRequest, "spec_required")
		return
	}
	status, ok := normalizePolicyStatus(req.Status)
	if !ok {
		api.writeError(w, r, http.StatusBadRequest, "invalid_status")
		return
	}

	spec, err := policy.ParseSpec([]byte(specRaw))
	if err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_spec")
		return
	}
	specJSON, err := json.Marshal(spec)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	specSHA := sha256HexBytes(specJSON)

	now := time.Now().UTC()
	policyID := uuid.NewString()
	versionID := uuid.NewString()

	type policyIntegrityInput struct {
		PolicyID    string    `json:"policy_id"`
		Name        string    `json:"name"`
		Description string    `json:"description,omitempty"`
		CreatedAt   time.Time `json:"created_at"`
		CreatedBy   string    `json:"created_by"`
	}
	policyIntegrity, err := integritySHA256(policyIntegrityInput{
		PolicyID:    policyID,
		Name:        name,
		Description: description,
		CreatedAt:   now,
		CreatedBy:   identity.Subject,
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	type versionIntegrityInput struct {
		PolicyVersionID string          `json:"policy_version_id"`
		PolicyID        string          `json:"policy_id"`
		Version         int             `json:"version"`
		Status          string          `json:"status"`
		SpecYAML        string          `json:"spec_yaml"`
		Spec            json.RawMessage `json:"spec"`
		SpecSHA256      string          `json:"spec_sha256"`
		CreatedAt       time.Time       `json:"created_at"`
		CreatedBy       string          `json:"created_by"`
	}
	versionIntegrity, err := integritySHA256(versionIntegrityInput{
		PolicyVersionID: versionID,
		PolicyID:        policyID,
		Version:         1,
		Status:          status,
		SpecYAML:        specRaw,
		Spec:            specJSON,
		SpecSHA256:      specSHA,
		CreatedAt:       now,
		CreatedBy:       identity.Subject,
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

	_, err = tx.ExecContext(
		r.Context(),
		`INSERT INTO policies (
			policy_id,
			name,
			description,
			created_at,
			created_by,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6)`,
		policyID,
		name,
		nullString(description),
		now,
		identity.Subject,
		policyIntegrity,
	)
	if err != nil {
		if isUniqueViolation(err) {
			api.writeError(w, r, http.StatusConflict, "policy_name_exists")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	_, err = tx.ExecContext(
		r.Context(),
		`INSERT INTO policy_versions (
			policy_version_id,
			policy_id,
			version,
			status,
			spec_yaml,
			spec_json,
			spec_sha256,
			created_at,
			created_by,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		versionID,
		policyID,
		1,
		status,
		specRaw,
		specJSON,
		specSHA,
		now,
		identity.Subject,
		versionIntegrity,
	)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	_, err = auditlog.Insert(r.Context(), tx, auditlog.Event{
		OccurredAt:   now,
		Actor:        identity.Subject,
		Action:       "policy.create",
		ResourceType: "policy",
		ResourceID:   policyID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":    "experiments",
			"policy_id":  policyID,
			"name":       name,
			"status":     status,
			"version_id": versionID,
		},
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
		return
	}

	_, err = auditlog.Insert(r.Context(), tx, auditlog.Event{
		OccurredAt:   now,
		Actor:        identity.Subject,
		Action:       "policy.version.create",
		ResourceType: "policy_version",
		ResourceID:   versionID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":     "experiments",
			"policy_id":   policyID,
			"version":     1,
			"status":      status,
			"spec_sha256": specSHA,
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

	api.writeJSON(w, http.StatusCreated, policyVersion{
		PolicyVersionID: versionID,
		PolicyID:        policyID,
		Version:         1,
		Status:          status,
		SpecYAML:        specRaw,
		Spec:            specJSON,
		SpecSHA256:      specSHA,
		CreatedAt:       now,
		CreatedBy:       identity.Subject,
	})
}

func (api *experimentsAPI) handleListPolicyVersions(w http.ResponseWriter, r *http.Request) {
	policyID := strings.TrimSpace(r.PathValue("policy_id"))
	if policyID == "" {
		api.writeError(w, r, http.StatusBadRequest, "policy_id_required")
		return
	}
	limit := clampInt(parseIntQuery(r, "limit", 100), 1, 500)

	if ok, err := api.policyExists(r.Context(), policyID); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	} else if !ok {
		api.writeError(w, r, http.StatusNotFound, "not_found")
		return
	}

	rows, err := api.db.QueryContext(
		r.Context(),
		`SELECT policy_version_id,
				version,
				status,
				spec_yaml,
				spec_json,
				spec_sha256,
				created_at,
				created_by
		 FROM policy_versions
		 WHERE policy_id = $1
		 ORDER BY version DESC
		 LIMIT $2`,
		policyID,
		limit,
	)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer rows.Close()

	out := make([]policyVersion, 0, limit)
	for rows.Next() {
		var (
			versionID string
			version   int
			status    string
			specYAML  string
			specJSON  []byte
			specSHA   string
			createdAt time.Time
			createdBy string
		)
		if err := rows.Scan(&versionID, &version, &status, &specYAML, &specJSON, &specSHA, &createdAt, &createdBy); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		out = append(out, policyVersion{
			PolicyVersionID: versionID,
			PolicyID:        policyID,
			Version:         version,
			Status:          strings.TrimSpace(status),
			SpecYAML:        specYAML,
			Spec:            normalizeJSON(specJSON),
			SpecSHA256:      strings.TrimSpace(specSHA),
			CreatedAt:       createdAt,
			CreatedBy:       createdBy,
		})
	}
	if err := rows.Err(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, policyVersionListResponse{
		PolicyID: policyID,
		Versions: out,
	})
}

func (api *experimentsAPI) handleCreatePolicyVersion(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	policyID := strings.TrimSpace(r.PathValue("policy_id"))
	if policyID == "" {
		api.writeError(w, r, http.StatusBadRequest, "policy_id_required")
		return
	}

	var req createPolicyVersionRequest
	if err := decodeJSON(r, &req); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}
	specRaw := strings.TrimSpace(req.Spec)
	if specRaw == "" {
		api.writeError(w, r, http.StatusBadRequest, "spec_required")
		return
	}
	status, ok := normalizePolicyStatus(req.Status)
	if !ok {
		api.writeError(w, r, http.StatusBadRequest, "invalid_status")
		return
	}

	spec, err := policy.ParseSpec([]byte(specRaw))
	if err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_spec")
		return
	}
	specJSON, err := json.Marshal(spec)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	specSHA := sha256HexBytes(specJSON)

	tx, err := api.db.BeginTx(r.Context(), nil)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback() }()

	var existingName string
	err = tx.QueryRowContext(
		r.Context(),
		`SELECT name FROM policies WHERE policy_id = $1`,
		policyID,
	).Scan(&existingName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	var maxVersion int
	err = tx.QueryRowContext(
		r.Context(),
		`SELECT COALESCE(MAX(version), 0) FROM policy_versions WHERE policy_id = $1`,
		policyID,
	).Scan(&maxVersion)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	version := maxVersion + 1
	now := time.Now().UTC()
	versionID := uuid.NewString()

	type versionIntegrityInput struct {
		PolicyVersionID string          `json:"policy_version_id"`
		PolicyID        string          `json:"policy_id"`
		Version         int             `json:"version"`
		Status          string          `json:"status"`
		SpecYAML        string          `json:"spec_yaml"`
		Spec            json.RawMessage `json:"spec"`
		SpecSHA256      string          `json:"spec_sha256"`
		CreatedAt       time.Time       `json:"created_at"`
		CreatedBy       string          `json:"created_by"`
	}
	versionIntegrity, err := integritySHA256(versionIntegrityInput{
		PolicyVersionID: versionID,
		PolicyID:        policyID,
		Version:         version,
		Status:          status,
		SpecYAML:        specRaw,
		Spec:            specJSON,
		SpecSHA256:      specSHA,
		CreatedAt:       now,
		CreatedBy:       identity.Subject,
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	_, err = tx.ExecContext(
		r.Context(),
		`INSERT INTO policy_versions (
			policy_version_id,
			policy_id,
			version,
			status,
			spec_yaml,
			spec_json,
			spec_sha256,
			created_at,
			created_by,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		versionID,
		policyID,
		version,
		status,
		specRaw,
		specJSON,
		specSHA,
		now,
		identity.Subject,
		versionIntegrity,
	)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	_, err = auditlog.Insert(r.Context(), tx, auditlog.Event{
		OccurredAt:   now,
		Actor:        identity.Subject,
		Action:       "policy.version.create",
		ResourceType: "policy_version",
		ResourceID:   versionID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":     "experiments",
			"policy_id":   policyID,
			"name":        existingName,
			"version":     version,
			"status":      status,
			"spec_sha256": specSHA,
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

	api.writeJSON(w, http.StatusCreated, policyVersion{
		PolicyVersionID: versionID,
		PolicyID:        policyID,
		Version:         version,
		Status:          status,
		SpecYAML:        specRaw,
		Spec:            specJSON,
		SpecSHA256:      specSHA,
		CreatedAt:       now,
		CreatedBy:       identity.Subject,
	})
}

func (api *experimentsAPI) handleListPolicyDecisions(w http.ResponseWriter, r *http.Request) {
	limit := clampInt(parseIntQuery(r, "limit", 100), 1, 500)
	runID := strings.TrimSpace(r.URL.Query().Get("run_id"))

	query := `SELECT d.decision_id,
			d.run_id,
			d.policy_id,
			p.name,
			d.policy_version_id,
			d.policy_sha256,
			d.context_sha256,
			d.decision,
			d.rule_id,
			d.reason,
			d.created_at,
			d.created_by
		FROM policy_decisions d
		JOIN policies p ON p.policy_id = d.policy_id`
	args := []any{}
	if runID != "" {
		args = append(args, runID)
		query += " WHERE d.run_id = $" + strconv.Itoa(len(args))
	}
	args = append(args, limit)
	query += " ORDER BY d.created_at DESC LIMIT $" + strconv.Itoa(len(args))

	rows, err := api.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer rows.Close()

	out := make([]policyDecisionSummary, 0, limit)
	for rows.Next() {
		var (
			decisionID string
			runIDVal   sql.NullString
			policyID   string
			policyName string
			versionID  string
			policySHA  string
			contextSHA string
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
			&decision,
			&ruleID,
			&reason,
			&createdAt,
			&createdBy,
		); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}

		out = append(out, policyDecisionSummary{
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
		})
	}
	if err := rows.Err(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, policyDecisionListResponse{Decisions: out})
}

func (api *experimentsAPI) handleGetPolicyDecision(w http.ResponseWriter, r *http.Request) {
	decisionID := strings.TrimSpace(r.PathValue("decision_id"))
	if decisionID == "" {
		api.writeError(w, r, http.StatusBadRequest, "decision_id_required")
		return
	}

	var (
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
	err := api.db.QueryRowContext(
		r.Context(),
		`SELECT d.run_id,
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
		 WHERE d.decision_id = $1`,
		decisionID,
	).Scan(
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
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, policyDecisionDetail{
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
}

func (api *experimentsAPI) handleListPolicyApprovals(w http.ResponseWriter, r *http.Request) {
	limit := clampInt(parseIntQuery(r, "limit", 100), 1, 500)
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	runID := strings.TrimSpace(r.URL.Query().Get("run_id"))
	if statusFilter != "" {
		if _, ok := normalizeApprovalStatus(statusFilter); !ok {
			api.writeError(w, r, http.StatusBadRequest, "invalid_status")
			return
		}
	}

	query := `SELECT a.approval_id,
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
			d.rule_id
		FROM policy_approvals a
		JOIN policy_decisions d ON d.decision_id = a.decision_id
		JOIN policies p ON p.policy_id = d.policy_id`
	args := []any{}
	clauses := []string{}
	if statusFilter != "" {
		args = append(args, strings.ToLower(statusFilter))
		clauses = append(clauses, "a.status = $"+strconv.Itoa(len(args)))
	}
	if runID != "" {
		args = append(args, runID)
		clauses = append(clauses, "a.run_id = $"+strconv.Itoa(len(args)))
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	args = append(args, limit)
	query += " ORDER BY a.requested_at DESC LIMIT $" + strconv.Itoa(len(args))

	rows, err := api.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer rows.Close()

	out := make([]policyApprovalSummary, 0, limit)
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
		); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}

		var decidedAtPtr *time.Time
		if decidedAt.Valid && !decidedAt.Time.IsZero() {
			t := decidedAt.Time.UTC()
			decidedAtPtr = &t
		}

		out = append(out, policyApprovalSummary{
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
		})
	}
	if err := rows.Err(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, policyApprovalListResponse{Approvals: out})
}

func (api *experimentsAPI) handleGetPolicyApproval(w http.ResponseWriter, r *http.Request) {
	approvalID := strings.TrimSpace(r.PathValue("approval_id"))
	if approvalID == "" {
		api.writeError(w, r, http.StatusBadRequest, "approval_id_required")
		return
	}

	var (
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
	err := api.db.QueryRowContext(
		r.Context(),
		`SELECT a.decision_id,
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
		 WHERE a.approval_id = $1`,
		approvalID,
	).Scan(
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
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	var decidedAtPtr *time.Time
	if decidedAt.Valid && !decidedAt.Time.IsZero() {
		t := decidedAt.Time.UTC()
		decidedAtPtr = &t
	}

	api.writeJSON(w, http.StatusOK, policyApprovalDetail{
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
}

func (api *experimentsAPI) handleApprovePolicyApproval(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if !auth.HasAtLeast(identity.Roles, auth.RoleAdmin) {
		api.writeError(w, r, http.StatusForbidden, "approval_requires_admin")
		return
	}

	approvalID := strings.TrimSpace(r.PathValue("approval_id"))
	if approvalID == "" {
		api.writeError(w, r, http.StatusBadRequest, "approval_id_required")
		return
	}

	var req policyApprovalActionRequest
	if err := decodeJSON(r, &req); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}
	reason := strings.TrimSpace(req.Reason)

	tx, err := api.db.BeginTx(r.Context(), nil)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback() }()

	var (
		decisionID  string
		runID       string
		status      string
		requestedAt time.Time
		requestedBy string
	)
	err = tx.QueryRowContext(
		r.Context(),
		`SELECT decision_id, run_id, status, requested_at, requested_by
		 FROM policy_approvals
		 WHERE approval_id = $1
		 FOR UPDATE`,
		approvalID,
	).Scan(&decisionID, &runID, &status, &requestedAt, &requestedBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if strings.TrimSpace(status) != approvalStatusPending {
		api.writeError(w, r, http.StatusConflict, "approval_not_pending")
		return
	}
	if requestedBy == identity.Subject {
		api.writeError(w, r, http.StatusForbidden, "approval_requires_second_reviewer")
		return
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}

	decidedAt := time.Now().UTC()
	type approvalIntegrityInput struct {
		ApprovalID  string     `json:"approval_id"`
		DecisionID  string     `json:"decision_id"`
		RunID       string     `json:"run_id"`
		Status      string     `json:"status"`
		RequestedAt time.Time  `json:"requested_at"`
		RequestedBy string     `json:"requested_by"`
		DecidedAt   *time.Time `json:"decided_at,omitempty"`
		DecidedBy   string     `json:"decided_by,omitempty"`
		Reason      string     `json:"reason,omitempty"`
	}
	approvalIntegrity, err := integritySHA256(approvalIntegrityInput{
		ApprovalID:  approvalID,
		DecisionID:  decisionID,
		RunID:       runID,
		Status:      approvalStatusApproved,
		RequestedAt: requestedAt,
		RequestedBy: requestedBy,
		DecidedAt:   &decidedAt,
		DecidedBy:   identity.Subject,
		Reason:      reason,
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	_, err = tx.ExecContext(
		r.Context(),
		`UPDATE policy_approvals
		 SET status = $1,
			 decided_at = $2,
			 decided_by = $3,
			 reason = $4,
			 integrity_sha256 = $5
		 WHERE approval_id = $6`,
		approvalStatusApproved,
		decidedAt,
		identity.Subject,
		nullString(reason),
		approvalIntegrity,
		approvalID,
	)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	_, err = auditlog.Insert(r.Context(), tx, auditlog.Event{
		OccurredAt:   decidedAt,
		Actor:        identity.Subject,
		Action:       "policy.approval.approved",
		ResourceType: "policy_approval",
		ResourceID:   approvalID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":     "experiments",
			"approval_id": approvalID,
			"decision_id": decisionID,
			"run_id":      runID,
			"reason":      reason,
		},
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
		return
	}

	var deniedCount int
	err = tx.QueryRowContext(
		r.Context(),
		`SELECT COUNT(1)
		 FROM policy_approvals
		 WHERE run_id = $1 AND status = $2`,
		runID,
		approvalStatusDenied,
	).Scan(&deniedCount)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if deniedCount > 0 {
		if err := tx.Commit(); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		api.writeError(w, r, http.StatusConflict, "approval_denied")
		return
	}

	var pendingCount int
	err = tx.QueryRowContext(
		r.Context(),
		`SELECT COUNT(1)
		 FROM policy_approvals
		 WHERE run_id = $1 AND status = $2`,
		runID,
		approvalStatusPending,
	).Scan(&pendingCount)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if pendingCount > 0 {
		if err := tx.Commit(); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		api.writeJSON(w, http.StatusOK, map[string]any{
			"approval_id":     approvalID,
			"approval_status": approvalStatusApproved,
			"run_id":          runID,
			"run_status":      "pending",
			"pending":         pendingCount,
		})
		return
	}

	now := time.Now().UTC()
	if err := api.insertRunEvent(r.Context(), tx, runID, identity.Subject, "info", "run approved; awaiting dataplane dispatch", map[string]any{
		"approved_by":       identity.Subject,
		"approval_id":       approvalID,
		"dispatch_required": true,
	}); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	_, err = auditlog.Insert(r.Context(), tx, auditlog.Event{
		OccurredAt:   now,
		Actor:        identity.Subject,
		Action:       "experiment_run.approved",
		ResourceType: "experiment_run",
		ResourceID:   runID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":           "experiments",
			"run_id":            runID,
			"approval_id":       approvalID,
			"approval_status":   approvalStatusApproved,
			"dispatch_required": true,
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
	api.writeJSON(w, http.StatusOK, map[string]any{
		"approval_id":       approvalID,
		"approval_status":   approvalStatusApproved,
		"run_id":            runID,
		"run_status":        "pending",
		"dispatch_required": true,
	})
}

func (api *experimentsAPI) handleDenyPolicyApproval(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if !auth.HasAtLeast(identity.Roles, auth.RoleAdmin) {
		api.writeError(w, r, http.StatusForbidden, "approval_requires_admin")
		return
	}

	approvalID := strings.TrimSpace(r.PathValue("approval_id"))
	if approvalID == "" {
		api.writeError(w, r, http.StatusBadRequest, "approval_id_required")
		return
	}

	var req policyApprovalActionRequest
	if err := decodeJSON(r, &req); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}
	reason := strings.TrimSpace(req.Reason)

	tx, err := api.db.BeginTx(r.Context(), nil)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback() }()

	var (
		decisionID  string
		runID       string
		status      string
		requestedAt time.Time
		requestedBy string
	)
	err = tx.QueryRowContext(
		r.Context(),
		`SELECT decision_id, run_id, status, requested_at, requested_by
		 FROM policy_approvals
		 WHERE approval_id = $1
		 FOR UPDATE`,
		approvalID,
	).Scan(&decisionID, &runID, &status, &requestedAt, &requestedBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if strings.TrimSpace(status) != approvalStatusPending {
		api.writeError(w, r, http.StatusConflict, "approval_not_pending")
		return
	}
	if requestedBy == identity.Subject {
		api.writeError(w, r, http.StatusForbidden, "approval_requires_second_reviewer")
		return
	}

	decidedAt := time.Now().UTC()
	type approvalIntegrityInput struct {
		ApprovalID  string     `json:"approval_id"`
		DecisionID  string     `json:"decision_id"`
		RunID       string     `json:"run_id"`
		Status      string     `json:"status"`
		RequestedAt time.Time  `json:"requested_at"`
		RequestedBy string     `json:"requested_by"`
		DecidedAt   *time.Time `json:"decided_at,omitempty"`
		DecidedBy   string     `json:"decided_by,omitempty"`
		Reason      string     `json:"reason,omitempty"`
	}
	approvalIntegrity, err := integritySHA256(approvalIntegrityInput{
		ApprovalID:  approvalID,
		DecisionID:  decisionID,
		RunID:       runID,
		Status:      approvalStatusDenied,
		RequestedAt: requestedAt,
		RequestedBy: requestedBy,
		DecidedAt:   &decidedAt,
		DecidedBy:   identity.Subject,
		Reason:      reason,
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	_, err = tx.ExecContext(
		r.Context(),
		`UPDATE policy_approvals
		 SET status = $1,
			 decided_at = $2,
			 decided_by = $3,
			 reason = $4,
			 integrity_sha256 = $5
		 WHERE approval_id = $6`,
		approvalStatusDenied,
		decidedAt,
		identity.Subject,
		nullString(reason),
		approvalIntegrity,
		approvalID,
	)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	_, err = api.insertRunStateEvent(r.Context(), tx, runID, "canceled", decidedAt, map[string]any{
		"reason":      "policy_denied",
		"approval_id": approvalID,
		"decision_id": decisionID,
		"approved_by": identity.Subject,
		"deny_reason": reason,
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if err := api.insertRunEvent(r.Context(), tx, runID, identity.Subject, "info", "approval denied", map[string]any{
		"approval_id": approvalID,
		"decision_id": decisionID,
		"reason":      reason,
	}); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	_, err = auditlog.Insert(r.Context(), tx, auditlog.Event{
		OccurredAt:   decidedAt,
		Actor:        identity.Subject,
		Action:       "policy.approval.denied",
		ResourceType: "policy_approval",
		ResourceID:   approvalID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":     "experiments",
			"approval_id": approvalID,
			"decision_id": decisionID,
			"run_id":      runID,
			"reason":      reason,
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

	api.writeJSON(w, http.StatusOK, map[string]any{
		"approval_id":     approvalID,
		"approval_status": approvalStatusDenied,
		"run_id":          runID,
		"run_status":      "canceled",
	})
}

func (api *experimentsAPI) policyExists(ctx context.Context, policyID string) (bool, error) {
	var one int
	err := api.db.QueryRowContext(
		ctx,
		`SELECT 1 FROM policies WHERE policy_id = $1`,
		policyID,
	).Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func normalizePolicyStatus(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return policyStatusActive, true
	}
	switch value {
	case policyStatusActive, policyStatusDisabled:
		return value, true
	default:
		return "", false
	}
}

func normalizeApprovalStatus(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case approvalStatusPending, approvalStatusApproved, approvalStatusDenied:
		return value, true
	default:
		return "", false
	}
}
