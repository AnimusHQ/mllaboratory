package k8s

import "time"

type ObjectMeta struct {
	Name      string            `json:"name,omitempty"`
	Namespace string            `json:"namespace,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ResourceRequirements struct {
	Limits   map[string]string `json:"limits,omitempty"`
	Requests map[string]string `json:"requests,omitempty"`
}

type Container struct {
	Name      string               `json:"name"`
	Image     string               `json:"image"`
	Command   []string             `json:"command,omitempty"`
	Args      []string             `json:"args,omitempty"`
	Env       []EnvVar             `json:"env,omitempty"`
	Resources ResourceRequirements `json:"resources,omitempty"`
}

type PodSpec struct {
	RestartPolicy      string      `json:"restartPolicy,omitempty"`
	ServiceAccountName string      `json:"serviceAccountName,omitempty"`
	Containers         []Container `json:"containers"`
}

type PodTemplateSpec struct {
	Metadata ObjectMeta `json:"metadata,omitempty"`
	Spec     PodSpec    `json:"spec"`
}

type JobSpec struct {
	BackoffLimit            *int32          `json:"backoffLimit,omitempty"`
	TTLSecondsAfterFinished *int32          `json:"ttlSecondsAfterFinished,omitempty"`
	Template                PodTemplateSpec `json:"template"`
}

type JobCondition struct {
	Type               string     `json:"type,omitempty"`
	Status             string     `json:"status,omitempty"`
	Reason             string     `json:"reason,omitempty"`
	Message            string     `json:"message,omitempty"`
	LastTransitionTime *time.Time `json:"lastTransitionTime,omitempty"`
}

type JobStatus struct {
	StartTime      *time.Time     `json:"startTime,omitempty"`
	CompletionTime *time.Time     `json:"completionTime,omitempty"`
	Active         int32          `json:"active,omitempty"`
	Succeeded      int32          `json:"succeeded,omitempty"`
	Failed         int32          `json:"failed,omitempty"`
	Conditions     []JobCondition `json:"conditions,omitempty"`
}

type Job struct {
	APIVersion string     `json:"apiVersion,omitempty"`
	Kind       string     `json:"kind,omitempty"`
	Metadata   ObjectMeta `json:"metadata"`
	Spec       JobSpec    `json:"spec"`
	Status     JobStatus  `json:"status,omitempty"`
}
