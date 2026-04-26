package runtimeexec

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/animus-labs/animus-go/closed/internal/platform/k8s"
)

type KubernetesJobExecutor struct {
	client            *k8s.Client
	namespace         string
	jobTTLSeconds     int32
	jobServiceAccount string
}

func NewKubernetesJobExecutor(client *k8s.Client, namespace string, jobTTLSeconds int32, jobServiceAccount string) (*KubernetesJobExecutor, error) {
	if client == nil {
		return nil, errors.New("k8s client is required")
	}
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		namespace = strings.TrimSpace(client.Namespace())
	}
	if namespace == "" {
		return nil, errors.New("training namespace is required")
	}
	if jobTTLSeconds < 0 {
		return nil, errors.New("job ttl must be non-negative")
	}
	return &KubernetesJobExecutor{
		client:            client,
		namespace:         namespace,
		jobTTLSeconds:     jobTTLSeconds,
		jobServiceAccount: strings.TrimSpace(jobServiceAccount),
	}, nil
}

func (e *KubernetesJobExecutor) Kind() string {
	return "kubernetes_job"
}

func (e *KubernetesJobExecutor) Submit(ctx context.Context, spec JobSpec) error {
	jobName := strings.TrimSpace(spec.K8sJobName)
	if jobName == "" {
		return errors.New("k8s job name is required")
	}
	if strings.TrimSpace(spec.RunID) == "" {
		return errors.New("run id is required")
	}
	if strings.TrimSpace(spec.ImageRef) == "" {
		return errors.New("image ref is required")
	}

	jobKind := strings.TrimSpace(spec.JobKind)
	if jobKind == "" {
		jobKind = "training"
	}
	component := "training-job"
	if strings.EqualFold(jobKind, "evaluation") {
		component = "evaluation-job"
	}

	namespace := strings.TrimSpace(spec.K8sNamespace)
	if namespace == "" {
		namespace = e.namespace
	}

	labels := map[string]string{
		"app.kubernetes.io/name":      "animus-datapilot",
		"app.kubernetes.io/component": component,
		"animus.run_id":               spec.RunID,
	}

	backoff := int32(0)
	var ttl *int32
	if e.jobTTLSeconds > 0 {
		ttl = &e.jobTTLSeconds
	}

	container := k8s.Container{
		Name:  "trainer",
		Image: spec.ImageRef,
		Env: []k8s.EnvVar{
			{Name: "RUN_ID", Value: spec.RunID},
			{Name: "DATASET_VERSION_ID", Value: spec.DatasetVersionID},
			{Name: "DATAPILOT_URL", Value: spec.DatapilotURL},
			{Name: "TOKEN", Value: spec.Token},
			{Name: "ANIMUS_JOB_KIND", Value: jobKind},
		},
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
			container.Env = append(container.Env, k8s.EnvVar{Name: key, Value: spec.Env[key]})
		}
	}

	applyResourceHints(&container, spec.Resources)

	podSpec := k8s.PodSpec{
		RestartPolicy: "Never",
		Containers:    []k8s.Container{container},
	}
	if e.jobServiceAccount != "" {
		podSpec.ServiceAccountName = e.jobServiceAccount
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

	err := e.client.CreateJob(ctx, namespace, job)
	if err == nil || errors.Is(err, k8s.ErrAlreadyExists) {
		return nil
	}
	return err
}

func (e *KubernetesJobExecutor) Inspect(ctx context.Context, execution Execution) (Observation, error) {
	namespace := strings.TrimSpace(execution.K8sNamespace)
	if namespace == "" {
		namespace = e.namespace
	}
	jobName := strings.TrimSpace(execution.K8sJobName)
	if jobName == "" {
		return Observation{}, errors.New("k8s job name is required")
	}

	job, err := e.client.GetJob(ctx, namespace, jobName)
	if err != nil {
		if errors.Is(err, k8s.ErrNotFound) {
			return Observation{Status: "pending", Message: "job_not_found"}, nil
		}
		return Observation{}, err
	}

	status := "pending"
	message := ""
	if cond, ok := findJobCondition(job.Status.Conditions, "Failed"); ok && strings.EqualFold(cond.Status, "True") {
		status = "failed"
		message = strings.TrimSpace(cond.Message)
		if message == "" {
			message = strings.TrimSpace(cond.Reason)
		}
	} else if cond, ok := findJobCondition(job.Status.Conditions, "Complete"); ok && strings.EqualFold(cond.Status, "True") {
		status = "succeeded"
		message = strings.TrimSpace(cond.Message)
		if message == "" {
			message = strings.TrimSpace(cond.Reason)
		}
	} else if job.Status.Active > 0 {
		status = "running"
	}

	conditionSummaries := make([]map[string]any, 0, len(job.Status.Conditions))
	for _, cond := range job.Status.Conditions {
		conditionSummaries = append(conditionSummaries, map[string]any{
			"type":    cond.Type,
			"status":  cond.Status,
			"reason":  cond.Reason,
			"message": cond.Message,
		})
	}

	return Observation{
		Status:  status,
		Message: message,
		Details: map[string]any{
			"k8s_namespace": namespace,
			"k8s_job_name":  jobName,
			"active":        job.Status.Active,
			"succeeded":     job.Status.Succeeded,
			"failed":        job.Status.Failed,
			"conditions":    conditionSummaries,
		},
	}, nil
}

func findJobCondition(conditions []k8s.JobCondition, conditionType string) (k8s.JobCondition, bool) {
	for _, cond := range conditions {
		if strings.EqualFold(strings.TrimSpace(cond.Type), strings.TrimSpace(conditionType)) {
			return cond, true
		}
	}
	return k8s.JobCondition{}, false
}

func applyResourceHints(container *k8s.Container, resources map[string]any) {
	if container == nil || len(resources) == 0 {
		return
	}

	gpus := int64(0)
	if v, ok := resources["gpus"]; ok {
		switch t := v.(type) {
		case float64:
			gpus = int64(t)
		case int64:
			gpus = t
		case int:
			gpus = int64(t)
		case string:
			if parsed, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64); err == nil {
				gpus = parsed
			}
		}
	}
	if gpus > 0 {
		if container.Resources.Limits == nil {
			container.Resources.Limits = map[string]string{}
		}
		container.Resources.Limits["nvidia.com/gpu"] = fmt.Sprintf("%d", gpus)
	}

	if cpu, ok := resources["cpu"].(string); ok && strings.TrimSpace(cpu) != "" {
		if container.Resources.Requests == nil {
			container.Resources.Requests = map[string]string{}
		}
		container.Resources.Requests["cpu"] = strings.TrimSpace(cpu)
	}
	if memory, ok := resources["memory"].(string); ok && strings.TrimSpace(memory) != "" {
		if container.Resources.Requests == nil {
			container.Resources.Requests = map[string]string{}
		}
		container.Resources.Requests["memory"] = strings.TrimSpace(memory)
	}
}
