package specvalidator

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"github.com/animus-labs/animus-go/closed/internal/domain"
)

// ValidateRunSpec performs strict validation of a RunSpec.
func ValidateRunSpec(spec domain.RunSpec) error {
	issues := &ValidationError{}

	if strings.TrimSpace(spec.RunSpecVersion) == "" {
		issues.Add("runSpecVersion is required")
	}
	if strings.TrimSpace(spec.ProjectID) == "" {
		issues.Add("projectId is required")
	}
	repoURL := strings.TrimSpace(spec.CodeRef.RepoURL)
	if repoURL == "" {
		issues.Add("codeRef.repoUrl is required")
	} else if !validRepoURL(repoURL) {
		issues.Add("codeRef.repoUrl is invalid")
	}
	commit := strings.TrimSpace(spec.CodeRef.CommitSHA)
	if commit == "" {
		issues.Add("codeRef.commitSha is required")
	} else if !validCommitSHA(commit) {
		issues.Add("codeRef.commitSha must be hex (7..64)")
	}
	if strings.TrimSpace(spec.EnvLock.EnvHash) == "" {
		issues.Add("envLock.envHash is required")
	}
	if strings.TrimSpace(spec.EnvLock.LockID) == "" {
		issues.Add("envLock.lockId is required")
	}
	if strings.TrimSpace(spec.EnvLock.EnvironmentDefinitionID) == "" {
		issues.Add("envLock.environmentDefinitionId is required")
	}
	if spec.EnvLock.EnvironmentDefinitionVersion <= 0 {
		issues.Add("envLock.environmentDefinitionVersion is required")
	}
	if spec.DatasetBindings == nil {
		issues.Add("datasetBindings is required")
	}
	if spec.Parameters == nil {
		issues.Add("parameters is required")
	}
	if spec.PolicySnapshot.SnapshotVersion == "" {
		issues.Add("policySnapshot.snapshotVersion is required")
	}
	if spec.PolicySnapshot.SnapshotSHA256 == "" {
		issues.Add("policySnapshot.snapshotSha256 is required")
	}
	if spec.PolicySnapshot.CapturedAt.IsZero() {
		issues.Add("policySnapshot.capturedAt is required")
	}
	if strings.TrimSpace(spec.PolicySnapshot.RBAC.Subject) == "" {
		issues.Add("policySnapshot.rbac.subject is required")
	}
	if strings.TrimSpace(spec.PolicySnapshot.RBAC.ProjectID) == "" {
		issues.Add("policySnapshot.rbac.projectId is required")
	}
	if spec.PolicySnapshot.RBAC.Roles == nil {
		issues.Add("policySnapshot.rbac.roles is required")
	}
	if spec.PolicySnapshot.Policies == nil {
		issues.Add("policySnapshot.policies is required")
	}

	if err := ValidatePipelineSpec(spec.PipelineSpec); err != nil {
		issues.Add(fmt.Sprintf("pipelineSpec invalid: %s", err.Error()))
	}

	if spec.DatasetBindings != nil {
		for key, value := range spec.DatasetBindings {
			if strings.TrimSpace(key) == "" {
				issues.Add("datasetBindings contains empty datasetRef")
			}
			if strings.TrimSpace(value) == "" {
				issues.Add(fmt.Sprintf("datasetBinding for %q is empty", key))
			}
		}
	}

	refs := pipelineDatasetRefs(spec.PipelineSpec)
	for ref := range refs {
		if spec.DatasetBindings == nil {
			break
		}
		if value, ok := spec.DatasetBindings[ref]; !ok || strings.TrimSpace(value) == "" {
			issues.Add(fmt.Sprintf("missing dataset binding for %q", ref))
		}
	}
	if spec.DatasetBindings != nil {
		for ref := range spec.DatasetBindings {
			if _, ok := refs[ref]; !ok {
				issues.Add(fmt.Sprintf("dataset binding %q not referenced in pipeline", ref))
			}
		}
	}

	if len(spec.EnvLock.Images) == 0 {
		issues.Add("envLock.images is required")
	} else {
		for _, image := range spec.EnvLock.Images {
			if strings.TrimSpace(image.Name) == "" {
				issues.Add("envLock.images[].name is required")
			}
			if strings.TrimSpace(image.Ref) == "" {
				issues.Add("envLock.images[].ref is required")
			}
			digest := strings.TrimSpace(image.Digest)
			if digest == "" {
				issues.Add("envLock.images[].digest is required")
			} else if !strings.HasPrefix(digest, "sha256:") {
				issues.Add("envLock.images[].digest must be sha256")
			}
		}
	}
	if spec.EnvLock.ResourceDefaults.GPU < 0 || spec.EnvLock.ResourceLimits.GPU < 0 {
		issues.Add("envLock resource GPU must be >= 0")
	}
	for _, accel := range spec.EnvLock.AllowedAccelerators {
		accel = strings.ToLower(strings.TrimSpace(accel))
		if accel == "" {
			continue
		}
		if accel != "cpu" && accel != "gpu" {
			issues.Add("envLock.allowedAccelerators contains unsupported value")
			break
		}
	}

	if spec.Parameters != nil {
		for key := range spec.Parameters {
			if strings.TrimSpace(key) == "" {
				issues.Add("parameters must not contain empty keys")
			}
		}
	}

	for _, policy := range spec.PolicySnapshot.Policies {
		if strings.TrimSpace(policy.PolicyID) == "" {
			issues.Add("policySnapshot.policies[].policyId is required")
		}
		if strings.TrimSpace(policy.PolicyVersionID) == "" {
			issues.Add("policySnapshot.policies[].policyVersionId is required")
		}
		if strings.TrimSpace(policy.PolicySHA256) == "" {
			issues.Add("policySnapshot.policies[].policySha256 is required")
		}
	}

	return issues.OrNil()
}

func validRepoURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	if strings.HasPrefix(raw, "git@") {
		parts := strings.SplitN(raw, ":", 2)
		if len(parts) != 2 {
			return false
		}
		host := strings.TrimSpace(strings.TrimPrefix(parts[0], "git@"))
		path := strings.TrimSpace(parts[1])
		return host != "" && path != ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	return true
}

func validCommitSHA(raw string) bool {
	raw = strings.TrimSpace(raw)
	if len(raw) < 7 || len(raw) > 64 {
		return false
	}
	for _, r := range raw {
		if !unicode.IsDigit(r) && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

func pipelineDatasetRefs(spec domain.PipelineSpec) map[string]struct{} {
	refs := make(map[string]struct{})
	for _, step := range spec.Spec.Steps {
		for _, dataset := range step.Inputs.Datasets {
			ref := strings.TrimSpace(dataset.DatasetRef)
			if ref == "" {
				continue
			}
			refs[ref] = struct{}{}
		}
	}
	return refs
}
