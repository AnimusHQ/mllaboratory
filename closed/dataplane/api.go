package main

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/dataplane"
	"github.com/animus-labs/animus-go/closed/internal/platform/k8s"
)

type dataplaneConfig struct {
	Namespace         string
	JobTTLSeconds     int32
	JobServiceAccount string
	HeartbeatInterval time.Duration
	PollInterval      time.Duration
}

type dataplaneAPI struct {
	logger *slog.Logger
	cp     *controlPlaneClient
	k8s    *k8s.Client
	cfg    dataplaneConfig

	mu       sync.Mutex
	trackers map[string]*runTracker
}

func newDataplaneAPI(logger *slog.Logger, cp *controlPlaneClient, k8sClient *k8s.Client, cfg dataplaneConfig) *dataplaneAPI {
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = 15 * time.Second
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 10 * time.Second
	}
	return &dataplaneAPI{
		logger:   logger,
		cp:       cp,
		k8s:      k8sClient,
		cfg:      cfg,
		trackers: make(map[string]*runTracker),
	}
}

func (api *dataplaneAPI) register(mux *http.ServeMux) {
	mux.HandleFunc("POST /internal/dp/runs/{run_id}:execute", api.handleExecuteRun)
	mux.HandleFunc("GET /internal/dp/runs/{run_id}/status", api.handleGetRunStatus)
}

func (api *dataplaneAPI) handleExecuteRun(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(r.PathValue("run_id"))
	if runID == "" {
		writeError(w, http.StatusBadRequest, "run_id_required", r.Header.Get("X-Request-Id"))
		return
	}

	var req dataplane.RunExecutionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", r.Header.Get("X-Request-Id"))
		return
	}
	if strings.TrimSpace(req.RunID) == "" || strings.TrimSpace(req.ProjectID) == "" || strings.TrimSpace(req.DispatchID) == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", r.Header.Get("X-Request-Id"))
		return
	}
	if !strings.EqualFold(runID, req.RunID) {
		writeError(w, http.StatusBadRequest, "run_id_mismatch", r.Header.Get("X-Request-Id"))
		return
	}
	if req.EmittedAt.IsZero() {
		writeError(w, http.StatusBadRequest, "emitted_at_required", r.Header.Get("X-Request-Id"))
		return
	}

	api.mu.Lock()
	if existing, ok := api.trackers[runID]; ok {
		api.mu.Unlock()
		if existing.DispatchID != req.DispatchID {
			writeError(w, http.StatusConflict, "dispatch_id_conflict", r.Header.Get("X-Request-Id"))
			return
		}
		writeJSON(w, http.StatusOK, dataplane.RunExecutionResponse{
			RunID:      runID,
			ProjectID:  existing.ProjectID,
			DispatchID: existing.DispatchID,
			Accepted:   true,
			JobName:    existing.JobName,
			Namespace:  existing.Namespace,
		})
		return
	}
	api.mu.Unlock()

	bundle, statusCode, err := api.cp.GetReproBundle(r.Context(), req.ProjectID, runID, r.Header.Get("X-Request-Id"))
	if err != nil {
		if statusCode == http.StatusNotFound {
			writeError(w, http.StatusNotFound, "not_found", r.Header.Get("X-Request-Id"))
			return
		}
		writeError(w, http.StatusBadGateway, "repro_bundle_unavailable", r.Header.Get("X-Request-Id"))
		return
	}
	if strings.TrimSpace(bundle.ProjectID) != strings.TrimSpace(req.ProjectID) || strings.TrimSpace(bundle.RunID) != strings.TrimSpace(runID) {
		writeError(w, http.StatusConflict, "bundle_mismatch", r.Header.Get("X-Request-Id"))
		return
	}

	runSpec, err := parseRunSpec(bundle.RunSpec)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_run_spec", r.Header.Get("X-Request-Id"))
		return
	}
	if strings.TrimSpace(runSpec.ProjectID) != strings.TrimSpace(req.ProjectID) {
		writeError(w, http.StatusConflict, "project_mismatch", r.Header.Get("X-Request-Id"))
		return
	}

	jobName := jobNameForRun(runID)
	namespace := strings.TrimSpace(api.cfg.Namespace)
	if namespace == "" {
		namespace = strings.TrimSpace(api.k8s.Namespace())
	}

	job, err := buildJobSpec(runSpec, runID, jobName, namespace, api.cfg.JobTTLSeconds, api.cfg.JobServiceAccount, req.DispatchID)
	if err != nil {
		writeError(w, http.StatusConflict, "job_build_failed", r.Header.Get("X-Request-Id"))
		return
	}

	if err := api.k8s.CreateJob(r.Context(), namespace, job); err != nil && !errors.Is(err, k8s.ErrAlreadyExists) {
		writeError(w, http.StatusBadGateway, "job_create_failed", r.Header.Get("X-Request-Id"))
		return
	}

	tracker := &runTracker{
		RunID:      runID,
		ProjectID:  req.ProjectID,
		DispatchID: req.DispatchID,
		JobName:    jobName,
		Namespace:  namespace,
		EnvLockID:  runSpec.EnvLock.LockID,
		PolicySHA:  runSpec.PolicySnapshot.SnapshotSHA256,
		StartedAt:  time.Now().UTC(),
	}
	api.addTracker(tracker)
	go api.monitorRun(tracker)

	writeJSON(w, http.StatusOK, dataplane.RunExecutionResponse{
		RunID:      runID,
		ProjectID:  req.ProjectID,
		DispatchID: req.DispatchID,
		Accepted:   true,
		JobName:    jobName,
		Namespace:  namespace,
	})
}

func (api *dataplaneAPI) handleGetRunStatus(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(r.PathValue("run_id"))
	projectID := strings.TrimSpace(r.URL.Query().Get("project_id"))
	if runID == "" {
		writeError(w, http.StatusBadRequest, "run_id_required", r.Header.Get("X-Request-Id"))
		return
	}
	if projectID == "" {
		writeError(w, http.StatusBadRequest, "project_id_required", r.Header.Get("X-Request-Id"))
		return
	}

	jobName := jobNameForRun(runID)
	namespace := strings.TrimSpace(api.cfg.Namespace)
	if namespace == "" {
		namespace = strings.TrimSpace(api.k8s.Namespace())
	}
	status, err := inspectJob(r.Context(), api.k8s, namespace, jobName)
	if err != nil {
		if errors.Is(err, errJobNotFound) {
			writeError(w, http.StatusNotFound, "not_found", r.Header.Get("X-Request-Id"))
			return
		}
		writeError(w, http.StatusBadGateway, "job_status_failed", r.Header.Get("X-Request-Id"))
		return
	}

	writeJSON(w, http.StatusOK, dataplane.RunExecutionStatus{
		RunID:      runID,
		ProjectID:  projectID,
		State:      status.State,
		JobName:    jobName,
		Namespace:  namespace,
		StartedAt:  status.StartedAt,
		FinishedAt: status.FinishedAt,
		Reason:     status.Reason,
	})
}

func (api *dataplaneAPI) addTracker(tracker *runTracker) {
	api.mu.Lock()
	defer api.mu.Unlock()
	api.trackers[tracker.RunID] = tracker
}

func (api *dataplaneAPI) removeTracker(runID string) {
	api.mu.Lock()
	defer api.mu.Unlock()
	delete(api.trackers, runID)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(body)
}

func writeError(w http.ResponseWriter, status int, code string, requestID string) {
	writeJSON(w, status, map[string]any{
		"error":      code,
		"request_id": requestID,
	})
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errors.New("multiple JSON values")
	}
	return nil
}
