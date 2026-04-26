package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type apiClient struct {
	baseURL   string
	token     string
	requestID string
	http      *http.Client
}

func newAPIClient(baseURL, token, requestID string) *apiClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return &apiClient{
		baseURL:   baseURL,
		token:     strings.TrimSpace(token),
		requestID: strings.TrimSpace(requestID),
		http:      &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *apiClient) do(req *http.Request) (*http.Response, []byte, error) {
	if c.requestID != "" {
		req.Header.Set("X-Request-Id", c.requestID)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return resp, body, fmt.Errorf("http %s %s: status=%d body=%s", req.Method, req.URL.String(), resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return resp, body, nil
}

func (c *apiClient) getJSON(path string, out any) error {
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	_, body, err := c.do(req)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

func (c *apiClient) postJSON(path string, in any, out any) error {
	payload, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	_, body, err := c.do(req)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

func (c *apiClient) postMultipart(path string, body io.Reader, contentType string, out any) error {
	req, err := http.NewRequest("POST", c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", contentType)
	_, respBody, err := c.do(req)
	if err != nil {
		return err
	}
	return json.Unmarshal(respBody, out)
}

type dataset struct {
	DatasetID string `json:"dataset_id"`
	Name      string `json:"name"`
}

type qualityRule struct {
	RuleID string `json:"rule_id"`
	Name   string `json:"name"`
}

type datasetVersion struct {
	VersionID     string `json:"version_id"`
	DatasetID     string `json:"dataset_id"`
	QualityRuleID string `json:"quality_rule_id,omitempty"`
	ContentSHA256 string `json:"content_sha256"`
	Ordinal       int64  `json:"ordinal"`
}

type evaluation struct {
	EvaluationID string                 `json:"evaluation_id"`
	Status       string                 `json:"status"`
	Summary      map[string]any         `json:"summary"`
	ReportKey    string                 `json:"report_object_key"`
	RuleID       string                 `json:"rule_id"`
	VersionID    string                 `json:"dataset_version_id"`
	Extra        map[string]interface{} `json:"-"`
}

type experiment struct {
	ExperimentID string `json:"experiment_id"`
	Name         string `json:"name"`
}

type experimentRun struct {
	RunID      string `json:"run_id"`
	Status     string `json:"status"`
	Experiment string `json:"experiment_id"`
}

type lineageNode struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type lineageEvent struct {
	EventID     int64  `json:"event_id"`
	SubjectType string `json:"subject_type"`
	SubjectID   string `json:"subject_id"`
	Predicate   string `json:"predicate"`
	ObjectType  string `json:"object_type"`
	ObjectID    string `json:"object_id"`
}

type lineageSubgraph struct {
	Root  lineageNode    `json:"root"`
	Nodes []lineageNode  `json:"nodes"`
	Edges []lineageEvent `json:"edges"`
}

type auditEvent struct {
	EventID     int64  `json:"event_id"`
	OccurredAt  string `json:"occurred_at"`
	Actor       string `json:"actor"`
	Action      string `json:"action"`
	Resource    string `json:"resource_type"`
	ResourceID  string `json:"resource_id"`
	RequestID   string `json:"request_id"`
	Integrity   string `json:"integrity_sha256"`
	IP          string `json:"ip"`
	UserAgent   string `json:"user_agent"`
	PayloadJSON any    `json:"payload"`
}

type auditEventList struct {
	Events []auditEvent `json:"events"`
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func main() {
	now := time.Now().UTC()
	defaultRequestID := fmt.Sprintf("demo-%s", now.Format("20060102T150405Z"))
	defaultSuffix := now.Format("20060102-150405")

	var (
		baseURL     = flag.String("gateway", envOr("ANIMUS_GATEWAY_URL", "http://localhost:8080"), "Gateway base URL")
		datasetPath = flag.String("dataset", envOr("ANIMUS_DEMO_DATASET_PATH", "open/demo/data/demo.csv"), "Dataset file path")
		token       = flag.String("token", envOr("ANIMUS_BEARER_TOKEN", ""), "Bearer token (optional; required for OIDC mode)")
		requestID   = flag.String("request-id", envOr("ANIMUS_DEMO_REQUEST_ID", defaultRequestID), "X-Request-Id for correlation")
		nameSuffix  = flag.String("name-suffix", envOr("ANIMUS_DEMO_SUFFIX", defaultSuffix), "Suffix to avoid name collisions")
	)
	flag.Parse()

	client := newAPIClient(*baseURL, *token, *requestID)

	fmt.Printf("==> animus demo (gateway=%s, request_id=%s)\n", client.baseURL, client.requestID)

	// 1) Create quality rule
	ruleName := "demo-rule-" + *nameSuffix
	ruleSpec := map[string]any{
		"schema": "animus.quality.rule.v1",
		"checks": []any{
			map[string]any{"id": "size", "type": "object_size_bytes", "min_bytes": 1},
			map[string]any{"id": "content", "type": "content_type_in", "allowed": []string{"text/csv"}},
			map[string]any{"id": "suffix", "type": "filename_suffix_in", "allowed": []string{".csv"}},
			map[string]any{"id": "meta", "type": "metadata_required_keys", "keys": []string{"source"}},
			map[string]any{"id": "csv", "type": "csv_header_has_columns", "columns": []string{"feature1", "feature2", "label"}, "delimiter": ","},
			map[string]any{"id": "sha", "type": "verify_content_sha256"},
		},
	}

	var createdRule qualityRule
	if err := client.postJSON("/api/quality/rules", map[string]any{
		"name":        ruleName,
		"description": "Demo quality rule (deterministic)",
		"spec":        ruleSpec,
	}, &createdRule); err != nil {
		die("create quality rule", err)
	}
	fmt.Printf("==> created quality rule: %s (%s)\n", createdRule.RuleID, createdRule.Name)

	// 2) Create dataset
	dsName := "demo-dataset-" + *nameSuffix
	var createdDataset dataset
	if err := client.postJSON("/api/dataset-registry/datasets", map[string]any{
		"name":        dsName,
		"description": "Demo dataset for DataPilot",
		"metadata": map[string]any{
			"source":  "demo",
			"dataset": filepath.Base(*datasetPath),
		},
	}, &createdDataset); err != nil {
		die("create dataset", err)
	}
	fmt.Printf("==> created dataset: %s (%s)\n", createdDataset.DatasetID, createdDataset.Name)

	// 3) Upload dataset version (bound to quality rule)
	version, err := uploadDatasetVersion(client, createdDataset.DatasetID, *datasetPath, createdRule.RuleID)
	if err != nil {
		die("upload dataset version", err)
	}
	fmt.Printf("==> uploaded dataset version: %s (ordinal=%d sha256=%s)\n", version.VersionID, version.Ordinal, version.ContentSHA256[:12]+"…")

	// 4) Evaluate (must PASS for quality gate)
	var createdEval evaluation
	if err := client.postJSON("/api/quality/evaluations", map[string]any{
		"dataset_version_id": version.VersionID,
		"rule_id":            createdRule.RuleID,
	}, &createdEval); err != nil {
		die("create evaluation", err)
	}
	fmt.Printf("==> created evaluation: %s (status=%s)\n", createdEval.EvaluationID, createdEval.Status)
	if createdEval.Status != "pass" {
		die("quality gate did not pass", fmt.Errorf("status=%s", createdEval.Status))
	}

	// 5) Create experiment
	expName := "demo-exp-" + *nameSuffix
	var createdExperiment experiment
	if err := client.postJSON("/api/experiments/experiments", map[string]any{
		"name":        expName,
		"description": "Demo experiment (gated by quality pass)",
		"metadata": map[string]any{
			"dataset_id":         createdDataset.DatasetID,
			"dataset_version_id": version.VersionID,
			"evaluation_id":      createdEval.EvaluationID,
		},
	}, &createdExperiment); err != nil {
		die("create experiment", err)
	}
	fmt.Printf("==> created experiment: %s (%s)\n", createdExperiment.ExperimentID, createdExperiment.Name)

	// 6) Create experiment run (gated by quality pass)
	var createdRun experimentRun
	if err := client.postJSON(fmt.Sprintf("/api/experiments/experiments/%s/runs", createdExperiment.ExperimentID), map[string]any{
		"dataset_version_id": version.VersionID,
		"status":             "succeeded",
		"git_repo":           "https://example.local/animus/demo.git",
		"git_ref":            "refs/heads/main",
		"git_commit":         "0123456789abcdef0123456789abcdef01234567",
		"params": map[string]any{
			"learning_rate": 0.01,
			"epochs":        3,
		},
		"metrics": map[string]any{
			"accuracy": 0.9,
			"loss":     0.1,
		},
	}, &createdRun); err != nil {
		die("create experiment run", err)
	}
	fmt.Printf("==> created experiment run: %s (status=%s)\n", createdRun.RunID, createdRun.Status)

	// 7) Query lineage subgraph for the run
	var graph lineageSubgraph
	if err := client.getJSON(fmt.Sprintf("/api/lineage/subgraphs/experiment-runs/%s?depth=4&max_edges=200", createdRun.RunID), &graph); err != nil {
		die("fetch lineage subgraph", err)
	}
	fmt.Printf("==> lineage subgraph: nodes=%d edges=%d root=%s:%s\n", len(graph.Nodes), len(graph.Edges), graph.Root.Type, graph.Root.ID)

	// 8) Query audit events for this request id
	var audit auditEventList
	if err := client.getJSON(fmt.Sprintf("/api/audit/events?limit=200&request_id=%s", url.QueryEscape(client.requestID)), &audit); err != nil {
		die("fetch audit events", err)
	}
	fmt.Printf("==> audit events: count=%d (request_id=%s)\n", len(audit.Events), client.requestID)

	fmt.Println()
	fmt.Println("Next: open the control plane to inspect the objects.")
	fmt.Printf("  - dataset: /app/datasets/%s\n", createdDataset.DatasetID)
	fmt.Printf("  - version: /app/datasets/versions/%s\n", version.VersionID)
	fmt.Printf("  - evaluation: /app/quality/evaluations/%s\n", createdEval.EvaluationID)
	fmt.Printf("  - experiment: /app/experiments/%s\n", createdExperiment.ExperimentID)
	fmt.Printf("  - run: /app/experiments/runs/%s\n", createdRun.RunID)
	fmt.Printf("  - lineage: /app/lineage?type=run&id=%s\n", createdRun.RunID)
	fmt.Printf("  - audit: /app/audit?request_id=%s\n", client.requestID)
}

func uploadDatasetVersion(client *apiClient, datasetID, datasetPath, qualityRuleID string) (datasetVersion, error) {
	f, err := os.Open(datasetPath)
	if err != nil {
		return datasetVersion{}, err
	}
	defer f.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	meta := map[string]any{
		"source": "demo",
		"note":   "deterministic demo dataset version",
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return datasetVersion{}, err
	}
	if err := writer.WriteField("metadata", string(metaBytes)); err != nil {
		return datasetVersion{}, err
	}
	if err := writer.WriteField("quality_rule_id", qualityRuleID); err != nil {
		return datasetVersion{}, err
	}

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filepath.Base(datasetPath)))
	header.Set("Content-Type", "text/csv")
	part, err := writer.CreatePart(header)
	if err != nil {
		return datasetVersion{}, err
	}
	if _, err := io.Copy(part, f); err != nil {
		return datasetVersion{}, err
	}

	if err := writer.Close(); err != nil {
		return datasetVersion{}, err
	}

	var version datasetVersion
	if err := client.postMultipart(fmt.Sprintf("/api/dataset-registry/datasets/%s/versions/upload", datasetID), &buf, writer.FormDataContentType(), &version); err != nil {
		return datasetVersion{}, err
	}
	return version, nil
}

func die(step string, err error) {
	fmt.Fprintf(os.Stderr, "error: %s: %v\n", step, err)
	os.Exit(1)
}
