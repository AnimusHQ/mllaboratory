package runtimeexec

import (
	"context"
	"errors"
)

// Executor defines the runtime execution surface used by the control plane.
type Executor interface {
	Kind() string
	Submit(ctx context.Context, spec JobSpec) error
	Inspect(ctx context.Context, execution Execution) (Observation, error)
}

// DockerImageIDResolver exposes image ID resolution for Docker-backed executors.
type DockerImageIDResolver interface {
	ResolveImageID(ctx context.Context, imageRef string) (string, error)
}

type JobSpec struct {
	RunID            string
	DatasetVersionID string
	ImageRef         string
	DatapilotURL     string
	Token            string
	Resources        map[string]any
	K8sNamespace     string
	K8sJobName       string
	DockerName       string
	JobKind          string
	Env              map[string]string
}

type Execution struct {
	RunID           string
	Executor        string
	K8sNamespace    string
	K8sJobName      string
	DockerContainer string
}

type Observation struct {
	Status  string
	Message string
	Details map[string]any
}

var ErrImageRefNotFound = errors.New("image_ref_not_found")
var ErrImageRefDigestRequired = errors.New("image_ref_digest_required")
