package specvalidator

import (
	"testing"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
)

func TestValidateRunSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    domain.RunSpec
		wantErr bool
	}{
		{
			name:    "ok minimal run spec",
			spec:    minimalRunSpec(pipelineWithDatasetRef("training")),
			wantErr: false,
		},
		{
			name: "missing commit sha",
			spec: func() domain.RunSpec {
				rs := minimalRunSpec(pipelineWithDatasetRef("training"))
				rs.CodeRef.CommitSHA = ""
				return rs
			}(),
			wantErr: true,
		},
		{
			name: "missing dataset binding",
			spec: func() domain.RunSpec {
				rs := minimalRunSpec(pipelineWithDatasetRef("labels"))
				rs.DatasetBindings = map[string]string{}
				return rs
			}(),
			wantErr: true,
		},
		{
			name: "extra dataset binding",
			spec: func() domain.RunSpec {
				rs := minimalRunSpec(minimalPipelineSpec())
				rs.DatasetBindings = map[string]string{
					"extra": "dsv_001",
				}
				return rs
			}(),
			wantErr: true,
		},
		{
			name: "missing env hash",
			spec: func() domain.RunSpec {
				rs := minimalRunSpec(pipelineWithDatasetRef("training"))
				rs.EnvLock.EnvHash = ""
				return rs
			}(),
			wantErr: true,
		},
		{
			name: "missing image digests",
			spec: func() domain.RunSpec {
				rs := minimalRunSpec(pipelineWithDatasetRef("training"))
				rs.EnvLock.Images = nil
				return rs
			}(),
			wantErr: true,
		},
		{
			name: "missing lock id",
			spec: func() domain.RunSpec {
				rs := minimalRunSpec(pipelineWithDatasetRef("training"))
				rs.EnvLock.LockID = ""
				return rs
			}(),
			wantErr: true,
		},
		{
			name: "missing parameters",
			spec: func() domain.RunSpec {
				rs := minimalRunSpec(pipelineWithDatasetRef("training"))
				rs.Parameters = nil
				return rs
			}(),
			wantErr: true,
		},
		{
			name: "missing policy snapshot",
			spec: func() domain.RunSpec {
				rs := minimalRunSpec(pipelineWithDatasetRef("training"))
				rs.PolicySnapshot = domain.PolicySnapshot{}
				return rs
			}(),
			wantErr: true,
		},
		{
			name: "invalid repo url",
			spec: func() domain.RunSpec {
				rs := minimalRunSpec(pipelineWithDatasetRef("training"))
				rs.CodeRef.RepoURL = "not a url"
				return rs
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		if err := ValidateRunSpec(tt.spec); (err != nil) != tt.wantErr {
			t.Fatalf("%s: expected err=%v, got %v", tt.name, tt.wantErr, err)
		}
	}
}

func minimalRunSpec(pipeline domain.PipelineSpec) domain.RunSpec {
	return domain.RunSpec{
		RunSpecVersion: "1.0",
		ProjectID:      "proj_123",
		PipelineSpec:   pipeline,
		DatasetBindings: map[string]string{
			"training": "dsv_123",
		},
		CodeRef: domain.CodeRef{
			RepoURL:   "https://github.com/acme/repo",
			CommitSHA: "deadbeef",
		},
		EnvLock: domain.EnvLock{
			LockID:                       "lock_123",
			EnvironmentDefinitionID:      "envdef_123",
			EnvironmentDefinitionVersion: 1,
			Images: []domain.EnvironmentImage{
				{
					Name:   "runtime",
					Ref:    "ghcr.io/acme/runtime:latest",
					Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				},
			},
			EnvHash: "envhash",
		},
		Parameters: domain.Metadata{
			"lr": 0.1,
		},
		PolicySnapshot: minimalPolicySnapshot(),
	}
}

func minimalPolicySnapshot() domain.PolicySnapshot {
	return domain.PolicySnapshot{
		SnapshotVersion: "1.0",
		CapturedAt:      time.Now().UTC(),
		CapturedBy:      "actor",
		RBAC: domain.PolicySnapshotRBAC{
			Subject:   "actor",
			Roles:     []string{"admin"},
			ProjectID: "proj_123",
		},
		Policies:       []domain.PolicySnapshotPolicy{},
		SnapshotSHA256: "snapsha",
	}
}

func pipelineWithDatasetRef(ref string) domain.PipelineSpec {
	spec := minimalPipelineSpec()
	spec.Spec.Steps[0].Inputs.Datasets = []domain.PipelineDatasetInput{
		{
			Name:       "training",
			DatasetRef: ref,
		},
	}
	return spec
}
