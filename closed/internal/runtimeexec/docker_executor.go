package runtimeexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

type DockerExecutor struct {
	dockerBin string
}

func NewDockerExecutor(dockerBin string) (*DockerExecutor, error) {
	dockerBin = strings.TrimSpace(dockerBin)
	if dockerBin == "" {
		dockerBin = "docker"
	}
	if _, err := exec.LookPath(dockerBin); err != nil {
		return nil, fmt.Errorf("docker binary not found: %w", err)
	}
	return &DockerExecutor{dockerBin: dockerBin}, nil
}

func (e *DockerExecutor) Kind() string {
	return "docker"
}

func (e *DockerExecutor) ResolveImageID(ctx context.Context, imageRef string) (string, error) {
	imageRef = strings.TrimSpace(imageRef)
	if imageRef == "" {
		return "", errors.New("image ref is required")
	}

	cmd := exec.CommandContext(ctx, e.dockerBin, "image", "inspect", "--format", "{{.Id}}", imageRef)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		lower := strings.ToLower(text)
		if strings.Contains(lower, "no such image") || strings.Contains(lower, "not found") || strings.Contains(lower, "no such object") {
			return "", fmt.Errorf("%w: %s", ErrImageRefNotFound, text)
		}
		return "", fmt.Errorf("docker image inspect failed: %w: %s", err, text)
	}
	if text == "" {
		return "", fmt.Errorf("%w: empty docker image id", ErrImageRefNotFound)
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return "", fmt.Errorf("%w: empty docker image id", ErrImageRefNotFound)
	}
	return strings.TrimSpace(fields[0]), nil
}

func (e *DockerExecutor) Submit(ctx context.Context, spec JobSpec) error {
	name := strings.TrimSpace(spec.DockerName)
	if name == "" {
		return errors.New("docker container name is required")
	}
	imageRef := strings.TrimSpace(spec.ImageRef)
	if imageRef == "" {
		return errors.New("image ref is required")
	}

	jobKind := strings.TrimSpace(spec.JobKind)
	if jobKind == "" {
		jobKind = "training"
	}

	args := []string{
		"run",
		"--detach",
		"--name", name,
		"--network", "host",
		"-e", "RUN_ID=" + spec.RunID,
		"-e", "DATASET_VERSION_ID=" + spec.DatasetVersionID,
		"-e", "DATAPILOT_URL=" + spec.DatapilotURL,
		"-e", "TOKEN=" + spec.Token,
		"-e", "ANIMUS_JOB_KIND=" + jobKind,
	}

	if len(spec.Env) > 0 {
		keys := make([]string, 0, len(spec.Env))
		for k := range spec.Env {
			key := strings.TrimSpace(k)
			if key == "" || isReservedJobEnvKey(key) {
				continue
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			args = append(args, "-e", key+"="+spec.Env[key])
		}
	}

	if gpus := parseIntResource(spec.Resources, "gpus"); gpus > 0 {
		args = append(args, "--gpus", strconv.Itoa(gpus))
	}
	if cpu, ok := spec.Resources["cpu"].(string); ok && strings.TrimSpace(cpu) != "" {
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(cpu), 64); err == nil && parsed > 0 {
			args = append(args, "--cpus", fmt.Sprintf("%g", parsed))
		}
	}
	if mem, ok := spec.Resources["memory"].(string); ok && strings.TrimSpace(mem) != "" {
		args = append(args, "--memory", strings.TrimSpace(mem))
	}

	args = append(args, imageRef)

	cmd := exec.CommandContext(ctx, e.dockerBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker run failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

type dockerInspectState struct {
	Status     string    `json:"Status"`
	ExitCode   int       `json:"ExitCode"`
	FinishedAt time.Time `json:"FinishedAt"`
}

func (e *DockerExecutor) Inspect(ctx context.Context, execution Execution) (Observation, error) {
	name := strings.TrimSpace(execution.DockerContainer)
	if name == "" {
		return Observation{}, errors.New("docker container name is required")
	}

	cmd := exec.CommandContext(ctx, e.dockerBin, "inspect", "--format", "{{json .State}}", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(out))
		if strings.Contains(text, "No such object") || strings.Contains(text, "not found") {
			return Observation{Status: "pending", Message: "container_not_found"}, nil
		}
		return Observation{}, fmt.Errorf("docker inspect failed: %w: %s", err, text)
	}

	var state dockerInspectState
	if err := json.Unmarshal(out, &state); err != nil {
		return Observation{}, fmt.Errorf("parse docker inspect: %w", err)
	}

	status := "pending"
	switch strings.ToLower(strings.TrimSpace(state.Status)) {
	case "running":
		status = "running"
	case "exited":
		if state.ExitCode == 0 {
			status = "succeeded"
		} else {
			status = "failed"
		}
	}

	return Observation{
		Status:  status,
		Message: strings.TrimSpace(state.Status),
		Details: map[string]any{
			"docker_container": name,
			"exit_code":        state.ExitCode,
			"finished_at":      state.FinishedAt,
		},
	}, nil
}
