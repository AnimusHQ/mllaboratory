package auth

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

type oidcTestTransport struct {
	issuer   string
	clientID string
	key      *rsa.PrivateKey
	keyID    string
	nonce    string
	email    string
	roles    []string
	expiry   time.Time
}

func (t *oidcTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	switch req.URL.Path {
	case "/.well-known/openid-configuration":
		return jsonResponse(req, http.StatusOK, map[string]any{
			"issuer":                   t.issuer,
			"authorization_endpoint":   t.issuer + "/authorize",
			"token_endpoint":           t.issuer + "/token",
			"jwks_uri":                 t.issuer + "/jwks",
			"response_types_supported": []string{"code"},
		})
	case "/jwks":
		jwks, err := t.jwksJSON()
		if err != nil {
			return jsonResponse(req, http.StatusInternalServerError, map[string]any{"error": "jwks_failed"})
		}
		return jsonResponse(req, http.StatusOK, json.RawMessage(jwks))
	case "/token":
		idToken, err := t.signedIDToken()
		if err != nil {
			return jsonResponse(req, http.StatusInternalServerError, map[string]any{"error": "token_failed"})
		}
		return jsonResponse(req, http.StatusOK, map[string]any{
			"access_token": "access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"id_token":     idToken,
		})
	default:
		return jsonResponse(req, http.StatusNotFound, map[string]any{"error": "not_found"})
	}
}

func (t *oidcTestTransport) signedIDToken() (string, error) {
	headerJSON, err := json.Marshal(map[string]any{
		"alg": "RS256",
		"kid": t.keyID,
		"typ": "JWT",
	})
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(map[string]any{
		"iss":   t.issuer,
		"sub":   "user-1",
		"aud":   t.clientID,
		"iat":   time.Now().UTC().Unix(),
		"exp":   t.expiry.Unix(),
		"nonce": t.nonce,
		"email": t.email,
		"roles": t.roles,
	})
	if err != nil {
		return "", err
	}
	encodedHeader := base64.RawURLEncoding.EncodeToString(headerJSON)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := encodedHeader + "." + encodedClaims
	sum := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, t.key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	encodedSig := base64.RawURLEncoding.EncodeToString(sig)
	return signingInput + "." + encodedSig, nil
}

func (t *oidcTestTransport) jwksJSON() ([]byte, error) {
	modulus := base64.RawURLEncoding.EncodeToString(t.key.PublicKey.N.Bytes())
	exponent := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(t.key.PublicKey.E)).Bytes())
	return json.Marshal(map[string]any{
		"keys": []map[string]string{
			{
				"kty": "RSA",
				"kid": t.keyID,
				"alg": "RS256",
				"use": "sig",
				"n":   modulus,
				"e":   exponent,
			},
		},
	})
}

func jsonResponse(req *http.Request, status int, payload any) (*http.Response, error) {
	var body []byte
	switch typed := payload.(type) {
	case json.RawMessage:
		body = typed
	default:
		encoded, err := json.Marshal(payload)
		if err != nil {
			encoded = []byte("{}")
		}
		body = encoded
	}
	resp := &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     make(http.Header),
		Request:    req,
	}
	resp.Header.Set("Content-Type", "application/json")
	return resp, nil
}

func TestOIDCLoginCallbackCreatesSession(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	fixedNow := time.Now().UTC()
	issuer := "https://issuer.test"
	clientID := "client-1"
	transport := &oidcTestTransport{
		issuer:   issuer,
		clientID: clientID,
		key:      key,
		keyID:    "kid-1",
		email:    "user@example.com",
		roles:    []string{"viewer"},
		expiry:   fixedNow.Add(15 * time.Minute).Truncate(time.Second),
	}
	client := &http.Client{Transport: transport}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, client)

	cfg := Config{
		Mode:                  ModeOIDC,
		RolesClaim:            "roles",
		EmailClaim:            "email",
		SessionCookieName:     "animus_session",
		SessionCookieMaxAge:   time.Hour,
		SessionCookieSameSite: "Lax",
		OIDCIssuerURL:         issuer,
		OIDCClientID:          clientID,
		OIDCClientSecret:      "secret",
		OIDCRedirectURL:       "https://gateway.test/auth/callback",
		OIDCScopes:            []string{"openid", "profile", "email"},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("config validate: %v", err)
	}

	store := &stubSessionStore{}
	sessionManager := &SessionManager{
		Store: store,
		Now:   func() time.Time { return fixedNow },
	}

	svc, err := NewOIDCService(ctx, cfg, sessionManager)
	if err != nil {
		t.Fatalf("NewOIDCService: %v", err)
	}

	login, err := svc.LoginHandler()
	if err != nil {
		t.Fatalf("LoginHandler: %v", err)
	}

	loginReq := httptest.NewRequest(http.MethodGet, "/auth/login?return_to=/app", nil)
	loginResp := httptest.NewRecorder()
	login(loginResp, loginReq)

	if loginResp.Code != http.StatusFound {
		t.Fatalf("login status=%d, want 302", loginResp.Code)
	}

	location := loginResp.Header().Get("Location")
	if location == "" {
		t.Fatalf("missing redirect location")
	}
	u, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse redirect: %v", err)
	}
	q := u.Query()
	if q.Get("code_challenge_method") != "S256" {
		t.Fatalf("expected S256 pkce method")
	}
	if q.Get("code_challenge") == "" {
		t.Fatalf("expected code_challenge")
	}

	state := q.Get("state")
	nonce := q.Get("nonce")
	if state == "" || nonce == "" {
		t.Fatalf("expected state and nonce in redirect")
	}

	cookies := loginResp.Result().Cookies()
	stateCookie := cookieValue(cookies, "animus_oidc_state")
	verifierCookie := cookieValue(cookies, "animus_oidc_verifier")
	nonceCookie := cookieValue(cookies, "animus_oidc_nonce")
	returnToCookie := cookieValue(cookies, "animus_return_to")
	if stateCookie != state {
		t.Fatalf("state cookie mismatch")
	}
	if nonceCookie != nonce {
		t.Fatalf("nonce cookie mismatch")
	}
	if verifierCookie == "" {
		t.Fatalf("missing verifier cookie")
	}
	if returnToCookie != "/app" {
		t.Fatalf("return_to cookie=%q, want /app", returnToCookie)
	}

	transport.nonce = nonceCookie

	callback, err := svc.CallbackHandler()
	if err != nil {
		t.Fatalf("CallbackHandler: %v", err)
	}

	callbackReq := httptest.NewRequest(http.MethodGet, "/auth/callback?state="+url.QueryEscape(state)+"&code=code-1", nil)
	callbackReq = callbackReq.WithContext(context.WithValue(callbackReq.Context(), oauth2.HTTPClient, client))
	callbackReq.AddCookie(&http.Cookie{Name: "animus_oidc_state", Value: stateCookie})
	callbackReq.AddCookie(&http.Cookie{Name: "animus_oidc_verifier", Value: verifierCookie})
	callbackReq.AddCookie(&http.Cookie{Name: "animus_oidc_nonce", Value: nonceCookie})
	callbackReq.AddCookie(&http.Cookie{Name: "animus_return_to", Value: returnToCookie})

	callbackResp := httptest.NewRecorder()
	callback(callbackResp, callbackReq)

	if callbackResp.Code != http.StatusFound {
		t.Fatalf("callback status=%d, want 302", callbackResp.Code)
	}
	if got := callbackResp.Header().Get("Location"); got != "/app" {
		t.Fatalf("callback redirect=%q, want /app", got)
	}

	if store.record.Subject != "user-1" {
		t.Fatalf("subject=%q, want user-1", store.record.Subject)
	}
	if store.record.Email != "user@example.com" {
		t.Fatalf("email=%q, want user@example.com", store.record.Email)
	}
	if !contains(store.record.Roles, "viewer") {
		t.Fatalf("roles=%v, want viewer", store.record.Roles)
	}
	if !store.record.ExpiresAt.Equal(transport.expiry) {
		t.Fatalf("expires_at=%s, want %s", store.record.ExpiresAt, transport.expiry)
	}

	sessionCookie := cookieValue(callbackResp.Result().Cookies(), cfg.SessionCookieName)
	if sessionCookie == "" {
		t.Fatalf("missing session cookie")
	}
	if sessionCookie != store.record.SessionID {
		t.Fatalf("session cookie mismatch")
	}
}

func cookieValue(cookies []*http.Cookie, name string) string {
	for _, cookie := range cookies {
		if strings.EqualFold(cookie.Name, name) {
			return cookie.Value
		}
	}
	return ""
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
