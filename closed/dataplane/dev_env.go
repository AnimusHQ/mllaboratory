package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/animus-labs/animus-go/closed/internal/dataplane"
	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/k8s"
)

const devEnvTTLMinimumSeconds int64 = 60

func (api *dataplaneAPI) handleCreateDevEnv(w http.ResponseWriter, r *http.Request) {
	devEnvID := strings.TrimSpace(r.PathValue("dev_env_id"))
	if devEnvID == "" {
		writeError(w, http.StatusBadRequest, "dev_env_id_required", r.Header.Get("X-Request-Id"))
		return
	}

	var req dataplane.DevEnvProvisionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", r.Header.Get("X-Request-Id"))
		return
	}
	if strings.TrimSpace(req.DevEnvID) == "" || strings.TrimSpace(req.ProjectID) == "" || strings.TrimSpace(req.TemplateRef) == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", r.Header.Get("X-Request-Id"))
		return
	}
	if req.DevEnvID != devEnvID {
		writeError(w, http.StatusBadRequest, "dev_env_id_mismatch", r.Header.Get("X-Request-Id"))
		return
	}
	if req.EmittedAt.IsZero() {
		writeError(w, http.StatusBadRequest, "emitted_at_required", r.Header.Get("X-Request-Id"))
		return
	}
	if strings.TrimSpace(req.ImageRef) == "" {
		writeError(w, http.StatusBadRequest, "image_ref_required", r.Header.Get("X-Request-Id"))
		return
	}
	if req.TTLSeconds < devEnvTTLMinimumSeconds {
		writeError(w, http.StatusBadRequest, "ttl_seconds_too_small", r.Header.Get("X-Request-Id"))
		return
	}
	repoURL := strings.TrimSpace(req.RepoURL)
	refType := strings.TrimSpace(req.RefType)
	refValue := strings.TrimSpace(req.RefValue)
	if repoURL != "" {
		if refType == "" || refValue == "" {
			writeError(w, http.StatusBadRequest, "repo_ref_required", r.Header.Get("X-Request-Id"))
			return
		}
		if !validDevEnvRefType(refType) {
			writeError(w, http.StatusBadRequest, "invalid_ref_type", r.Header.Get("X-Request-Id"))
			return
		}
	}

	if err := validateEgressPolicy(api.cfg.EgressMode, domain.EnvLock{NetworkClassRef: req.NetworkClassRef}); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "network_policy_required", r.Header.Get("X-Request-Id"))
		return
	}

	jobName := jobNameForDevEnv(devEnvID)
	namespace := devEnvNamespace(api.cfg, api.k8s)

	job, err := buildDevEnvJobSpec(
		req,
		jobName,
		namespace,
		api.cfg.DevEnvServiceAccount,
		api.cfg.DevEnvTTLAfterFinishedSeconds,
		api.cfg.DevEnvWorkspacePath,
		api.cfg.DevEnvGitImage,
		api.cfg.DevEnvCodeServerCommand,
		api.cfg.DevEnvCodeServerPort,
	)
	if err != nil {
		writeError(w, http.StatusConflict, "devenv_job_build_failed", r.Header.Get("X-Request-Id"))
		return
	}

	service, err := buildDevEnvServiceSpec(req, jobName, namespace, api.cfg.DevEnvCodeServerPort)
	if err != nil {
		writeError(w, http.StatusConflict, "devenv_service_build_failed", r.Header.Get("X-Request-Id"))
		return
	}
	serviceCreated := false
	if err := api.k8s.CreateService(r.Context(), namespace, service); err != nil {
		if !errors.Is(err, k8s.ErrAlreadyExists) {
			writeError(w, http.StatusBadGateway, "devenv_service_create_failed", r.Header.Get("X-Request-Id"))
			return
		}
	} else {
		serviceCreated = true
	}

	if err := api.k8s.CreateJob(r.Context(), namespace, job); err != nil && !errors.Is(err, k8s.ErrAlreadyExists) {
		if serviceCreated {
			_ = api.k8s.DeleteService(r.Context(), namespace, jobName)
		}
		writeError(w, http.StatusBadGateway, "devenv_job_create_failed", r.Header.Get("X-Request-Id"))
		return
	}

	writeJSON(w, http.StatusOK, dataplane.DevEnvProvisionResponse{
		DevEnvID:  devEnvID,
		ProjectID: req.ProjectID,
		Accepted:  true,
		JobName:   jobName,
		Namespace: namespace,
	})
}

func (api *dataplaneAPI) handleDeleteDevEnv(w http.ResponseWriter, r *http.Request) {
	devEnvID := strings.TrimSpace(r.PathValue("dev_env_id"))
	if devEnvID == "" {
		writeError(w, http.StatusBadRequest, "dev_env_id_required", r.Header.Get("X-Request-Id"))
		return
	}

	var req dataplane.DevEnvDeleteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", r.Header.Get("X-Request-Id"))
		return
	}
	if strings.TrimSpace(req.DevEnvID) == "" || strings.TrimSpace(req.ProjectID) == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", r.Header.Get("X-Request-Id"))
		return
	}
	if req.DevEnvID != devEnvID {
		writeError(w, http.StatusBadRequest, "dev_env_id_mismatch", r.Header.Get("X-Request-Id"))
		return
	}
	if req.EmittedAt.IsZero() {
		writeError(w, http.StatusBadRequest, "emitted_at_required", r.Header.Get("X-Request-Id"))
		return
	}

	jobName := jobNameForDevEnv(devEnvID)
	namespace := devEnvNamespace(api.cfg, api.k8s)
	err := api.k8s.DeleteJob(r.Context(), namespace, jobName)
	if err != nil && !errors.Is(err, k8s.ErrNotFound) {
		writeError(w, http.StatusBadGateway, "devenv_job_delete_failed", r.Header.Get("X-Request-Id"))
		return
	}
	serviceErr := api.k8s.DeleteService(r.Context(), namespace, jobName)
	if serviceErr != nil && !errors.Is(serviceErr, k8s.ErrNotFound) {
		writeError(w, http.StatusBadGateway, "devenv_service_delete_failed", r.Header.Get("X-Request-Id"))
		return
	}

	writeJSON(w, http.StatusOK, dataplane.DevEnvDeleteResponse{
		DevEnvID:  devEnvID,
		ProjectID: req.ProjectID,
		Deleted:   true,
	})
}

func (api *dataplaneAPI) handleAccessDevEnv(w http.ResponseWriter, r *http.Request) {
	devEnvID := strings.TrimSpace(r.PathValue("dev_env_id"))
	if devEnvID == "" {
		writeError(w, http.StatusBadRequest, "dev_env_id_required", r.Header.Get("X-Request-Id"))
		return
	}

	var req dataplane.DevEnvAccessRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", r.Header.Get("X-Request-Id"))
		return
	}
	if strings.TrimSpace(req.DevEnvID) == "" || strings.TrimSpace(req.ProjectID) == "" || strings.TrimSpace(req.SessionID) == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", r.Header.Get("X-Request-Id"))
		return
	}
	if req.DevEnvID != devEnvID {
		writeError(w, http.StatusBadRequest, "dev_env_id_mismatch", r.Header.Get("X-Request-Id"))
		return
	}
	if req.EmittedAt.IsZero() {
		writeError(w, http.StatusBadRequest, "emitted_at_required", r.Header.Get("X-Request-Id"))
		return
	}

	jobName := jobNameForDevEnv(devEnvID)
	namespace := devEnvNamespace(api.cfg, api.k8s)

	status, err := inspectJob(r.Context(), api.k8s, namespace, jobName)
	if err != nil {
		if errors.Is(err, errJobNotFound) {
			writeError(w, http.StatusNotFound, "not_found", r.Header.Get("X-Request-Id"))
			return
		}
		writeError(w, http.StatusBadGateway, "devenv_status_unavailable", r.Header.Get("X-Request-Id"))
		return
	}

	ready := status.State == jobStateRunning
	message := status.Reason
	if message == "" {
		message = status.State
	}

	writeJSON(w, http.StatusOK, dataplane.DevEnvAccessResponse{
		DevEnvID:  devEnvID,
		ProjectID: req.ProjectID,
		Ready:     ready,
		JobName:   jobName,
		Namespace: namespace,
		Message:   message,
	})
}

func buildDevEnvJobSpec(req dataplane.DevEnvProvisionRequest, jobName, namespace, serviceAccount string, ttlAfterFinishedSeconds int32, workspacePath, gitImage, codeServerCommand string, codeServerPort int32) (k8s.Job, error) {
	if strings.TrimSpace(req.ImageRef) == "" {
		return k8s.Job{}, errors.New("image ref is required")
	}
	if strings.TrimSpace(jobName) == "" {
		return k8s.Job{}, errors.New("job name is required")
	}
	if strings.TrimSpace(namespace) == "" {
		return k8s.Job{}, errors.New("namespace is required")
	}

	labels := buildDevEnvLabels(req)
	workspacePath = strings.TrimSpace(workspacePath)
	if workspacePath == "" {
		workspacePath = "/workspace"
	}
	if codeServerPort <= 0 {
		codeServerPort = 8080
	}

	container := k8s.Container{
		Name:      "devenv",
		Image:     strings.TrimSpace(req.ImageRef),
		Resources: buildResourceRequirements(req.ResourceDefaults, req.ResourceLimits),
		Env: []k8s.EnvVar{
			{Name: "ANIMUS_DEV_ENV_ID", Value: strings.TrimSpace(req.DevEnvID)},
			{Name: "ANIMUS_PROJECT_ID", Value: strings.TrimSpace(req.ProjectID)},
			{Name: "ANIMUS_TEMPLATE_REF", Value: strings.TrimSpace(req.TemplateRef)},
			{Name: "ANIMUS_NETWORK_CLASS_REF", Value: strings.TrimSpace(req.NetworkClassRef)},
			{Name: "ANIMUS_SECRET_ACCESS_CLASS_REF", Value: strings.TrimSpace(req.SecretAccessClassRef)},
			{Name: "ANIMUS_DEV_ENV_TTL_SECONDS", Value: strconv.FormatInt(req.TTLSeconds, 10)},
			{Name: "ANIMUS_DEV_ENV_WORKSPACE", Value: workspacePath},
		},
		Ports: []k8s.ContainerPort{
			{Name: "ide", ContainerPort: codeServerPort, Protocol: "TCP"},
		},
		VolumeMounts: []k8s.VolumeMount{
			{Name: "workspace", MountPath: workspacePath},
		},
	}
	if strings.TrimSpace(codeServerCommand) != "" {
		container.Command = []string{"/bin/sh", "-lc"}
		container.Args = []string{strings.TrimSpace(codeServerCommand)}
	}

	podSpec := k8s.PodSpec{
		RestartPolicy: "Never",
		Containers:    []k8s.Container{container},
		Volumes: []k8s.Volume{
			{Name: "workspace", EmptyDir: &k8s.EmptyDirVolumeSource{}},
		},
	}
	if initContainer, ok := buildDevEnvInitContainer(req, workspacePath, gitImage); ok {
		podSpec.InitContainers = []k8s.Container{initContainer}
	}
	if strings.TrimSpace(serviceAccount) != "" {
		podSpec.ServiceAccountName = strings.TrimSpace(serviceAccount)
	}

	backoff := int32(0)
	var ttl *int32
	if ttlAfterFinishedSeconds > 0 {
		ttl = &ttlAfterFinishedSeconds
	}
	var activeDeadline *int64
	if req.TTLSeconds > 0 {
		ttlSeconds := req.TTLSeconds
		activeDeadline = &ttlSeconds
	}

	job := k8s.Job{
		Metadata: k8s.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: k8s.JobSpec{
			BackoffLimit:            &backoff,
			TTLSecondsAfterFinished: ttl,
			ActiveDeadlineSeconds:   activeDeadline,
			Template: k8s.PodTemplateSpec{
				Metadata: k8s.ObjectMeta{Labels: labels},
				Spec:     podSpec,
			},
		},
	}
	return job, nil
}

func buildDevEnvServiceSpec(req dataplane.DevEnvProvisionRequest, serviceName, namespace string, port int32) (k8s.Service, error) {
	serviceName = strings.TrimSpace(serviceName)
	namespace = strings.TrimSpace(namespace)
	if serviceName == "" {
		return k8s.Service{}, errors.New("service name is required")
	}
	if namespace == "" {
		return k8s.Service{}, errors.New("namespace is required")
	}
	if port <= 0 {
		port = 8080
	}
	labels := buildDevEnvLabels(req)
	return k8s.Service{
		Metadata: k8s.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: k8s.ServiceSpec{
			Selector: labels,
			Ports: []k8s.ServicePort{
				{Name: "ide", Port: port, TargetPort: port, Protocol: "TCP"},
			},
		},
	}, nil
}

func buildDevEnvLabels(req dataplane.DevEnvProvisionRequest) map[string]string {
	labels := map[string]string{
		"app.kubernetes.io/name":      "animus-dataplane",
		"app.kubernetes.io/component": "devenv",
		"animus.dev_env_id":           strings.TrimSpace(req.DevEnvID),
		"animus.project_id":           strings.TrimSpace(req.ProjectID),
		"animus.template_ref":         strings.TrimSpace(req.TemplateRef),
	}
	if strings.TrimSpace(req.NetworkClassRef) != "" {
		labels["animus.network_class_ref"] = strings.TrimSpace(req.NetworkClassRef)
	}
	if strings.TrimSpace(req.SecretAccessClassRef) != "" {
		labels["animus.secret_access_class_ref"] = strings.TrimSpace(req.SecretAccessClassRef)
	}
	return filterLabelLength(labels)
}

func buildDevEnvInitContainer(req dataplane.DevEnvProvisionRequest, workspacePath, gitImage string) (k8s.Container, bool) {
	if strings.TrimSpace(req.RepoURL) == "" {
		return k8s.Container{}, false
	}
	gitImage = strings.TrimSpace(gitImage)
	if gitImage == "" {
		return k8s.Container{}, false
	}
	script := `set -euo pipefail
repo="${ANIMUS_DEVENV_REPO_URL}"
ref_type="${ANIMUS_DEVENV_REF_TYPE}"
ref_value="${ANIMUS_DEVENV_REF_VALUE}"
commit_pin="${ANIMUS_DEVENV_COMMIT_PIN:-}"
workspace="${ANIMUS_DEVENV_WORKSPACE}"

mkdir -p "${workspace}"
git init "${workspace}"
cd "${workspace}"
git remote add origin "${repo}"

case "${ref_type}" in
  branch)
    git fetch --depth=1 origin "refs/heads/${ref_value}"
    git checkout -B "${ref_value}" FETCH_HEAD
    ;;
  tag)
    git fetch --depth=1 origin "refs/tags/${ref_value}"
    git checkout FETCH_HEAD
    ;;
  commit)
    git fetch --depth=1 origin "${ref_value}"
    git checkout "${ref_value}"
    ;;
  *)
    echo "unsupported ref type" >&2
    exit 1
    ;;
esac

if [ -n "${commit_pin}" ]; then
  git fetch --depth=1 origin "${commit_pin}"
  git checkout "${commit_pin}"
fi
`
	return k8s.Container{
		Name:    "repo-init",
		Image:   gitImage,
		Command: []string{"/bin/sh", "-lc"},
		Args:    []string{script},
		Env: []k8s.EnvVar{
			{Name: "ANIMUS_DEVENV_REPO_URL", Value: strings.TrimSpace(req.RepoURL)},
			{Name: "ANIMUS_DEVENV_REF_TYPE", Value: strings.TrimSpace(req.RefType)},
			{Name: "ANIMUS_DEVENV_REF_VALUE", Value: strings.TrimSpace(req.RefValue)},
			{Name: "ANIMUS_DEVENV_COMMIT_PIN", Value: strings.TrimSpace(req.CommitPin)},
			{Name: "ANIMUS_DEVENV_WORKSPACE", Value: workspacePath},
		},
		VolumeMounts: []k8s.VolumeMount{
			{Name: "workspace", MountPath: workspacePath},
		},
	}, true
}

func validDevEnvRefType(refType string) bool {
	switch strings.ToLower(strings.TrimSpace(refType)) {
	case domain.DevEnvRefTypeBranch, domain.DevEnvRefTypeTag, domain.DevEnvRefTypeCommit:
		return true
	default:
		return false
	}
}

func devEnvNamespace(cfg dataplaneConfig, client *k8s.Client) string {
	if strings.TrimSpace(cfg.DevEnvNamespace) != "" {
		return strings.TrimSpace(cfg.DevEnvNamespace)
	}
	if strings.TrimSpace(cfg.Namespace) != "" {
		return strings.TrimSpace(cfg.Namespace)
	}
	if client == nil {
		return ""
	}
	return strings.TrimSpace(client.Namespace())
}

func jobNameForDevEnv(devEnvID string) string {
	base := "animus-devenv-" + sanitizeName(devEnvID)
	if len(base) <= 63 {
		return base
	}
	sum := sha256.Sum256([]byte(devEnvID))
	suffix := hex.EncodeToString(sum[:6])
	trim := 63 - len(suffix) - 1
	if trim < 1 {
		trim = 1
	}
	return base[:trim] + "-" + suffix
}
