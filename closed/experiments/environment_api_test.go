package main

import (
	"testing"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
)

func TestNormalizeEnvironmentDefinitionRejectsDigestRef(t *testing.T) {
	req := environmentDefinitionRequest{
		Name: "base",
		BaseImages: []domain.EnvironmentBaseImage{
			{Name: "runtime", Ref: "ghcr.io/acme/runtime@sha256:deadbeef"},
		},
	}
	if _, err := normalizeEnvironmentDefinitionRequest(req, false); err == nil {
		t.Fatal("expected error for digest base image ref")
	}
}

func TestBuildEnvironmentLockValidation(t *testing.T) {
	def := domain.EnvironmentDefinition{
		ID:      "envdef-1",
		Name:    "base",
		Version: 1,
		BaseImages: []domain.EnvironmentBaseImage{
			{Name: "runtime", Ref: "ghcr.io/acme/runtime:latest"},
		},
		AllowedAccelerators: []string{"cpu"},
	}
	req := environmentLockRequest{
		EnvironmentDefinitionID: def.ID,
		ImageDigests:            map[string]string{},
	}
	if _, err := buildEnvironmentLock(def, req, "lock-1", time.Now().UTC()); err == nil {
		t.Fatal("expected error for missing image digest")
	}
}

func TestEnvironmentDefinitionToLockToRunFlow(t *testing.T) {
	def := domain.EnvironmentDefinition{
		ID:      "envdef-1",
		Name:    "base",
		Version: 1,
		BaseImages: []domain.EnvironmentBaseImage{
			{Name: "runtime", Ref: "ghcr.io/acme/runtime:latest"},
		},
		AllowedAccelerators: []string{"cpu"},
	}
	lockReq := environmentLockRequest{
		EnvironmentDefinitionID: def.ID,
		ImageDigests: map[string]string{
			"runtime": validDigest,
		},
	}
	lock, err := buildEnvironmentLock(def, lockReq, "lock-1", time.Now().UTC())
	if err != nil {
		t.Fatalf("build lock: %v", err)
	}
	runReq := createRunRequest{
		PipelineSpec:    rawSpec(minimalPipelineSpecJSON(validImageRef)),
		DatasetBindings: map[string]string{},
		CodeRef:         runSpecCodeRef{RepoURL: "https://github.com/acme/repo", CommitSHA: "deadbeef"},
		EnvLock:         runSpecEnvLockRef{LockID: lock.LockID},
		Parameters:      map[string]any{},
	}
	_, runSpec, err := buildRunSpec("proj-1", "actor", runReq, lock, minimalPolicySnapshot())
	if err != nil {
		t.Fatalf("build run spec: %v", err)
	}
	if runSpec.EnvLock.LockID != lock.LockID {
		t.Fatalf("expected lock id %s, got %s", lock.LockID, runSpec.EnvLock.LockID)
	}
}
