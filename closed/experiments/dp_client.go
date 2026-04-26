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
	dataplaneActorSubject = "system:control-plane"
	dataplaneActorRoles   = "admin"
)

type dataplaneClient struct {
	baseURL    string
	secret     string
	httpClient *http.Client
}

func newDataplaneClient(baseURL, secret string) (*dataplaneClient, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("dataplane base url is required")
	}
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("internal auth secret is required")
	}
	return &dataplaneClient{
		baseURL: baseURL,
		secret:  secret,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (c *dataplaneClient) ExecuteRun(ctx context.Context, req dataplane.RunExecutionRequest, requestID string) (dataplane.RunExecutionResponse, int, error) {
	if c == nil {
		return dataplane.RunExecutionResponse{}, 0, errors.New("dataplane client not initialized")
	}
	path := fmt.Sprintf("/internal/dp/runs/%s:execute", strings.TrimSpace(req.RunID))
	var resp dataplane.RunExecutionResponse
	status, err := c.postJSON(ctx, path, req, requestID, &resp)
	return resp, status, err
}

func (c *dataplaneClient) GetRunStatus(ctx context.Context, projectID, runID, requestID string) (dataplane.RunExecutionStatus, int, error) {
	if c == nil {
		return dataplane.RunExecutionStatus{}, 0, errors.New("dataplane client not initialized")
	}
	path := fmt.Sprintf("/internal/dp/runs/%s/status?project_id=%s", strings.TrimSpace(runID), strings.TrimSpace(projectID))
	var resp dataplane.RunExecutionStatus
	status, err := c.getJSON(ctx, path, requestID, &resp)
	return resp, status, err
}

func (c *dataplaneClient) ProvisionDevEnv(ctx context.Context, req dataplane.DevEnvProvisionRequest, requestID string) (dataplane.DevEnvProvisionResponse, int, error) {
	if c == nil {
		return dataplane.DevEnvProvisionResponse{}, 0, errors.New("dataplane client not initialized")
	}
	path := fmt.Sprintf("/internal/dp/dev-envs/%s:create", strings.TrimSpace(req.DevEnvID))
	var resp dataplane.DevEnvProvisionResponse
	status, err := c.postJSON(ctx, path, req, requestID, &resp)
	return resp, status, err
}

func (c *dataplaneClient) DeleteDevEnv(ctx context.Context, req dataplane.DevEnvDeleteRequest, requestID string) (dataplane.DevEnvDeleteResponse, int, error) {
	if c == nil {
		return dataplane.DevEnvDeleteResponse{}, 0, errors.New("dataplane client not initialized")
	}
	path := fmt.Sprintf("/internal/dp/dev-envs/%s:delete", strings.TrimSpace(req.DevEnvID))
	var resp dataplane.DevEnvDeleteResponse
	status, err := c.postJSON(ctx, path, req, requestID, &resp)
	return resp, status, err
}

func (c *dataplaneClient) AccessDevEnv(ctx context.Context, req dataplane.DevEnvAccessRequest, requestID string) (dataplane.DevEnvAccessResponse, int, error) {
	if c == nil {
		return dataplane.DevEnvAccessResponse{}, 0, errors.New("dataplane client not initialized")
	}
	path := fmt.Sprintf("/internal/dp/dev-envs/%s/access", strings.TrimSpace(req.DevEnvID))
	var resp dataplane.DevEnvAccessResponse
	status, err := c.postJSON(ctx, path, req, requestID, &resp)
	return resp, status, err
}

func (c *dataplaneClient) postJSON(ctx context.Context, path string, payload any, requestID string, out any) (int, error) {
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

func (c *dataplaneClient) getJSON(ctx context.Context, path string, requestID string, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return 0, err
	}
	return c.doJSON(req, requestID, out)
}

func (c *dataplaneClient) doJSON(req *http.Request, requestID string, out any) (int, error) {
	if req == nil {
		return 0, errors.New("request required")
	}
	if strings.TrimSpace(requestID) == "" {
		requestID = uuid.NewString()
	}
	ts := fmt.Sprintf("%d", time.Now().UTC().Unix())
	sig, err := auth.ComputeInternalAuthSignature(c.secret, ts, req.Method, req.URL.Path, requestID, dataplaneActorSubject, "", dataplaneActorRoles)
	if err != nil {
		return 0, err
	}
	req.Header.Set("X-Request-Id", requestID)
	req.Header.Set(auth.HeaderSubject, dataplaneActorSubject)
	req.Header.Set(auth.HeaderRoles, dataplaneActorRoles)
	req.Header.Set(auth.HeaderInternalAuthTimestamp, ts)
	req.Header.Set(auth.HeaderInternalAuthSignature, sig)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("dataplane error status: %d", resp.StatusCode)
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
