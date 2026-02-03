package main

import (
	"testing"

	"github.com/animus-labs/animus-go/closed/internal/domain"
)

const (
	validDigest   = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	validImageRef = "ghcr.io/acme/runtime:latest"
)

func TestBuildJobSpecUsesEnvLockDigest(t *testing.T) {
	runSpec := minimalRunSpec("runtime", []domain.EnvironmentImage{
		{Name: "runtime", Ref: validImageRef, Digest: validDigest},
	})
	runSpec.EnvLock.NetworkClassRef = "net-class"
	runSpec.EnvLock.SecretAccessClassRef = "secret-class"

	job, err := buildJobSpec(runSpec, "run-1", "job-1", "ns", 0, "", "dispatch-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	container := job.Spec.Template.Spec.Containers[0]
	if container.Image != "ghcr.io/acme/runtime:latest@"+validDigest {
		t.Fatalf("unexpected image: %s", container.Image)
	}
	if got := job.Metadata.Labels["animus.run_id"]; got != "run-1" {
		t.Fatalf("run label: %s", got)
	}
	if got := job.Metadata.Labels["animus.project_id"]; got != "proj-1" {
		t.Fatalf("project label: %s", got)
	}
	if got := job.Metadata.Labels["animus.env_lock_id"]; got != "lock-1" {
		t.Fatalf("env lock label: %s", got)
	}
	if got := job.Metadata.Labels["animus.dispatch_id"]; got != "dispatch-1" {
		t.Fatalf("dispatch label: %s", got)
	}
	if got := job.Metadata.Labels["animus.network_class_ref"]; got != "net-class" {
		t.Fatalf("network class label: %s", got)
	}
	if got := job.Metadata.Labels["animus.secret_access_class_ref"]; got != "secret-class" {
		t.Fatalf("secret class label: %s", got)
	}

	if container.Resources.Requests["cpu"] != "2" {
		t.Fatalf("cpu request: %s", container.Resources.Requests["cpu"])
	}
	if container.Resources.Requests["memory"] != "512Mi" {
		t.Fatalf("memory request: %s", container.Resources.Requests["memory"])
	}
	if container.Resources.Limits["cpu"] != "4" {
		t.Fatalf("cpu limit: %s", container.Resources.Limits["cpu"])
	}
	if container.Resources.Limits["memory"] != "2Gi" {
		t.Fatalf("memory limit: %s", container.Resources.Limits["memory"])
	}

	env := map[string]string{}
	for _, entry := range container.Env {
		env[entry.Name] = entry.Value
	}
	if env["ANIMUS_RUN_ID"] != "run-1" {
		t.Fatalf("run id env missing")
	}
	if env["ANIMUS_PROJECT_ID"] != "proj-1" {
		t.Fatalf("project id env missing")
	}
	if env["ANIMUS_ENV_LOCK_ID"] != "lock-1" {
		t.Fatalf("env lock id env missing")
	}
	if env["ANIMUS_POLICY_SNAPSHOT_SHA"] != "policy-sha" {
		t.Fatalf("policy snapshot env missing")
	}
	if _, ok := env["ANIMUS_SKIP"]; ok {
		t.Fatalf("reserved env should be filtered")
	}
	if env["CUSTOM_VAR"] != "ok" {
		t.Fatalf("custom env missing")
	}
}

func TestBuildJobSpecRejectsMultipleSteps(t *testing.T) {
	runSpec := minimalRunSpec("runtime", []domain.EnvironmentImage{{Name: "runtime", Ref: validImageRef, Digest: validDigest}})
	runSpec.PipelineSpec.Spec.Steps = append(runSpec.PipelineSpec.Spec.Steps, runSpec.PipelineSpec.Spec.Steps[0])

	if _, err := buildJobSpec(runSpec, "run-1", "job-1", "", 0, "", "dispatch-1", nil); err == nil {
		t.Fatalf("expected error for multiple steps")
	}
}

func TestBuildJobSpecRejectsUnresolvedImage(t *testing.T) {
	runSpec := minimalRunSpec("ghcr.io/acme/train:latest", nil)

	if _, err := buildJobSpec(runSpec, "run-1", "job-1", "", 0, "", "dispatch-1", nil); err == nil {
		t.Fatalf("expected error for unresolved image")
	}
}

func TestBuildJobSpecAcceptsDigestImage(t *testing.T) {
	pinned := "ghcr.io/acme/train@" + validDigest
	runSpec := minimalRunSpec(pinned, nil)

	job, err := buildJobSpec(runSpec, "run-1", "job-1", "", 0, "", "dispatch-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := job.Spec.Template.Spec.Containers[0].Image; got != pinned {
		t.Fatalf("expected pinned image, got %s", got)
	}
}

func minimalRunSpec(stepImage string, images []domain.EnvironmentImage) domain.RunSpec {
	step := domain.PipelineStep{
		Name:    "step-a",
		Image:   stepImage,
		Command: []string{"/bin/echo"},
		Args:    []string{"ok"},
		Inputs:  domain.PipelineStepInputs{Datasets: []domain.PipelineDatasetInput{}, Artifacts: []domain.PipelineArtifactInput{}},
		Outputs: domain.PipelineStepOutputs{Artifacts: []domain.PipelineArtifactOutput{}},
		Env: []domain.EnvVar{
			{Name: "CUSTOM_VAR", Value: "ok"},
			{Name: "ANIMUS_SKIP", Value: "no"},
		},
		Resources: domain.PipelineResources{CPU: "2"},
	}

	return domain.RunSpec{
		ProjectID: "proj-1",
		PipelineSpec: domain.PipelineSpec{
			APIVersion:  "animus/v1alpha1",
			Kind:        "Pipeline",
			SpecVersion: "1.0",
			Spec: domain.PipelineSpecBody{
				Steps:        []domain.PipelineStep{step},
				Dependencies: []domain.PipelineDependency{},
			},
		},
		DatasetBindings: map[string]string{"training": "ds-1"},
		CodeRef:         domain.CodeRef{RepoURL: "https://github.com/acme/repo", CommitSHA: "deadbeef"},
		EnvLock: domain.EnvLock{
			LockID:           "lock-1",
			Images:           images,
			ResourceDefaults: domain.EnvironmentResources{CPU: "1", Memory: "512Mi"},
			ResourceLimits:   domain.EnvironmentResources{CPU: "4", Memory: "2Gi"},
			EnvHash:          "env-hash",
		},
		Parameters:     domain.Metadata{"lr": 0.1},
		PolicySnapshot: domain.PolicySnapshot{SnapshotSHA256: "policy-sha"},
	}
}
