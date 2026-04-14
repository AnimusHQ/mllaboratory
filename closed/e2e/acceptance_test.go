//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	testPipelineDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testEnvDigest      = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

type e2eConfig struct {
	GatewayURL   string
	DatabaseURL  string
	Timeout      time.Duration
	FailureTests bool
	ArtifactsDir string
	PipelineImg  string
	DevEnvImg    string
	DevEnvRepo   string
	DevEnvRef    string
	DevEnvRefVal string
	WebhookURL   string
}

type e2eState struct {
	ProjectID        string
	DatasetID        string
	DatasetVersionID string
	EnvDefID         string
	EnvLockID        string
	RunID            string
	ArtifactID       string
	ModelID          string
	ModelVersionID   string
	WebhookSubID     string
	DevEnvDefID      string
	DevEnvID         string
	DevEnvSessionID  string
	DenyProjectID    string
	AuditDeliveryID  int64
}

type httpResponse struct {
	StatusCode int
	Body       []byte
}

func TestE2EFullStack(t *testing.T) {
	cfg := loadConfig(t)
	client := &http.Client{Timeout: cfg.Timeout}
	state := &e2eState{}

	t.Run("datasets", func(t *testing.T) {
		state.ProjectID = createProject(t, client, cfg.datasetURL())
		state.DatasetID = createDataset(t, client, cfg.datasetURL(), state.ProjectID)
		state.DatasetVersionID = uploadDatasetVersion(t, client, cfg.datasetURL(), state.ProjectID, state.DatasetID)
		downloadDatasetVersion(t, client, cfg.datasetURL(), state.ProjectID, state.DatasetVersionID)
	})

	t.Run("webhook-subscriptions", func(t *testing.T) {
		state.WebhookSubID = createWebhookSubscription(t, client, cfg.experimentsURL(), state.ProjectID, cfg.WebhookURL)
	})

	t.Run("environment-locks", func(t *testing.T) {
		state.EnvDefID = createEnvironmentDefinition(t, client, cfg.experimentsURL(), state.ProjectID, "runtime", "python:3.11")
		state.EnvLockID = createEnvironmentLock(t, client, cfg.experimentsURL(), state.ProjectID, state.EnvDefID, http.StatusOK)
	})

	t.Run("registry-deny-policy", func(t *testing.T) {
		if cfg.DatabaseURL == "" {
			t.Skip("ANIMUS_E2E_DATABASE_URL not set")
		}
		db := openDB(t, cfg.DatabaseURL)
		defer db.Close()

		state.DenyProjectID = createProject(t, client, cfg.datasetURL())
		upsertRegistryPolicy(t, db, state.DenyProjectID, "deny_unsigned", "noop")

		denyEnvDefID := createEnvironmentDefinition(t, client, cfg.experimentsURL(), state.DenyProjectID, "runtime", "python:3.11")
		createEnvironmentLock(t, client, cfg.experimentsURL(), state.DenyProjectID, denyEnvDefID, http.StatusUnprocessableEntity)
	})

	t.Run("devenv", func(t *testing.T) {
		state.DevEnvDefID = createEnvironmentDefinition(t, client, cfg.experimentsURL(), state.ProjectID, "ide", cfg.DevEnvImg)
		state.DevEnvID = createDevEnv(t, client, cfg.experimentsURL(), state.ProjectID, state.DevEnvDefID, cfg)
		state.DevEnvSessionID = openDevEnvSession(t, client, cfg.experimentsURL(), state.ProjectID, state.DevEnvID, 60)
		proxyURL := cfg.experimentsURL() + "/devenv-sessions/" + state.DevEnvSessionID + "/proxy/"
		waitForProxy(t, client, proxyURL, state.ProjectID, 60*time.Second)

		expiringSessionID := openDevEnvSession(t, client, cfg.experimentsURL(), state.ProjectID, state.DevEnvID, 2)
		time.Sleep(3 * time.Second)
		resp := doRequest(t, client, http.MethodGet, cfg.experimentsURL()+"/devenv-sessions/"+expiringSessionID+"/proxy/", nil, state.ProjectID)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected session expired 403, got %d: %s", resp.StatusCode, resp.Body)
		}
	})

	t.Run("runs", func(t *testing.T) {
		invalidPayload := runPayload(state.EnvLockID, state.DatasetVersionID, cfg.PipelineImg, "not a url", "deadbeef")
		invalidResp := doJSONWithIdempotency(t, client, http.MethodPost, fmt.Sprintf("%s/projects/%s/runs", cfg.experimentsURL(), state.ProjectID), invalidPayload, state.ProjectID, "invalid-run-"+uuid.NewString())
		if invalidResp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected invalid codeRef to fail, got %d body=%s", invalidResp.StatusCode, invalidResp.Body)
		}

		idempotencyKey := "run-idem-" + uuid.NewString()
		state.RunID = createRun(t, client, cfg.experimentsURL(), state.ProjectID, state.EnvLockID, state.DatasetVersionID, cfg.PipelineImg, idempotencyKey)
		runID2 := createRun(t, client, cfg.experimentsURL(), state.ProjectID, state.EnvLockID, state.DatasetVersionID, cfg.PipelineImg, idempotencyKey)
		if runID2 != state.RunID {
			t.Fatalf("idempotent run mismatch: %s vs %s", state.RunID, runID2)
		}

		planRun(t, client, cfg.experimentsURL(), state.ProjectID, state.RunID)
		dispatchID := dispatchRun(t, client, cfg.experimentsURL(), state.ProjectID, state.RunID)
		dispatchID2 := dispatchRun(t, client, cfg.experimentsURL(), state.ProjectID, state.RunID)
		if dispatchID2 != dispatchID {
			t.Fatalf("dispatch idempotency mismatch: %s vs %s", dispatchID, dispatchID2)
		}

		postHeartbeat(t, client, cfg.experimentsURL(), state.ProjectID, state.RunID, "hb-"+state.RunID)
		postArtifactCommitted(t, client, cfg.experimentsURL(), state.ProjectID, state.RunID, "artifact-"+state.RunID)
		postTerminal(t, client, cfg.experimentsURL(), state.ProjectID, state.RunID, "term-"+state.RunID)
		postTerminal(t, client, cfg.experimentsURL(), state.ProjectID, state.RunID, "term-"+state.RunID)

		state.ArtifactID = createRunArtifact(t, client, cfg.experimentsURL(), state.RunID)
		downloadRunArtifact(t, client, cfg.experimentsURL(), state.RunID, state.ArtifactID)
		getRun(t, client, cfg.experimentsURL(), state.ProjectID, state.RunID)
		getReproBundle(t, client, cfg.experimentsURL(), state.ProjectID, state.RunID)
	})

	t.Run("model-registry", func(t *testing.T) {
		state.ModelID = createModel(t, client, cfg.experimentsURL(), state.ProjectID)
		state.ModelVersionID = createModelVersion(t, client, cfg.experimentsURL(), state.ProjectID, state.ModelID, state.RunID, state.ArtifactID)
		transitionModelVersion(t, client, cfg.experimentsURL(), state.ProjectID, state.ModelVersionID, "validate")
		transitionModelVersion(t, client, cfg.experimentsURL(), state.ProjectID, state.ModelVersionID, "approve")

		exportStatus := exportModelVersion(t, client, cfg.experimentsURL(), state.ProjectID, state.ModelVersionID, "export-idem-"+uuid.NewString())
		if exportStatus != http.StatusForbidden {
			t.Fatalf("expected export denied, got %d", exportStatus)
		}

		if cfg.DatabaseURL == "" {
			t.Skip("ANIMUS_E2E_DATABASE_URL not set for export allow")
		}
		db := openDB(t, cfg.DatabaseURL)
		defer db.Close()
		insertPolicyDecisionAllow(t, db, state.RunID)
		exportStatus = exportModelVersion(t, client, cfg.experimentsURL(), state.ProjectID, state.ModelVersionID, "export-idem-"+uuid.NewString())
		if exportStatus != http.StatusOK {
			t.Fatalf("expected export allowed, got %d", exportStatus)
		}
	})

	t.Run("webhook-deliveries", func(t *testing.T) {
		waitForWebhookDelivery(t, client, cfg.experimentsURL(), state.ProjectID, "RunFinished", 1, 30*time.Second)
		waitForWebhookDelivery(t, client, cfg.experimentsURL(), state.ProjectID, "ModelApproved", 1, 30*time.Second)
	})

	t.Run("lineage", func(t *testing.T) {
		getLineageRun(t, client, cfg.lineageURL(), state.RunID)
		getLineageModelVersion(t, client, cfg.lineageURL(), state.ModelVersionID)
	})

	if cfg.FailureTests {
		t.Run("resilience", func(t *testing.T) {
			deliveryID := firstWebhookDeliveryID(t, client, cfg.experimentsURL(), state.ProjectID, "RunFinished")
			replayWebhookDelivery(t, client, cfg.experimentsURL(), state.ProjectID, deliveryID, "replay-token")
			replayWebhookDelivery(t, client, cfg.experimentsURL(), state.ProjectID, deliveryID, "replay-token")
			conflict := postTerminalWithState(t, client, cfg.experimentsURL(), state.ProjectID, state.RunID, "term-fail-"+state.RunID, "failed")
			if conflict != http.StatusConflict {
				t.Fatalf("expected invalid transition conflict, got %d", conflict)
			}
			waitForAuditDLQ(t, client, cfg.auditURL(), 1, 45*time.Second)
			replayAuditDLQWithToken(t, client, cfg.auditURL(), "audit-replay-token")
			replayAuditDLQWithToken(t, client, cfg.auditURL(), "audit-replay-token")
		})
	}

	writeArtifacts(t, cfg, state)
}

func loadConfig(t *testing.T) e2eConfig {
	t.Helper()
	gateway := strings.TrimRight(strings.TrimSpace(os.Getenv("ANIMUS_E2E_GATEWAY_URL")), "/")
	if gateway == "" {
		t.Skip("ANIMUS_E2E_GATEWAY_URL not set")
	}
	timeout := 20 * time.Second
	if raw := strings.TrimSpace(os.Getenv("ANIMUS_E2E_TIMEOUT")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			t.Fatalf("invalid ANIMUS_E2E_TIMEOUT: %v", err)
		}
		timeout = parsed
	}
	pipelineImg := strings.TrimSpace(os.Getenv("ANIMUS_E2E_PIPELINE_IMAGE"))
	if pipelineImg == "" {
		pipelineImg = "ghcr.io/acme/train@" + testPipelineDigest
	}
	if !strings.Contains(pipelineImg, "@sha256:") {
		t.Fatalf("ANIMUS_E2E_PIPELINE_IMAGE must be digest-pinned")
	}
	devEnvImg := strings.TrimSpace(os.Getenv("ANIMUS_E2E_DEVENV_IMAGE"))
	if devEnvImg == "" {
		devEnvImg = "nginx:alpine"
	}
	devEnvRepo := strings.TrimSpace(os.Getenv("ANIMUS_E2E_DEVENV_REPO_URL"))
	if devEnvRepo == "" {
		devEnvRepo = "https://github.com/animus-labs/animus-go"
	}
	devEnvRef := strings.TrimSpace(os.Getenv("ANIMUS_E2E_DEVENV_REF_TYPE"))
	if devEnvRef == "" {
		devEnvRef = "branch"
	}
	devEnvRefVal := strings.TrimSpace(os.Getenv("ANIMUS_E2E_DEVENV_REF_VALUE"))
	if devEnvRefVal == "" {
		devEnvRefVal = "main"
	}
	webhookURL := strings.TrimSpace(os.Getenv("ANIMUS_E2E_WEBHOOK_TARGET_URL"))
	if webhookURL == "" {
		webhookURL = "http://127.0.0.1:1/webhook"
	}

	return e2eConfig{
		GatewayURL:   gateway,
		DatabaseURL:  strings.TrimSpace(os.Getenv("ANIMUS_E2E_DATABASE_URL")),
		Timeout:      timeout,
		FailureTests: parseBool(os.Getenv("ANIMUS_E2E_FAILURES")),
		ArtifactsDir: strings.TrimSpace(os.Getenv("ANIMUS_E2E_ARTIFACTS_DIR")),
		PipelineImg:  pipelineImg,
		DevEnvImg:    devEnvImg,
		DevEnvRepo:   devEnvRepo,
		DevEnvRef:    devEnvRef,
		DevEnvRefVal: devEnvRefVal,
		WebhookURL:   webhookURL,
	}
}

func (c e2eConfig) datasetURL() string {
	return c.GatewayURL + "/api/dataset-registry"
}

func (c e2eConfig) experimentsURL() string {
	return c.GatewayURL + "/api/experiments"
}

func (c e2eConfig) auditURL() string {
	return c.GatewayURL + "/api/audit"
}

func (c e2eConfig) lineageURL() string {
	return c.GatewayURL + "/api/lineage"
}

func createProject(t *testing.T, client *http.Client, baseURL string) string {
	payload := map[string]any{"name": "e2e-project-" + uuid.NewString()}
	resp := doJSON(t, client, http.MethodPost, baseURL+"/projects", payload, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create project status=%d body=%s", resp.StatusCode, resp.Body)
	}
	var out struct {
		ProjectID string `json:"project_id"`
	}
	decodeJSONBody(t, resp.Body, &out)
	if out.ProjectID == "" {
		t.Fatalf("project id missing")
	}
	return out.ProjectID
}

func createDataset(t *testing.T, client *http.Client, baseURL, projectID string) string {
	payload := map[string]any{"name": "e2e-dataset"}
	resp := doJSON(t, client, http.MethodPost, baseURL+"/datasets", payload, projectID)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create dataset status=%d body=%s", resp.StatusCode, resp.Body)
	}
	var out struct {
		DatasetID string `json:"dataset_id"`
	}
	decodeJSONBody(t, resp.Body, &out)
	if out.DatasetID == "" {
		t.Fatalf("dataset id missing")
	}
	return out.DatasetID
}

func uploadDatasetVersion(t *testing.T, client *http.Client, baseURL, projectID, datasetID string) string {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	file, err := writer.CreateFormFile("file", "dataset.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := file.Write([]byte("e2e dataset")); err != nil {
		t.Fatalf("write dataset: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	url := fmt.Sprintf("%s/datasets/%s/versions/upload", baseURL, datasetID)
	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	setRequestHeaders(req, projectID)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("upload dataset version: %v", err)
	}
	defer resp.Body.Close()
	body := readBody(t, resp.Body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload dataset version status=%d body=%s", resp.StatusCode, body)
	}
	var out struct {
		VersionID string `json:"version_id"`
	}
	decodeJSONBody(t, body, &out)
	if out.VersionID == "" {
		t.Fatalf("dataset version id missing")
	}
	return out.VersionID
}

func downloadDatasetVersion(t *testing.T, client *http.Client, baseURL, projectID, versionID string) {
	url := fmt.Sprintf("%s/dataset-versions/%s/download", baseURL, versionID)
	resp := doRequest(t, client, http.MethodGet, url, nil, projectID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download dataset version status=%d body=%s", resp.StatusCode, resp.Body)
	}
}

func createEnvironmentDefinition(t *testing.T, client *http.Client, baseURL, projectID, imageName, imageRef string) string {
	payload := map[string]any{
		"name":       "e2e-env-" + imageName,
		"baseImages": []map[string]any{{"name": imageName, "ref": imageRef}},
	}
	url := fmt.Sprintf("%s/projects/%s/environment-definitions", baseURL, projectID)
	resp := doJSONWithIdempotency(t, client, http.MethodPost, url, payload, projectID, "envdef-"+uuid.NewString())
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create env def status=%d body=%s", resp.StatusCode, resp.Body)
	}
	var out struct {
		Definition struct {
			ID string `json:"environmentDefinitionId"`
		} `json:"definition"`
	}
	decodeJSONBody(t, resp.Body, &out)
	if out.Definition.ID == "" {
		t.Fatalf("env definition id missing")
	}
	return out.Definition.ID
}

func createEnvironmentLock(t *testing.T, client *http.Client, baseURL, projectID, defID string, expectedStatus int) string {
	payload := map[string]any{
		"environmentDefinitionId": defID,
		"imageDigests":            map[string]string{"runtime": testEnvDigest, "ide": testEnvDigest},
	}
	url := fmt.Sprintf("%s/projects/%s/environment-locks", baseURL, projectID)
	resp := doJSONWithIdempotency(t, client, http.MethodPost, url, payload, projectID, "envlock-"+uuid.NewString())
	if resp.StatusCode != expectedStatus {
		t.Fatalf("create env lock status=%d body=%s", resp.StatusCode, resp.Body)
	}
	if expectedStatus != http.StatusOK {
		return ""
	}
	var out struct {
		Lock struct {
			LockID string `json:"lockId"`
		} `json:"lock"`
	}
	decodeJSONBody(t, resp.Body, &out)
	if out.Lock.LockID == "" {
		t.Fatalf("env lock id missing")
	}
	return out.Lock.LockID
}

func createDevEnv(t *testing.T, client *http.Client, baseURL, projectID, templateID string, cfg e2eConfig) string {
	payload := map[string]any{
		"templateRef": templateID,
		"repoUrl":     cfg.DevEnvRepo,
		"refType":     cfg.DevEnvRef,
		"refValue":    cfg.DevEnvRefVal,
		"ttlSeconds":  600,
	}
	url := fmt.Sprintf("%s/projects/%s/devenvs", baseURL, projectID)
	resp := doJSONWithIdempotency(t, client, http.MethodPost, url, payload, projectID, "devenv-"+uuid.NewString())
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create devenv status=%d body=%s", resp.StatusCode, resp.Body)
	}
	var out struct {
		Environment struct {
			ID string `json:"devEnvId"`
		} `json:"environment"`
	}
	decodeJSONBody(t, resp.Body, &out)
	if out.Environment.ID == "" {
		t.Fatalf("devenv id missing")
	}
	return out.Environment.ID
}

func openDevEnvSession(t *testing.T, client *http.Client, baseURL, projectID, devEnvID string, ttlSeconds int) string {
	payload := map[string]any{"ttlSeconds": ttlSeconds}
	url := fmt.Sprintf("%s/projects/%s/devenvs/%s:open-ide-session", baseURL, projectID, devEnvID)
	resp := doJSON(t, client, http.MethodPost, url, payload, projectID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("open session status=%d body=%s", resp.StatusCode, resp.Body)
	}
	var out struct {
		SessionID string `json:"sessionId"`
	}
	decodeJSONBody(t, resp.Body, &out)
	if out.SessionID == "" {
		t.Fatalf("session id missing")
	}
	return out.SessionID
}

func waitForProxy(t *testing.T, client *http.Client, url, projectID string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	var last httpResponse
	for time.Now().Before(deadline) {
		last = doRequest(t, client, http.MethodGet, url, nil, projectID)
		if last.StatusCode == http.StatusOK {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("proxy did not become ready: status=%d body=%s", last.StatusCode, last.Body)
}

func createRun(t *testing.T, client *http.Client, baseURL, projectID, lockID, datasetVersionID, pipelineImage, idempotencyKey string) string {
	payload := runPayload(lockID, datasetVersionID, pipelineImage, "https://github.com/animus-labs/animus-go", "deadbeef")
	url := fmt.Sprintf("%s/projects/%s/runs", baseURL, projectID)
	resp := doJSONWithIdempotency(t, client, http.MethodPost, url, payload, projectID, idempotencyKey)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("create run status=%d body=%s", resp.StatusCode, resp.Body)
	}
	var out struct {
		RunID string `json:"runId"`
	}
	decodeJSONBody(t, resp.Body, &out)
	if out.RunID == "" {
		t.Fatalf("run id missing")
	}
	return out.RunID
}

func runPayload(lockID, datasetVersionID, pipelineImage, repoURL, commitSHA string) map[string]any {
	pipelineSpec := map[string]any{
		"apiVersion":  "animus/v1alpha1",
		"kind":        "Pipeline",
		"specVersion": "1.0",
		"metadata": map[string]any{
			"name": "e2e-pipeline",
		},
		"spec": map[string]any{
			"steps": []map[string]any{
				{
					"name":    "train",
					"image":   pipelineImage,
					"command": []string{"/bin/sh", "-c"},
					"args":    []string{"echo ok"},
					"inputs": map[string]any{
						"datasets": []map[string]any{
							{"name": "dataset", "datasetRef": "dataset"},
						},
						"artifacts": []any{},
					},
					"outputs": map[string]any{
						"artifacts": []any{},
					},
					"env":       []any{},
					"resources": map[string]any{"cpu": "1", "memory": "1Gi", "gpu": 0},
					"retryPolicy": map[string]any{
						"maxAttempts": 1,
						"backoff": map[string]any{
							"type":           "fixed",
							"initialSeconds": 0,
							"maxSeconds":     0,
							"multiplier":     1,
						},
					},
				},
			},
			"dependencies": []any{},
		},
	}

	return map[string]any{
		"pipelineSpec":    pipelineSpec,
		"datasetBindings": map[string]string{"dataset": datasetVersionID},
		"codeRef": map[string]any{
			"repoUrl":   repoURL,
			"commitSha": commitSHA,
			"scmType":   "git",
		},
		"envLock":    map[string]any{"lockId": lockID},
		"parameters": map[string]any{},
	}
}

func planRun(t *testing.T, client *http.Client, baseURL, projectID, runID string) {
	url := fmt.Sprintf("%s/projects/%s/runs/%s:plan", baseURL, projectID, runID)
	resp := doJSON(t, client, http.MethodPost, url, map[string]any{}, projectID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("plan run status=%d body=%s", resp.StatusCode, resp.Body)
	}
}

func dispatchRun(t *testing.T, client *http.Client, baseURL, projectID, runID string) string {
	url := fmt.Sprintf("%s/projects/%s/runs/%s:dispatch", baseURL, projectID, runID)
	resp := doJSON(t, client, http.MethodPost, url, map[string]any{}, projectID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dispatch run status=%d body=%s", resp.StatusCode, resp.Body)
	}
	var out struct {
		DispatchID string `json:"dispatchId"`
	}
	decodeJSONBody(t, resp.Body, &out)
	if out.DispatchID == "" {
		t.Fatalf("dispatch id missing")
	}
	return out.DispatchID
}

func postHeartbeat(t *testing.T, client *http.Client, baseURL, projectID, runID, eventID string) {
	payload := map[string]any{
		"eventId":   eventID,
		"runId":     runID,
		"projectId": projectID,
		"emittedAt": time.Now().UTC().Format(time.RFC3339Nano),
	}
	url := fmt.Sprintf("%s/internal/cp/runs/%s/heartbeat", baseURL, runID)
	resp := doJSON(t, client, http.MethodPost, url, payload, projectID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("heartbeat status=%d body=%s", resp.StatusCode, resp.Body)
	}
}

func postTerminal(t *testing.T, client *http.Client, baseURL, projectID, runID, eventID string) {
	status := postTerminalWithState(t, client, baseURL, projectID, runID, eventID, "succeeded")
	if status != http.StatusOK {
		t.Fatalf("terminal status=%d", status)
	}
}

func postTerminalWithState(t *testing.T, client *http.Client, baseURL, projectID, runID, eventID, state string) int {
	now := time.Now().UTC()
	payload := map[string]any{
		"eventId":    eventID,
		"runId":      runID,
		"projectId":  projectID,
		"state":      state,
		"emittedAt":  now.Format(time.RFC3339Nano),
		"finishedAt": now.Format(time.RFC3339Nano),
	}
	url := fmt.Sprintf("%s/internal/cp/runs/%s/terminal", baseURL, runID)
	resp := doJSON(t, client, http.MethodPost, url, payload, projectID)
	return resp.StatusCode
}

func postArtifactCommitted(t *testing.T, client *http.Client, baseURL, projectID, runID, eventID string) {
	payload := map[string]any{
		"eventId":   eventID,
		"runId":     runID,
		"projectId": projectID,
		"emittedAt": time.Now().UTC().Format(time.RFC3339Nano),
		"payload":   map[string]any{"note": "artifact committed"},
	}
	url := fmt.Sprintf("%s/internal/cp/runs/%s/artifact-committed", baseURL, runID)
	resp := doJSON(t, client, http.MethodPost, url, payload, projectID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("artifact committed status=%d body=%s", resp.StatusCode, resp.Body)
	}
}

func createRunArtifact(t *testing.T, client *http.Client, baseURL, runID string) string {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("kind", "model")
	file, err := writer.CreateFormFile("file", "model.bin")
	if err != nil {
		t.Fatalf("create artifact form: %v", err)
	}
	if _, err := file.Write([]byte("model artifact")); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close artifact writer: %v", err)
	}

	url := fmt.Sprintf("%s/experiment-runs/%s/artifacts", baseURL, runID)
	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		t.Fatalf("new artifact request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	setRequestHeaders(req, "")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	defer resp.Body.Close()
	body := readBody(t, resp.Body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create artifact status=%d body=%s", resp.StatusCode, body)
	}
	var out struct {
		Artifact struct {
			ArtifactID string `json:"artifact_id"`
		} `json:"artifact"`
	}
	decodeJSONBody(t, body, &out)
	if out.Artifact.ArtifactID == "" {
		t.Fatalf("artifact id missing")
	}
	return out.Artifact.ArtifactID
}

func downloadRunArtifact(t *testing.T, client *http.Client, baseURL, runID, artifactID string) {
	url := fmt.Sprintf("%s/experiment-runs/%s/artifacts/%s/download", baseURL, runID, artifactID)
	resp := doRequest(t, client, http.MethodGet, url, nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download artifact status=%d body=%s", resp.StatusCode, resp.Body)
	}
}

func getRun(t *testing.T, client *http.Client, baseURL, projectID, runID string) {
	url := fmt.Sprintf("%s/projects/%s/runs/%s", baseURL, projectID, runID)
	resp := doRequest(t, client, http.MethodGet, url, nil, projectID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get run status=%d body=%s", resp.StatusCode, resp.Body)
	}
}

func getReproBundle(t *testing.T, client *http.Client, baseURL, projectID, runID string) {
	url := fmt.Sprintf("%s/projects/%s/runs/%s/reproducibility-bundle", baseURL, projectID, runID)
	resp := doRequest(t, client, http.MethodGet, url, nil, projectID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get repro bundle status=%d body=%s", resp.StatusCode, resp.Body)
	}
}

func createModel(t *testing.T, client *http.Client, baseURL, projectID string) string {
	payload := map[string]any{"name": "e2e-model"}
	url := fmt.Sprintf("%s/projects/%s/models", baseURL, projectID)
	resp := doJSONWithIdempotency(t, client, http.MethodPost, url, payload, projectID, "model-"+uuid.NewString())
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create model status=%d body=%s", resp.StatusCode, resp.Body)
	}
	var out struct {
		Model struct {
			ModelID string `json:"modelId"`
		} `json:"model"`
	}
	decodeJSONBody(t, resp.Body, &out)
	if out.Model.ModelID == "" {
		t.Fatalf("model id missing")
	}
	return out.Model.ModelID
}

func createModelVersion(t *testing.T, client *http.Client, baseURL, projectID, modelID, runID, artifactID string) string {
	payload := map[string]any{
		"version":     "v1",
		"runId":       runID,
		"artifactIds": []string{artifactID},
	}
	url := fmt.Sprintf("%s/projects/%s/models/%s/versions", baseURL, projectID, modelID)
	resp := doJSONWithIdempotency(t, client, http.MethodPost, url, payload, projectID, "modelver-"+uuid.NewString())
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create model version status=%d body=%s", resp.StatusCode, resp.Body)
	}
	var out struct {
		ModelVersion struct {
			ModelVersionID string `json:"modelVersionId"`
		} `json:"modelVersion"`
	}
	decodeJSONBody(t, resp.Body, &out)
	if out.ModelVersion.ModelVersionID == "" {
		t.Fatalf("model version id missing")
	}
	return out.ModelVersion.ModelVersionID
}

func transitionModelVersion(t *testing.T, client *http.Client, baseURL, projectID, versionID, action string) {
	url := fmt.Sprintf("%s/projects/%s/model-versions/%s:%s", baseURL, projectID, versionID, action)
	resp := doJSON(t, client, http.MethodPost, url, map[string]any{}, projectID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("model version %s status=%d body=%s", action, resp.StatusCode, resp.Body)
	}
}

func exportModelVersion(t *testing.T, client *http.Client, baseURL, projectID, versionID, idempotencyKey string) int {
	url := fmt.Sprintf("%s/projects/%s/model-versions/%s:export", baseURL, projectID, versionID)
	resp := doJSONWithIdempotency(t, client, http.MethodPost, url, map[string]any{}, projectID, idempotencyKey)
	return resp.StatusCode
}

func createWebhookSubscription(t *testing.T, client *http.Client, baseURL, projectID, targetURL string) string {
	payload := map[string]any{
		"name":        "e2e-webhook",
		"target_url":  targetURL,
		"event_types": []string{"RunFinished", "ModelApproved"},
	}
	url := fmt.Sprintf("%s/projects/%s/webhooks/subscriptions", baseURL, projectID)
	resp := doJSON(t, client, http.MethodPost, url, payload, projectID)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create webhook status=%d body=%s", resp.StatusCode, resp.Body)
	}
	var out struct {
		ID string `json:"id"`
	}
	decodeJSONBody(t, resp.Body, &out)
	if out.ID == "" {
		t.Fatalf("webhook subscription id missing")
	}
	return out.ID
}

type webhookDelivery struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	EventType string `json:"event_type"`
}

func listWebhookDeliveries(t *testing.T, client *http.Client, baseURL, projectID, eventType string) []webhookDelivery {
	url := fmt.Sprintf("%s/projects/%s/webhooks/deliveries?event_type=%s", baseURL, projectID, eventType)
	resp := doRequest(t, client, http.MethodGet, url, nil, projectID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list webhook deliveries status=%d body=%s", resp.StatusCode, resp.Body)
	}
	var out struct {
		Deliveries []webhookDelivery `json:"deliveries"`
	}
	decodeJSONBody(t, resp.Body, &out)
	return out.Deliveries
}

func waitForWebhookDelivery(t *testing.T, client *http.Client, baseURL, projectID, eventType string, minCount int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		deliveries := listWebhookDeliveries(t, client, baseURL, projectID, eventType)
		if len(deliveries) >= minCount {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("timeout waiting for webhook deliveries %s", eventType)
}

func firstWebhookDeliveryID(t *testing.T, client *http.Client, baseURL, projectID, eventType string) string {
	deliveries := listWebhookDeliveries(t, client, baseURL, projectID, eventType)
	if len(deliveries) == 0 {
		t.Fatalf("no webhook deliveries for %s", eventType)
	}
	return deliveries[0].ID
}

func replayWebhookDelivery(t *testing.T, client *http.Client, baseURL, projectID, deliveryID, token string) {
	url := fmt.Sprintf("%s/projects/%s/webhooks/deliveries/%s:replay", baseURL, projectID, deliveryID)
	resp := doJSONWithIdempotency(t, client, http.MethodPost, url, map[string]any{}, projectID, token)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("webhook replay status=%d body=%s", resp.StatusCode, resp.Body)
	}
}

func getLineageRun(t *testing.T, client *http.Client, baseURL, runID string) {
	url := fmt.Sprintf("%s/runs/%s", baseURL, runID)
	resp := doRequest(t, client, http.MethodGet, url, nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("lineage run status=%d body=%s", resp.StatusCode, resp.Body)
	}
}

func getLineageModelVersion(t *testing.T, client *http.Client, baseURL, versionID string) {
	url := fmt.Sprintf("%s/model-versions/%s", baseURL, versionID)
	resp := doRequest(t, client, http.MethodGet, url, nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("lineage model version status=%d body=%s", resp.StatusCode, resp.Body)
	}
}

func waitForAuditDLQ(t *testing.T, client *http.Client, baseURL string, minCount int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		url := fmt.Sprintf("%s/admin/audit/exports/deliveries?status=dlq", baseURL)
		resp := doRequest(t, client, http.MethodGet, url, nil, "")
		if resp.StatusCode == http.StatusOK {
			var out struct {
				Deliveries []struct {
					DeliveryID int64 `json:"delivery_id"`
				} `json:"deliveries"`
			}
			decodeJSONBody(t, resp.Body, &out)
			if len(out.Deliveries) >= minCount {
				return
			}
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("timeout waiting for audit DLQ")
}

func replayAuditDLQWithToken(t *testing.T, client *http.Client, baseURL, token string) {
	url := fmt.Sprintf("%s/admin/audit/exports/deliveries?status=dlq", baseURL)
	resp := doRequest(t, client, http.MethodGet, url, nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list dlq status=%d body=%s", resp.StatusCode, resp.Body)
	}
	var out struct {
		Deliveries []struct {
			DeliveryID int64 `json:"delivery_id"`
		} `json:"deliveries"`
	}
	decodeJSONBody(t, resp.Body, &out)
	if len(out.Deliveries) == 0 {
		t.Fatalf("expected at least one dlq delivery")
	}
	replayURL := fmt.Sprintf("%s/admin/audit/exports/dlq/%d:replay", baseURL, out.Deliveries[0].DeliveryID)
	resp = doJSONWithIdempotency(t, client, http.MethodPost, replayURL, map[string]any{}, "", token)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("replay status=%d body=%s", resp.StatusCode, resp.Body)
	}
}

func openDB(t *testing.T, dbURL string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("ping db: %v", err)
	}
	return db
}

func upsertRegistryPolicy(t *testing.T, db *sql.DB, projectID, mode, provider string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO registry_policies (project_id, mode, provider, created_at, updated_at)
		VALUES ($1,$2,$3,now(),now())
		ON CONFLICT (project_id) DO UPDATE SET mode = EXCLUDED.mode, provider = EXCLUDED.provider, updated_at = now()`, projectID, mode, provider)
	if err != nil {
		t.Fatalf("upsert registry policy: %v", err)
	}
}

func insertPolicyDecisionAllow(t *testing.T, db *sql.DB, runID string) {
	t.Helper()
	policyID := "e2e-policy"
	versionID := "e2e-policy-v1"
	decisionID := "e2e-decision-" + uuid.NewString()
	_, err := db.Exec(`INSERT INTO policies (policy_id, name, description, created_at, created_by, integrity_sha256)
		VALUES ($1,$2,$3,now(),$4,$5)
		ON CONFLICT (policy_id) DO NOTHING`, policyID, "e2e", "e2e", "e2e", "e2e")
	if err != nil {
		t.Fatalf("insert policy: %v", err)
	}
	_, err = db.Exec(`INSERT INTO policy_versions (policy_version_id, policy_id, version, status, spec_yaml, spec_json, spec_sha256, created_at, created_by, integrity_sha256)
		VALUES ($1,$2,$3,$4,$5,'{}'::jsonb,$6,now(),$7,$8)
		ON CONFLICT (policy_version_id) DO NOTHING`, versionID, policyID, 1, "active", "e2e", "e2e", "e2e", "e2e")
	if err != nil {
		t.Fatalf("insert policy version: %v", err)
	}
	_, err = db.Exec(`INSERT INTO policy_decisions (decision_id, run_id, policy_id, policy_version_id, policy_sha256, context, context_sha256, decision, rule_id, reason, created_at, created_by, integrity_sha256)
		VALUES ($1,$2,$3,$4,$5,'{}'::jsonb,$6,$7,$8,$9,now(),$10,$11)
		ON CONFLICT (decision_id) DO NOTHING`, decisionID, runID, policyID, versionID, "e2e", "e2e", "allow", "e2e", "e2e", "e2e", "e2e")
	if err != nil {
		t.Fatalf("insert policy decision: %v", err)
	}
}

func writeArtifacts(t *testing.T, cfg e2eConfig, state *e2eState) {
	t.Helper()
	if cfg.ArtifactsDir == "" {
		return
	}
	if err := os.MkdirAll(cfg.ArtifactsDir, 0o755); err != nil {
		t.Fatalf("create artifacts dir: %v", err)
	}
	payload := map[string]any{
		"project_id":         state.ProjectID,
		"dataset_id":         state.DatasetID,
		"dataset_version_id": state.DatasetVersionID,
		"env_definition_id":  state.EnvDefID,
		"env_lock_id":        state.EnvLockID,
		"run_id":             state.RunID,
		"artifact_id":        state.ArtifactID,
		"model_id":           state.ModelID,
		"model_version_id":   state.ModelVersionID,
		"webhook_sub_id":     state.WebhookSubID,
		"devenv_id":          state.DevEnvID,
		"devenv_session_id":  state.DevEnvSessionID,
		"deny_project_id":    state.DenyProjectID,
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	path := filepath.Join(cfg.ArtifactsDir, "e2e_ids.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write artifacts: %v", err)
	}
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func doJSON(t *testing.T, client *http.Client, method, url string, payload any, projectID string) httpResponse {
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	setRequestHeaders(req, projectID)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	body := readBody(t, resp.Body)
	return httpResponse{StatusCode: resp.StatusCode, Body: body}
}

func doJSONWithIdempotency(t *testing.T, client *http.Client, method, url string, payload any, projectID, idempotencyKey string) httpResponse {
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey == "" {
		idempotencyKey = "idem-" + uuid.NewString()
	}
	req.Header.Set("Idempotency-Key", idempotencyKey)
	setRequestHeaders(req, projectID)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	body := readBody(t, resp.Body)
	return httpResponse{StatusCode: resp.StatusCode, Body: body}
}

func doRequest(t *testing.T, client *http.Client, method, url string, body io.Reader, projectID string) httpResponse {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	setRequestHeaders(req, projectID)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	return httpResponse{StatusCode: resp.StatusCode, Body: readBody(t, resp.Body)}
}

func setRequestHeaders(req *http.Request, projectID string) {
	req.Header.Set("X-Request-Id", "req-"+uuid.NewString())
	if projectID != "" {
		req.Header.Set("X-Project-Id", projectID)
	}
}

func decodeJSONBody(t *testing.T, data []byte, out any) {
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode json: %v", err)
	}
}

func readBody(t *testing.T, r io.Reader) []byte {
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return data
}
