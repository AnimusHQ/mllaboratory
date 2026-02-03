package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/dataplane"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/google/uuid"
)

const (
	controlPlaneSubject = "system:dataplane"
	controlPlaneRoles   = "admin"
)

type controlPlaneClient struct {
	baseURL    string
	secret     string
	httpClient *http.Client
}

func newControlPlaneClient(baseURL, secret string) (*controlPlaneClient, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("control plane base url is required")
	}
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("internal auth secret is required")
	}
	return &controlPlaneClient{
		baseURL: baseURL,
		secret:  secret,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

type reproBundle struct {
	Schema            string          `json:"schema"`
	RunID             string          `json:"runId"`
	ProjectID         string          `json:"projectId"`
	SpecHash          string          `json:"specHash"`
	RunSpec           json.RawMessage `json:"runSpec"`
	PolicySnapshotSHA string          `json:"policySnapshotSha256"`
	GeneratedAt       time.Time       `json:"generatedAt"`
	GeneratedBy       string          `json:"generatedBy,omitempty"`
}

func (c *controlPlaneClient) GetReproBundle(ctx context.Context, projectID, runID, requestID string) (reproBundle, int, error) {
	path := fmt.Sprintf("/projects/%s/runs/%s/reproducibility-bundle", strings.TrimSpace(projectID), strings.TrimSpace(runID))
	var out reproBundle
	status, err := c.getJSON(ctx, path, requestID, &out)
	return out, status, err
}

func (c *controlPlaneClient) SendHeartbeat(ctx context.Context, heartbeat dataplane.RunHeartbeat, requestID string) (dataplane.RunHeartbeatResponse, int, error) {
	path := fmt.Sprintf("/internal/cp/runs/%s/heartbeat", strings.TrimSpace(heartbeat.RunID))
	var out dataplane.RunHeartbeatResponse
	status, err := c.postJSON(ctx, path, heartbeat, requestID, &out)
	return out, status, err
}

func (c *controlPlaneClient) SendTerminal(ctx context.Context, terminal dataplane.RunTerminalState, requestID string) (dataplane.RunTerminalResponse, int, error) {
	path := fmt.Sprintf("/internal/cp/runs/%s/terminal", strings.TrimSpace(terminal.RunID))
	var out dataplane.RunTerminalResponse
	status, err := c.postJSON(ctx, path, terminal, requestID, &out)
	return out, status, err
}

func (c *controlPlaneClient) SendArtifactCommitted(ctx context.Context, event dataplane.ArtifactCommitted, requestID string) (dataplane.ArtifactCommittedResponse, int, error) {
	path := fmt.Sprintf("/internal/cp/runs/%s/artifact-committed", strings.TrimSpace(event.RunID))
	var out dataplane.ArtifactCommittedResponse
	status, err := c.postJSON(ctx, path, event, requestID, &out)
	return out, status, err
}

func (c *controlPlaneClient) postJSON(ctx context.Context, path string, payload any, requestID string, out any) (int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.doJSON(req, requestID, out)
}

func (c *controlPlaneClient) getJSON(ctx context.Context, path string, requestID string, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return 0, err
	}
	return c.doJSON(req, requestID, out)
}

func (c *controlPlaneClient) doJSON(req *http.Request, requestID string, out any) (int, error) {
	if req == nil {
		return 0, errors.New("request required")
	}
	if strings.TrimSpace(requestID) == "" {
		requestID = uuid.NewString()
	}
	ts := fmt.Sprintf("%d", time.Now().UTC().Unix())
	sig, err := auth.ComputeInternalAuthSignature(c.secret, ts, req.Method, req.URL.Path, requestID, controlPlaneSubject, "", controlPlaneRoles)
	if err != nil {
		return 0, err
	}
	req.Header.Set("X-Request-Id", requestID)
	req.Header.Set(auth.HeaderSubject, controlPlaneSubject)
	req.Header.Set(auth.HeaderRoles, controlPlaneRoles)
	req.Header.Set(auth.HeaderInternalAuthTimestamp, ts)
	req.Header.Set(auth.HeaderInternalAuthSignature, sig)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("control plane error status: %d", resp.StatusCode)
	}
	if out == nil {
		return resp.StatusCode, nil
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(out); err != nil {
		return resp.StatusCode, err
	}
	return resp.StatusCode, nil
}
