package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/repo/postgres"
)

const policySnapshotVersion = "1.0"

func (api *experimentsAPI) buildPolicySnapshot(ctx context.Context, projectID string, identity auth.Identity, envLock domain.EnvLock) (domain.PolicySnapshot, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return domain.PolicySnapshot{}, errors.New("project id is required")
	}
	policies, err := api.loadActivePolicyVersions(ctx)
	if err != nil {
		return domain.PolicySnapshot{}, err
	}
	now := time.Now().UTC()
	snapshot := assemblePolicySnapshot(projectID, identity, envLock, policies, now)

	hash, err := hashPolicySnapshot(snapshot)
	if err != nil {
		return domain.PolicySnapshot{}, err
	}
	snapshot.SnapshotSHA256 = hash
	return snapshot, nil
}

func assemblePolicySnapshot(projectID string, identity auth.Identity, envLock domain.EnvLock, policies []policyVersionRecord, now time.Time) domain.PolicySnapshot {
	policySnapshots := make([]domain.PolicySnapshotPolicy, 0, len(policies))
	for _, record := range policies {
		policySnapshots = append(policySnapshots, domain.PolicySnapshotPolicy{
			PolicyID:        strings.TrimSpace(record.PolicyID),
			PolicyName:      strings.TrimSpace(record.PolicyName),
			PolicyVersionID: strings.TrimSpace(record.PolicyVersionID),
			PolicyVersion:   record.Version,
			PolicySHA256:    strings.TrimSpace(record.SpecSHA256),
			Status:          strings.TrimSpace(record.Status),
		})
	}
	sort.Slice(policySnapshots, func(i, j int) bool {
		if policySnapshots[i].PolicyID == policySnapshots[j].PolicyID {
			return policySnapshots[i].PolicyVersionID < policySnapshots[j].PolicyVersionID
		}
		return policySnapshots[i].PolicyID < policySnapshots[j].PolicyID
	})

	roles := identity.Roles
	if roles == nil {
		roles = []string{}
	}
	// Sort roles for deterministic hashing.
	sortedRoles := append([]string{}, roles...)
	sort.Strings(sortedRoles)

	rbacDecision := "allow"
	retention := domain.PolicySnapshotRetention{Mode: "not_configured"}
	network := domain.PolicySnapshotNetwork{Mode: "not_configured"}
	secrets := domain.PolicySnapshotSecrets{Mode: "not_configured"}
	templates := domain.PolicySnapshotTemplates{Mode: "not_configured"}
	if strings.TrimSpace(envLock.EnvironmentDefinitionID) != "" {
		templates = domain.PolicySnapshotTemplates{
			Mode:               "locked",
			AllowedTemplateIDs: []string{strings.TrimSpace(envLock.EnvironmentDefinitionID)},
		}
	}
	if strings.TrimSpace(envLock.NetworkClassRef) != "" {
		network = domain.PolicySnapshotNetwork{
			Mode:     "class_ref",
			ClassRef: strings.TrimSpace(envLock.NetworkClassRef),
		}
	}
	if strings.TrimSpace(envLock.SecretAccessClassRef) != "" {
		secrets = domain.PolicySnapshotSecrets{
			Mode:     "class_ref",
			ClassRef: strings.TrimSpace(envLock.SecretAccessClassRef),
		}
	}
	return domain.PolicySnapshot{
		SnapshotVersion: policySnapshotVersion,
		CapturedAt:      now,
		CapturedBy:      strings.TrimSpace(identity.Subject),
		RBAC: domain.PolicySnapshotRBAC{
			Subject:   strings.TrimSpace(identity.Subject),
			Roles:     sortedRoles,
			ProjectID: strings.TrimSpace(projectID),
			Decision:  rbacDecision,
		},
		Retention: retention,
		Network:   network,
		Secrets:   secrets,
		Templates: templates,
		Policies:  policySnapshots,
	}
}

type policySnapshotHashInput struct {
	SnapshotVersion string                         `json:"snapshotVersion"`
	RBAC            domain.PolicySnapshotRBAC      `json:"rbac"`
	Retention       domain.PolicySnapshotRetention `json:"retention,omitempty"`
	Network         domain.PolicySnapshotNetwork   `json:"network,omitempty"`
	Secrets         domain.PolicySnapshotSecrets   `json:"secrets,omitempty"`
	Templates       domain.PolicySnapshotTemplates `json:"templates,omitempty"`
	Policies        []domain.PolicySnapshotPolicy  `json:"policies"`
}

func hashPolicySnapshot(snapshot domain.PolicySnapshot) (string, error) {
	policies := append([]domain.PolicySnapshotPolicy{}, snapshot.Policies...)
	sort.Slice(policies, func(i, j int) bool {
		if policies[i].PolicyID == policies[j].PolicyID {
			return policies[i].PolicyVersionID < policies[j].PolicyVersionID
		}
		return policies[i].PolicyID < policies[j].PolicyID
	})
	roles := append([]string{}, snapshot.RBAC.Roles...)
	sort.Strings(roles)
	rbac := snapshot.RBAC
	rbac.Roles = roles

	payload := policySnapshotHashInput{
		SnapshotVersion: snapshot.SnapshotVersion,
		RBAC:            rbac,
		Retention:       snapshot.Retention,
		Network:         snapshot.Network,
		Secrets:         snapshot.Secrets,
		Templates:       snapshot.Templates,
		Policies:        policies,
	}
	blob, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return sha256HexBytes(blob), nil
}

func (api *experimentsAPI) ensureDatasetBindingsExist(ctx context.Context, projectID string, bindings map[string]string) error {
	if bindings == nil || len(bindings) == 0 {
		return nil
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return errors.New("project id is required")
	}
	seen := make(map[string]struct{}, len(bindings))
	for _, versionID := range bindings {
		versionID = strings.TrimSpace(versionID)
		if versionID == "" {
			continue
		}
		if _, ok := seen[versionID]; ok {
			continue
		}
		seen[versionID] = struct{}{}
		var ok int
		err := api.db.QueryRowContext(
			ctx,
			`SELECT 1 FROM dataset_versions WHERE project_id = $1 AND version_id = $2`,
			projectID,
			versionID,
		).Scan(&ok)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errDatasetVersionMissing
			}
			return err
		}
		if ok != 1 {
			return errDatasetVersionMissing
		}
	}
	return nil
}

func (api *experimentsAPI) persistRunBindings(ctx context.Context, store *postgres.RunBindingsStore, runID, projectID string, spec domain.RunSpec, createdBy string) error {
	if store == nil {
		return errors.New("run bindings store is required")
	}
	createdAt := spec.CreatedAt
	actor := strings.TrimSpace(createdBy)
	if actor == "" {
		actor = strings.TrimSpace(spec.CreatedBy)
	}

	codeRefIntegrity, err := integritySHA256(struct {
		RunID     string         `json:"run_id"`
		ProjectID string         `json:"project_id"`
		CodeRef   domain.CodeRef `json:"code_ref"`
		CreatedAt time.Time      `json:"created_at"`
		CreatedBy string         `json:"created_by"`
	}{
		RunID:     runID,
		ProjectID: projectID,
		CodeRef:   spec.CodeRef,
		CreatedAt: createdAt,
		CreatedBy: actor,
	})
	if err != nil {
		return err
	}
	if err := store.InsertCodeRef(ctx, runID, projectID, spec.CodeRef, createdAt, actor, codeRefIntegrity); err != nil {
		return err
	}

	if strings.TrimSpace(spec.EnvLock.LockID) == "" {
		return errors.New("env lock id is required")
	}
	lockIntegrity, err := integritySHA256(struct {
		RunID     string         `json:"run_id"`
		ProjectID string         `json:"project_id"`
		EnvLock   domain.EnvLock `json:"env_lock"`
		CreatedAt time.Time      `json:"created_at"`
		CreatedBy string         `json:"created_by"`
	}{
		RunID:     runID,
		ProjectID: projectID,
		EnvLock:   spec.EnvLock,
		CreatedAt: createdAt,
		CreatedBy: actor,
	})
	if err != nil {
		return err
	}
	if _, err := store.InsertEnvLock(ctx, runID, projectID, spec.EnvLock, createdAt, actor, lockIntegrity); err != nil {
		return err
	}

	snapshotJSON, err := json.Marshal(spec.PolicySnapshot)
	if err != nil {
		return err
	}
	policyIntegrity, err := integritySHA256(struct {
		RunID       string    `json:"run_id"`
		ProjectID   string    `json:"project_id"`
		SnapshotSHA string    `json:"snapshot_sha256"`
		CreatedAt   time.Time `json:"created_at"`
		CreatedBy   string    `json:"created_by"`
	}{
		RunID:       runID,
		ProjectID:   projectID,
		SnapshotSHA: spec.PolicySnapshot.SnapshotSHA256,
		CreatedAt:   createdAt,
		CreatedBy:   actor,
	})
	if err != nil {
		return err
	}
	if _, err := store.InsertPolicySnapshot(ctx, runID, projectID, spec.PolicySnapshot, snapshotJSON, createdAt, actor, policyIntegrity); err != nil {
		return err
	}
	return nil
}
