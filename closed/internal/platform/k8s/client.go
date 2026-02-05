package k8s

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultTokenFile     = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	defaultNamespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	defaultCAFile        = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

var (
	ErrNotFound      = errors.New("kubernetes resource not found")
	ErrAlreadyExists = errors.New("kubernetes resource already exists")
	ErrUnauthorized  = errors.New("kubernetes request unauthorized")
	ErrForbidden     = errors.New("kubernetes request forbidden")
	ErrUnexpectedAPI = errors.New("kubernetes unexpected response")
)

type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	body := strings.TrimSpace(e.Body)
	if body == "" {
		return fmt.Sprintf("kubernetes api error (status=%d)", e.StatusCode)
	}
	return fmt.Sprintf("kubernetes api error (status=%d): %s", e.StatusCode, body)
}

type Client struct {
	baseURL   string
	token     string
	namespace string
	http      *http.Client
}

func NewInClusterClient() (*Client, error) {
	host := strings.TrimSpace(os.Getenv("KUBERNETES_SERVICE_HOST"))
	port := strings.TrimSpace(os.Getenv("KUBERNETES_SERVICE_PORT"))
	baseURL := "https://kubernetes.default.svc"
	if host != "" {
		if port == "" {
			port = "443"
		}
		baseURL = "https://" + host + ":" + port
	}
	return NewInClusterClientWithBaseURL(baseURL)
}

func NewInClusterClientWithBaseURL(baseURL string) (*Client, error) {
	tokenBytes, err := os.ReadFile(defaultTokenFile)
	if err != nil {
		return nil, fmt.Errorf("read serviceaccount token: %w", err)
	}
	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		return nil, errors.New("serviceaccount token is empty")
	}

	namespaceBytes, err := os.ReadFile(defaultNamespaceFile)
	if err != nil {
		return nil, fmt.Errorf("read serviceaccount namespace: %w", err)
	}
	namespace := strings.TrimSpace(string(namespaceBytes))
	if namespace == "" {
		return nil, errors.New("serviceaccount namespace is empty")
	}

	caBytes, err := os.ReadFile(defaultCAFile)
	if err != nil {
		return nil, fmt.Errorf("read serviceaccount ca: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caBytes) {
		return nil, errors.New("invalid serviceaccount ca bundle")
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
	}

	return &Client{
		baseURL:   strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:     token,
		namespace: namespace,
		http: &http.Client{
			Transport: transport,
			Timeout:   15 * time.Second,
		},
	}, nil
}

func (c *Client) Namespace() string {
	return c.namespace
}

func (c *Client) CreateJob(ctx context.Context, namespace string, job Job) error {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		namespace = c.namespace
	}
	job.APIVersion = "batch/v1"
	job.Kind = "Job"
	job.Metadata.Namespace = namespace

	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	path := fmt.Sprintf("/apis/batch/v1/namespaces/%s/jobs", namespace)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, nil)
}

func (c *Client) CreateService(ctx context.Context, namespace string, service Service) error {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		namespace = c.namespace
	}
	service.APIVersion = "v1"
	service.Kind = "Service"
	service.Metadata.Namespace = namespace

	body, err := json.Marshal(service)
	if err != nil {
		return fmt.Errorf("marshal service: %w", err)
	}
	path := fmt.Sprintf("/api/v1/namespaces/%s/services", namespace)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, nil)
}

func (c *Client) GetJob(ctx context.Context, namespace string, name string) (Job, error) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		namespace = c.namespace
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Job{}, errors.New("job name is required")
	}
	path := fmt.Sprintf("/apis/batch/v1/namespaces/%s/jobs/%s", namespace, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return Job{}, err
	}
	var out Job
	if err := c.do(req, &out); err != nil {
		return Job{}, err
	}
	return out, nil
}

func (c *Client) DeleteJob(ctx context.Context, namespace string, name string) error {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		namespace = c.namespace
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("job name is required")
	}
	path := fmt.Sprintf("/apis/batch/v1/namespaces/%s/jobs/%s", namespace, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func (c *Client) DeleteService(ctx context.Context, namespace string, name string) error {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		namespace = c.namespace
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("service name is required")
	}
	path := fmt.Sprintf("/api/v1/namespaces/%s/services/%s", namespace, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func (c *Client) do(req *http.Request, out any) error {
	if req == nil {
		return errors.New("request is required")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusNoContent:
		if out == nil {
			return nil
		}
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("decode kubernetes response: %w", err)
		}
		return nil
	case http.StatusConflict:
		return ErrAlreadyExists
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	default:
		return &APIError{StatusCode: resp.StatusCode, Body: string(body)}
	}
}
