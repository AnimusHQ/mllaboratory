package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
)

const (
	validDigest   = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	validImageRef = "ghcr.io/acme/train@" + validDigest
)

func TestBuildRunSpecValidationErrors(t *testing.T) {
	envLock := minimalEnvLock()
	tests := []struct {
		name    string
		req     createRunRequest
		wantErr error
	}{
		{
			name: "missing commit sha",
			req: createRunRequest{
				PipelineSpec:    rawSpec(minimalPipelineSpecJSON(validImageRef)),
				DatasetBindings: map[string]string{},
				CodeRef:         runSpecCodeRef{RepoURL: "https://github.com/acme/repo", CommitSHA: ""},
				EnvLock:         runSpecEnvLockRef{LockID: envLock.LockID},
				Parameters:      map[string]any{},
			},
			wantErr: errInvalidRunSpec,
		},
		{
			name: "missing dataset binding",
			req: createRunRequest{
				PipelineSpec:    rawSpec(pipelineSpecWithDatasetRef(validImageRef, "training")),
				DatasetBindings: map[string]string{},
				CodeRef:         runSpecCodeRef{RepoURL: "https://github.com/acme/repo", CommitSHA: "deadbeef"},
				EnvLock:         runSpecEnvLockRef{LockID: envLock.LockID},
				Parameters:      map[string]any{},
			},
			wantErr: errInvalidRunSpec,
		},
		{
			name: "non digest image",
			req: createRunRequest{
				PipelineSpec:    rawSpec(minimalPipelineSpecJSON("ghcr.io/acme/train:latest")),
				DatasetBindings: map[string]string{},
				CodeRef:         runSpecCodeRef{RepoURL: "https://github.com/acme/repo", CommitSHA: "deadbeef"},
				EnvLock:         runSpecEnvLockRef{LockID: envLock.LockID},
				Parameters:      map[string]any{},
			},
			wantErr: errInvalidPipelineSpec,
		},
		{
			name: "duplicate step names",
			req: createRunRequest{
				PipelineSpec:    rawSpec(pipelineSpecWithDuplicateSteps(validImageRef)),
				DatasetBindings: map[string]string{},
				CodeRef:         runSpecCodeRef{RepoURL: "https://github.com/acme/repo", CommitSHA: "deadbeef"},
				EnvLock:         runSpecEnvLockRef{LockID: envLock.LockID},
				Parameters:      map[string]any{},
			},
			wantErr: errInvalidPipelineSpec,
		},
		{
			name: "cycle detected",
			req: createRunRequest{
				PipelineSpec:    rawSpec(pipelineSpecWithCycle(validImageRef)),
				DatasetBindings: map[string]string{},
				CodeRef:         runSpecCodeRef{RepoURL: "https://github.com/acme/repo", CommitSHA: "deadbeef"},
				EnvLock:         runSpecEnvLockRef{LockID: envLock.LockID},
				Parameters:      map[string]any{},
			},
			wantErr: errInvalidPipelineSpec,
		},
		{
			name: "missing dataset bindings object",
			req: createRunRequest{
				PipelineSpec: rawSpec(minimalPipelineSpecJSON(validImageRef)),
				CodeRef:      runSpecCodeRef{RepoURL: "https://github.com/acme/repo", CommitSHA: "deadbeef"},
				EnvLock:      runSpecEnvLockRef{LockID: envLock.LockID},
				Parameters:   map[string]any{},
			},
			wantErr: errDatasetBindingsNeeded,
		},
	}

	for _, tc := range tests {
		if _, _, err := buildRunSpec("proj-1", "actor", tc.req, envLock, minimalPolicySnapshot()); err != tc.wantErr {
			t.Fatalf("%s: expected err %v, got %v", tc.name, tc.wantErr, err)
		}
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
			ProjectID: "proj-1",
			Decision:  "allow",
		},
		Retention:      domain.PolicySnapshotRetention{Mode: "not_configured"},
		Network:        domain.PolicySnapshotNetwork{Mode: "not_configured"},
		Secrets:        domain.PolicySnapshotSecrets{Mode: "not_configured"},
		Templates:      domain.PolicySnapshotTemplates{Mode: "not_configured"},
		Policies:       []domain.PolicySnapshotPolicy{},
		SnapshotSHA256: "snapsha",
	}
}

func minimalEnvLock() domain.EnvLock {
	return domain.EnvLock{
		LockID:                       "lock-1",
		EnvironmentDefinitionID:      "envdef-1",
		EnvironmentDefinitionVersion: 1,
		Images: []domain.EnvironmentImage{
			{
				Name:   "runtime",
				Ref:    "ghcr.io/acme/runtime:latest",
				Digest: validDigest,
			},
		},
		EnvHash: "envhash",
	}
}

func rawSpec(value string) json.RawMessage {
	return json.RawMessage(value)
}

func minimalPipelineSpecJSON(image string) string {
	return `{
  "apiVersion":"animus/v1alpha1",
  "kind":"Pipeline",
  "specVersion":"1.0",
  "spec":{
    "steps":[
      {
        "name":"step-a",
        "image":"` + image + `",
        "command":["echo"],
        "args":["ok"],
        "inputs":{"datasets":[],"artifacts":[]},
        "outputs":{"artifacts":[]},
        "env":[],
        "resources":{"cpu":"1","memory":"1Gi","gpu":0},
        "retryPolicy":{"maxAttempts":1,"backoff":{"type":"fixed","initialSeconds":0,"maxSeconds":0,"multiplier":1}}
      }
    ],
    "dependencies":[]
  }
}`
}

func pipelineSpecWithDatasetRef(image string, datasetRef string) string {
	return `{
  "apiVersion":"animus/v1alpha1",
  "kind":"Pipeline",
  "specVersion":"1.0",
  "spec":{
    "steps":[
      {
        "name":"step-a",
        "image":"` + image + `",
        "command":["echo"],
        "args":["ok"],
        "inputs":{"datasets":[{"name":"data","datasetRef":"` + datasetRef + `"}],"artifacts":[]},
        "outputs":{"artifacts":[]},
        "env":[],
        "resources":{"cpu":"1","memory":"1Gi","gpu":0},
        "retryPolicy":{"maxAttempts":1,"backoff":{"type":"fixed","initialSeconds":0,"maxSeconds":0,"multiplier":1}}
      }
    ],
    "dependencies":[]
  }
}`
}

func pipelineSpecWithDuplicateSteps(image string) string {
	return `{
  "apiVersion":"animus/v1alpha1",
  "kind":"Pipeline",
  "specVersion":"1.0",
  "spec":{
    "steps":[
      {
        "name":"step-a",
        "image":"` + image + `",
        "command":["echo"],
        "args":["one"],
        "inputs":{"datasets":[],"artifacts":[]},
        "outputs":{"artifacts":[]},
        "env":[],
        "resources":{"cpu":"1","memory":"1Gi","gpu":0},
        "retryPolicy":{"maxAttempts":1,"backoff":{"type":"fixed","initialSeconds":0,"maxSeconds":0,"multiplier":1}}
      },
      {
        "name":"step-a",
        "image":"` + image + `",
        "command":["echo"],
        "args":["two"],
        "inputs":{"datasets":[],"artifacts":[]},
        "outputs":{"artifacts":[]},
        "env":[],
        "resources":{"cpu":"1","memory":"1Gi","gpu":0},
        "retryPolicy":{"maxAttempts":1,"backoff":{"type":"fixed","initialSeconds":0,"maxSeconds":0,"multiplier":1}}
      }
    ],
    "dependencies":[]
  }
}`
}

func TestCreateRunRequiresIdempotencyKey(t *testing.T) {
	body := `{
  "pipelineSpec": {},
  "datasetBindings": {},
  "codeRef": {"repoUrl":"https://github.com/acme/repo","commitSha":"deadbeef"},
  "envLock": {"lockId":"lock-1"},
  "parameters": {}
}`
	req := httptest.NewRequest(http.MethodPost, "/projects/proj-1/runs", strings.NewReader(body))
	req.SetPathValue("project_id", "proj-1")
	req = req.WithContext(auth.ContextWithIdentity(req.Context(), auth.Identity{Subject: "actor"}))
	w := httptest.NewRecorder()

	api := &experimentsAPI{}
	api.handleCreateRun(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["error"] != "idempotency_key_required" {
		t.Fatalf("unexpected error code: %v", resp["error"])
	}
}

func pipelineSpecWithCycle(image string) string {
	return `{
  "apiVersion":"animus/v1alpha1",
  "kind":"Pipeline",
  "specVersion":"1.0",
  "spec":{
    "steps":[
      {
        "name":"step-a",
        "image":"` + image + `",
        "command":["echo"],
        "args":["one"],
        "inputs":{"datasets":[],"artifacts":[]},
        "outputs":{"artifacts":[]},
        "env":[],
        "resources":{"cpu":"1","memory":"1Gi","gpu":0},
        "retryPolicy":{"maxAttempts":1,"backoff":{"type":"fixed","initialSeconds":0,"maxSeconds":0,"multiplier":1}}
      },
      {
        "name":"step-b",
        "image":"` + image + `",
        "command":["echo"],
        "args":["two"],
        "inputs":{"datasets":[],"artifacts":[]},
        "outputs":{"artifacts":[]},
        "env":[],
        "resources":{"cpu":"1","memory":"1Gi","gpu":0},
        "retryPolicy":{"maxAttempts":1,"backoff":{"type":"fixed","initialSeconds":0,"maxSeconds":0,"multiplier":1}}
      }
    ],
    "dependencies":[
      {"from":"step-a","to":"step-b"},
      {"from":"step-b","to":"step-a"}
    ]
  }
}`
}
