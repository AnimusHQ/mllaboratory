package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/dataplane"
	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/k8s"
	"github.com/google/uuid"
)

var errJobNotFound = errors.New("job not found")

const (
	jobStatePending   = "pending"
	jobStateRunning   = "running"
	jobStateSucceeded = "succeeded"
	jobStateFailed    = "failed"
)

type runTracker struct {
	RunID      string
	ProjectID  string
	DispatchID string
	JobName    string
	Namespace  string
	EnvLockID  string
	PolicySHA  string
	StartedAt  time.Time

	missingCount int
}

type jobStatus struct {
	State      string
	Reason     string
	StartedAt  *time.Time
	FinishedAt *time.Time
	Details    map[string]any
}

func (api *dataplaneAPI) monitorRun(tracker *runTracker) {
	lastHeartbeat := time.Time{}
	pollInterval := api.cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = 10 * time.Second
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for range ticker.C {
		status, err := inspectJob(context.Background(), api.k8s, tracker.Namespace, tracker.JobName)
		if err != nil {
			if errors.Is(err, errJobNotFound) {
				tracker.missingCount++
				if tracker.missingCount >= 3 {
					api.emitTerminal(tracker, jobStateFailed, "job_not_found", nil)
					api.removeTracker(tracker.RunID)
					return
				}
			} else if api.logger != nil {
				api.logger.Warn("job inspect failed", "run_id", tracker.RunID, "error", err)
			}
			continue
		}
		tracker.missingCount = 0

		now := time.Now().UTC()
		if lastHeartbeat.IsZero() || now.Sub(lastHeartbeat) >= api.cfg.HeartbeatInterval {
			api.emitHeartbeat(tracker, status)
			lastHeartbeat = now
		}

		switch status.State {
		case jobStateSucceeded:
			api.emitTerminal(tracker, jobStateSucceeded, status.Reason, status.FinishedAt)
			api.removeTracker(tracker.RunID)
			return
		case jobStateFailed:
			api.emitTerminal(tracker, jobStateFailed, status.Reason, status.FinishedAt)
			api.removeTracker(tracker.RunID)
			return
		}
	}
}

func (api *dataplaneAPI) emitHeartbeat(tracker *runTracker, status jobStatus) {
	if api.cp == nil {
		return
	}
	event := dataplane.RunHeartbeat{
		EventID:   uuid.NewString(),
		RunID:     tracker.RunID,
		ProjectID: tracker.ProjectID,
		EmittedAt: time.Now().UTC(),
		Details: map[string]any{
			"job_state": status.State,
			"reason":    status.Reason,
			"job_name":  tracker.JobName,
			"namespace": tracker.Namespace,
			"details":   status.Details,
		},
	}
	_, _, _ = api.cp.SendHeartbeat(context.Background(), event, "")
}

func (api *dataplaneAPI) emitTerminal(tracker *runTracker, state string, reason string, finishedAt *time.Time) {
	if api.cp == nil {
		return
	}
	terminalState := mapJobStateToTerminal(state)
	eventID := uuid.NewString()
	for {
		event := dataplane.RunTerminalState{
			EventID:    eventID,
			RunID:      tracker.RunID,
			ProjectID:  tracker.ProjectID,
			State:      terminalState,
			EmittedAt:  time.Now().UTC(),
			FinishedAt: finishedAt,
			Reason:     strings.TrimSpace(reason),
			Details: map[string]any{
				"job_name":   tracker.JobName,
				"namespace":  tracker.Namespace,
				"env_lock":   tracker.EnvLockID,
				"policy_sha": tracker.PolicySHA,
			},
		}
		if _, _, err := api.cp.SendTerminal(context.Background(), event, ""); err == nil {
			return
		}
		time.Sleep(5 * time.Second)
	}
}

func mapJobStateToTerminal(state string) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case jobStateSucceeded:
		return "succeeded"
	case jobStateFailed:
		return "failed"
	default:
		return "failed"
	}
}

func inspectJob(ctx context.Context, client *k8s.Client, namespace, jobName string) (jobStatus, error) {
	job, err := client.GetJob(ctx, namespace, jobName)
	if err != nil {
		if errors.Is(err, k8s.ErrNotFound) {
			return jobStatus{}, errJobNotFound
		}
		return jobStatus{}, err
	}

	status := jobStatus{
		State:   jobStatePending,
		Reason:  "",
		Details: map[string]any{},
	}

	if job.Status.StartTime != nil {
		started := job.Status.StartTime.UTC()
		status.StartedAt = &started
	}
	if job.Status.CompletionTime != nil {
		finished := job.Status.CompletionTime.UTC()
		status.FinishedAt = &finished
	}

	for _, cond := range job.Status.Conditions {
		if strings.EqualFold(cond.Type, "Failed") && strings.EqualFold(cond.Status, "True") {
			status.State = jobStateFailed
			status.Reason = strings.TrimSpace(cond.Reason)
			if status.Reason == "" {
				status.Reason = strings.TrimSpace(cond.Message)
			}
			break
		}
		if strings.EqualFold(cond.Type, "Complete") && strings.EqualFold(cond.Status, "True") {
			status.State = jobStateSucceeded
			status.Reason = strings.TrimSpace(cond.Reason)
			if status.Reason == "" {
				status.Reason = strings.TrimSpace(cond.Message)
			}
			break
		}
	}

	if status.State == jobStatePending && job.Status.Active > 0 {
		status.State = jobStateRunning
	}

	status.Details["active"] = job.Status.Active
	status.Details["succeeded"] = job.Status.Succeeded
	status.Details["failed"] = job.Status.Failed

	return status, nil
}

func jobNameForRun(runID string) string {
	base := "animus-run-" + sanitizeName(runID)
	if len(base) <= 63 {
		return base
	}
	hash := sha256.Sum256([]byte(runID))
	suffix := hex.EncodeToString(hash[:6])
	trim := 63 - len(suffix) - 1
	if trim < 1 {
		trim = 1
	}
	return base[:trim] + "-" + suffix
}

func sanitizeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "run"
	}
	out := make([]rune, 0, len(value))
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			out = append(out, r)
		case r >= '0' && r <= '9':
			out = append(out, r)
		case r == '-':
			out = append(out, r)
		default:
			out = append(out, '-')
		}
	}
	clean := strings.Trim(outToString(out), "-")
	if clean == "" {
		return "run"
	}
	return clean
}

func outToString(runes []rune) string {
	if len(runes) == 0 {
		return ""
	}
	return string(runes)
}

func buildJobSpec(runSpec domain.RunSpec, runID, jobName, namespace string, ttlSeconds int32, serviceAccount, dispatchID string) (k8s.Job, error) {
	steps := runSpec.PipelineSpec.Spec.Steps
	if len(steps) != 1 {
		return k8s.Job{}, errors.New("single step pipeline required")
	}
	step := steps[0]

	image := resolveStepImage(step.Image, runSpec.EnvLock.Images)
	if strings.TrimSpace(image) == "" {
		return k8s.Job{}, errors.New("image resolution failed")
	}

	resources := mergeResources(step.Resources, runSpec.EnvLock.ResourceDefaults)
	limits := runSpec.EnvLock.ResourceLimits
	containerResources := buildResourceRequirements(resources, limits)

	labels := map[string]string{
		"app.kubernetes.io/name":      "animus-dataplane",
		"app.kubernetes.io/component": "run",
		"animus.run_id":               runID,
		"animus.project_id":           runSpec.ProjectID,
		"animus.env_lock_id":          runSpec.EnvLock.LockID,
		"animus.dispatch_id":          dispatchID,
	}
	if strings.TrimSpace(runSpec.EnvLock.NetworkClassRef) != "" {
		labels["animus.network_class_ref"] = strings.TrimSpace(runSpec.EnvLock.NetworkClassRef)
	}
	if strings.TrimSpace(runSpec.EnvLock.SecretAccessClassRef) != "" {
		labels["animus.secret_access_class_ref"] = strings.TrimSpace(runSpec.EnvLock.SecretAccessClassRef)
	}
	labels = filterLabelLength(labels)

	container := k8s.Container{
		Name:      "runner",
		Image:     image,
		Command:   step.Command,
		Args:      step.Args,
		Env:       buildEnvVars(runSpec, runID, step),
		Resources: containerResources,
	}

	podSpec := k8s.PodSpec{
		RestartPolicy: "Never",
		Containers:    []k8s.Container{container},
	}
	if strings.TrimSpace(serviceAccount) != "" {
		podSpec.ServiceAccountName = strings.TrimSpace(serviceAccount)
	}

	backoff := int32(0)
	var ttl *int32
	if ttlSeconds > 0 {
		ttl = &ttlSeconds
	}

	job := k8s.Job{
		Metadata: k8s.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: k8s.JobSpec{
			BackoffLimit: &backoff,
			Template: k8s.PodTemplateSpec{
				Metadata: k8s.ObjectMeta{Labels: labels},
				Spec:     podSpec,
			},
			TTLSecondsAfterFinished: ttl,
		},
	}
	return job, nil
}

func resolveStepImage(stepImage string, images []domain.EnvironmentImage) string {
	stepImage = strings.TrimSpace(stepImage)
	if stepImage == "" {
		return ""
	}
	for _, image := range images {
		if strings.EqualFold(image.Name, stepImage) {
			return imageRefWithDigest(image.Ref, image.Digest)
		}
	}
	if strings.Contains(stepImage, "@sha256:") {
		return stepImage
	}
	return ""
}

func imageRefWithDigest(ref, digest string) string {
	ref = strings.TrimSpace(ref)
	digest = strings.TrimSpace(digest)
	if ref == "" {
		return ""
	}
	if digest == "" {
		return ref
	}
	if strings.Contains(ref, "@") {
		parts := strings.SplitN(ref, "@", 2)
		ref = parts[0]
	}
	return ref + "@" + digest
}

func mergeResources(step domain.PipelineResources, defaults domain.EnvironmentResources) domain.EnvironmentResources {
	out := defaults
	if strings.TrimSpace(step.CPU) != "" {
		out.CPU = step.CPU
	}
	if strings.TrimSpace(step.Memory) != "" {
		out.Memory = step.Memory
	}
	if step.GPU > 0 {
		out.GPU = step.GPU
	}
	return out
}

func buildResourceRequirements(requests domain.EnvironmentResources, limits domain.EnvironmentResources) k8s.ResourceRequirements {
	out := k8s.ResourceRequirements{}
	if strings.TrimSpace(requests.CPU) != "" || strings.TrimSpace(requests.Memory) != "" || requests.GPU > 0 {
		out.Requests = map[string]string{}
		if strings.TrimSpace(requests.CPU) != "" {
			out.Requests["cpu"] = strings.TrimSpace(requests.CPU)
		}
		if strings.TrimSpace(requests.Memory) != "" {
			out.Requests["memory"] = strings.TrimSpace(requests.Memory)
		}
		if requests.GPU > 0 {
			out.Requests["nvidia.com/gpu"] = strconv.Itoa(requests.GPU)
		}
	}
	if strings.TrimSpace(limits.CPU) != "" || strings.TrimSpace(limits.Memory) != "" || limits.GPU > 0 {
		out.Limits = map[string]string{}
		if strings.TrimSpace(limits.CPU) != "" {
			out.Limits["cpu"] = strings.TrimSpace(limits.CPU)
		}
		if strings.TrimSpace(limits.Memory) != "" {
			out.Limits["memory"] = strings.TrimSpace(limits.Memory)
		}
		if limits.GPU > 0 {
			out.Limits["nvidia.com/gpu"] = strconv.Itoa(limits.GPU)
		}
	}
	return out
}

func buildEnvVars(runSpec domain.RunSpec, runID string, step domain.PipelineStep) []k8s.EnvVar {
	out := []k8s.EnvVar{}
	appendEnv := func(name, value string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		out = append(out, k8s.EnvVar{Name: name, Value: value})
	}
	appendEnv("ANIMUS_RUN_ID", runID)
	appendEnv("ANIMUS_PROJECT_ID", runSpec.ProjectID)
	appendEnv("ANIMUS_ENV_LOCK_ID", runSpec.EnvLock.LockID)
	appendEnv("ANIMUS_ENV_HASH", runSpec.EnvLock.EnvHash)
	appendEnv("ANIMUS_POLICY_SNAPSHOT_SHA", runSpec.PolicySnapshot.SnapshotSHA256)
	appendEnv("ANIMUS_NETWORK_CLASS_REF", runSpec.EnvLock.NetworkClassRef)
	appendEnv("ANIMUS_SECRET_ACCESS_CLASS_REF", runSpec.EnvLock.SecretAccessClassRef)
	appendEnv("ANIMUS_CODE_REPO", runSpec.CodeRef.RepoURL)
	appendEnv("ANIMUS_CODE_COMMIT", runSpec.CodeRef.CommitSHA)
	appendEnv("ANIMUS_CODE_PATH", runSpec.CodeRef.Path)
	appendEnv("ANIMUS_CODE_SCM", runSpec.CodeRef.SCMType)
	appendEnv("ANIMUS_STEP_NAME", step.Name)

	bindingsJSON, _ := json.Marshal(runSpec.DatasetBindings)
	appendEnv("ANIMUS_DATASET_BINDINGS", string(bindingsJSON))
	paramsJSON, _ := json.Marshal(runSpec.Parameters)
	appendEnv("ANIMUS_PARAMETERS", string(paramsJSON))

	reserved := map[string]struct{}{}
	for _, env := range out {
		reserved[env.Name] = struct{}{}
	}
	for _, env := range step.Env {
		name := strings.TrimSpace(env.Name)
		if name == "" {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(name), "ANIMUS_") {
			continue
		}
		if _, ok := reserved[name]; ok {
			continue
		}
		out = append(out, k8s.EnvVar{Name: name, Value: env.Value})
	}
	return out
}

func filterLabelLength(labels map[string]string) map[string]string {
	out := make(map[string]string, len(labels))
	for key, value := range labels {
		k := strings.TrimSpace(key)
		v := strings.TrimSpace(value)
		if k == "" || v == "" {
			continue
		}
		if len(k) > 63 || len(v) > 63 {
			continue
		}
		out[k] = v
	}
	return out
}
