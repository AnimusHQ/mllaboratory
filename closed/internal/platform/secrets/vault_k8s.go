package secrets

import (
	"bytes"
	"context"
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
	defaultVaultAuthPath = "auth/kubernetes/login"
	defaultVaultJWTPath  = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

type VaultK8sManager struct {
	addr      string
	authPath  string
	role      string
	jwtPath   string
	namespace string
	leaseTT   time.Duration
	http      *http.Client
}

type vaultAuthResponse struct {
	Auth struct {
		ClientToken   string `json:"client_token"`
		LeaseDuration int    `json:"lease_duration"`
	} `json:"auth"`
}

type vaultSecretResponse struct {
	Data          map[string]any `json:"data"`
	LeaseID       string         `json:"lease_id"`
	LeaseDuration int            `json:"lease_duration"`
}

func NewVaultK8sManager(cfg Config) (*VaultK8sManager, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	addr := strings.TrimRight(strings.TrimSpace(cfg.VaultAddr), "/")
	if addr == "" {
		return nil, errors.New("vault addr is required")
	}
	role := strings.TrimSpace(cfg.VaultRole)
	if role == "" {
		return nil, errors.New("vault role is required")
	}
	authPath := strings.Trim(strings.TrimSpace(cfg.VaultAuthPath), "/")
	if authPath == "" {
		authPath = defaultVaultAuthPath
	}
	jwtPath := strings.TrimSpace(cfg.VaultJWTPath)
	if jwtPath == "" {
		jwtPath = defaultVaultJWTPath
	}
	timeout := cfg.VaultTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &VaultK8sManager{
		addr:      addr,
		authPath:  authPath,
		role:      role,
		jwtPath:   jwtPath,
		namespace: strings.TrimSpace(cfg.VaultNamespace),
		leaseTT:   cfg.LeaseTTL,
		http:      &http.Client{Timeout: timeout},
	}, nil
}

func (m *VaultK8sManager) Fetch(ctx context.Context, req Request) (Lease, error) {
	if m == nil {
		return Lease{}, errors.New("secrets manager not initialized")
	}
	classRef := strings.TrimSpace(req.ClassRef)
	if classRef == "" {
		return Lease{Env: map[string]string{}, ExpiresAt: time.Now().UTC().Add(m.leaseTTL())}, nil
	}
	token, authTTL, err := m.login(ctx)
	if err != nil {
		return Lease{}, err
	}
	secretEnv, leaseID, secretTTL, err := m.readSecret(ctx, token, classRef)
	if err != nil {
		return Lease{}, err
	}
	ttl := secretTTL
	if ttl <= 0 {
		ttl = authTTL
	}
	if ttl <= 0 {
		ttl = m.leaseTTL()
	}
	if ttl <= 0 {
		return Lease{}, errors.New("vault lease ttl required")
	}
	return Lease{
		LeaseID:   leaseID,
		Env:       secretEnv,
		ExpiresAt: time.Now().UTC().Add(ttl),
	}, nil
}

func (m *VaultK8sManager) login(ctx context.Context) (string, time.Duration, error) {
	jwt, err := readJWT(m.jwtPath)
	if err != nil {
		return "", 0, err
	}
	payload, err := json.Marshal(map[string]string{
		"role": m.role,
		"jwt":  jwt,
	})
	if err != nil {
		return "", 0, errors.New("vault auth payload failed")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.url(m.authPath), bytes.NewReader(payload))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	m.applyNamespace(req)

	resp, body, err := m.do(req)
	if err != nil {
		return "", 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("vault auth failed (status=%d)", resp.StatusCode)
	}
	var parsed vaultAuthResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", 0, errors.New("vault auth decode failed")
	}
	token := strings.TrimSpace(parsed.Auth.ClientToken)
	if token == "" {
		return "", 0, errors.New("vault auth token missing")
	}
	if parsed.Auth.LeaseDuration <= 0 {
		return "", 0, errors.New("vault auth lease expired")
	}
	return token, time.Duration(parsed.Auth.LeaseDuration) * time.Second, nil
}

func (m *VaultK8sManager) readSecret(ctx context.Context, token, classRef string) (map[string]string, string, time.Duration, error) {
	classRef = strings.Trim(strings.TrimSpace(classRef), "/")
	if classRef == "" {
		return map[string]string{}, "", 0, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.url(classRef), nil)
	if err != nil {
		return nil, "", 0, err
	}
	req.Header.Set("X-Vault-Token", token)
	m.applyNamespace(req)

	resp, body, err := m.do(req)
	if err != nil {
		return nil, "", 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", 0, fmt.Errorf("vault secret read failed (status=%d)", resp.StatusCode)
	}
	var parsed vaultSecretResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, "", 0, errors.New("vault secret decode failed")
	}
	payload := parsed.Data
	if nested, ok := parsed.Data["data"]; ok {
		if inner, ok := nested.(map[string]any); ok {
			payload = inner
		}
	}
	env := map[string]string{}
	for k, v := range payload {
		key := strings.TrimSpace(k)
		if key == "" || v == nil {
			continue
		}
		switch typed := v.(type) {
		case string:
			env[key] = typed
		default:
			env[key] = fmt.Sprint(typed)
		}
	}
	return env, strings.TrimSpace(parsed.LeaseID), time.Duration(parsed.LeaseDuration) * time.Second, nil
}

func (m *VaultK8sManager) applyNamespace(req *http.Request) {
	if req == nil {
		return
	}
	if m.namespace == "" {
		return
	}
	req.Header.Set("X-Vault-Namespace", m.namespace)
}

func (m *VaultK8sManager) url(path string) string {
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "" {
		return m.addr
	}
	if !strings.HasPrefix(path, "v1/") {
		path = "v1/" + path
	}
	return m.addr + "/" + path
}

func (m *VaultK8sManager) leaseTTL() time.Duration {
	if m.leaseTT <= 0 {
		return 10 * time.Minute
	}
	return m.leaseTT
}

func (m *VaultK8sManager) do(req *http.Request) (*http.Response, []byte, error) {
	if req == nil {
		return nil, nil, errors.New("request is required")
	}
	req.Header.Set("Accept", "application/json")
	resp, err := m.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, nil, err
	}
	return resp, body, nil
}

func readJWT(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read serviceaccount jwt: %w", err)
	}
	jwt := strings.TrimSpace(string(data))
	if jwt == "" {
		return "", errors.New("serviceaccount jwt is empty")
	}
	return jwt, nil
}
