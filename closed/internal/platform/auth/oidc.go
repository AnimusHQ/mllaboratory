package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type OIDCService struct {
	cfg          Config
	provider     *oidc.Provider
	verifier     *oidc.IDTokenVerifier
	oauth2Config oauth2.Config
	sessions     *SessionManager
}

func NewOIDCService(ctx context.Context, cfg Config, sessions *SessionManager) (*OIDCService, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.Mode != ModeOIDC {
		return nil, fmt.Errorf("auth mode must be oidc (got %q)", cfg.Mode)
	}

	provider, err := oidc.NewProvider(ctx, cfg.OIDCIssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc provider: %w", err)
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.OIDCClientID})
	oauth2Cfg := oauth2.Config{
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  cfg.OIDCRedirectURL,
		Scopes:       cfg.OIDCScopes,
	}

	return &OIDCService{
		cfg:          cfg,
		provider:     provider,
		verifier:     verifier,
		oauth2Config: oauth2Cfg,
		sessions:     sessions,
	}, nil
}

func (s *OIDCService) Authenticate(ctx context.Context, r *http.Request) (Identity, error) {
	rawToken := tokenFromHeader(r)
	if rawToken == "" {
		if s.sessions != nil {
			sessionID := tokenFromCookie(r, s.cfg.SessionCookieName)
			if sessionID == "" {
				return Identity{}, ErrUnauthenticated
			}
			session, err := s.sessions.GetSession(ctx, sessionID)
			if err != nil {
				return Identity{}, ErrUnauthenticated
			}
			return Identity{
				Subject: session.Subject,
				Email:   session.Email,
				Roles:   session.Roles,
			}, nil
		}
		rawToken = tokenFromCookie(r, s.cfg.SessionCookieName)
	}
	if rawToken == "" {
		return Identity{}, ErrUnauthenticated
	}

	idToken, err := s.verifier.Verify(ctx, rawToken)
	if err != nil {
		return Identity{}, err
	}

	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return Identity{}, err
	}

	return identityFromClaims(s.cfg, claims), nil
}

func (s *OIDCService) LoginHandler() (http.HandlerFunc, error) {
	if err := s.cfg.ValidateForLogin(); err != nil {
		return nil, err
	}

	return func(w http.ResponseWriter, r *http.Request) {
		returnTo := SafeReturnTo(r.URL.Query().Get("return_to"), s.cfg)

		state, err := randomBase64URL(32)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "internal_error"})
			return
		}
		verifier, err := randomBase64URL(32)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "internal_error"})
			return
		}
		nonce, err := randomBase64URL(32)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "internal_error"})
			return
		}
		challenge := pkceS256Challenge(verifier)

		setShortCookie(w, "animus_oidc_state", state, s.cfg)
		setShortCookie(w, "animus_oidc_verifier", verifier, s.cfg)
		setShortCookie(w, "animus_oidc_nonce", nonce, s.cfg)
		setShortCookie(w, "animus_return_to", returnTo, s.cfg)

		redirectURL := s.oauth2Config.AuthCodeURL(
			state,
			oauth2.AccessTypeOnline,
			oauth2.SetAuthURLParam("code_challenge", challenge),
			oauth2.SetAuthURLParam("code_challenge_method", "S256"),
			oauth2.SetAuthURLParam("nonce", nonce),
		)
		http.Redirect(w, r, redirectURL, http.StatusFound)
	}, nil
}

func (s *OIDCService) CallbackHandler() (http.HandlerFunc, error) {
	if err := s.cfg.ValidateForLogin(); err != nil {
		return nil, err
	}

	return func(w http.ResponseWriter, r *http.Request) {
		stateQuery := r.URL.Query().Get("state")
		code := r.URL.Query().Get("code")
		if stateQuery == "" || code == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing_code_or_state"})
			return
		}

		stateCookie := tokenFromCookie(r, "animus_oidc_state")
		if stateCookie == "" || stateCookie != stateQuery {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_state"})
			return
		}

		codeVerifier := tokenFromCookie(r, "animus_oidc_verifier")
		nonceCookie := tokenFromCookie(r, "animus_oidc_nonce")
		returnTo := SafeReturnTo(tokenFromCookie(r, "animus_return_to"), s.cfg)
		if codeVerifier == "" || nonceCookie == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing_pkce_or_nonce"})
			return
		}

		exchangeCtx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		token, err := s.oauth2Config.Exchange(exchangeCtx, code, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "token_exchange_failed"})
			return
		}

		rawIDToken, ok := token.Extra("id_token").(string)
		if !ok || rawIDToken == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "missing_id_token"})
			return
		}

		idToken, err := s.verifier.Verify(exchangeCtx, rawIDToken)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid_id_token"})
			return
		}

		var nonceClaim struct {
			Nonce string `json:"nonce"`
		}
		if err := idToken.Claims(&nonceClaim); err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid_id_token_claims"})
			return
		}
		if nonceClaim.Nonce == "" || nonceClaim.Nonce != nonceCookie {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid_nonce"})
			return
		}

		var claims map[string]any
		if err := idToken.Claims(&claims); err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid_id_token_claims"})
			return
		}

		sessionID := rawIDToken
		if s.sessions != nil {
			meta := SessionRequestMeta{
				RequestID: r.Header.Get("X-Request-Id"),
				UserAgent: r.UserAgent(),
				RemoteIP:  ParseRemoteIP(r.RemoteAddr),
				Actor:     extractSubjectClaim(claims),
			}
			now := time.Now().UTC()
			if s.sessions != nil && s.sessions.Now != nil {
				now = s.sessions.Now().UTC()
			}
			expiresAt := now.Add(s.cfg.SessionCookieMaxAge)
			if !idToken.Expiry.IsZero() && idToken.Expiry.Before(expiresAt) {
				expiresAt = idToken.Expiry.UTC()
			}
			identity := identityFromClaims(s.cfg, claims)
			session, err := s.sessions.CreateSession(r.Context(), identity, s.cfg.OIDCIssuerURL, expiresAt, TokenSHA256(rawIDToken), meta)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "session_create_failed"})
				return
			}
			sessionID = session.SessionID
		}

		setSessionCookie(w, s.cfg.SessionCookieName, sessionID, s.cfg)
		clearCookie(w, "animus_oidc_state", s.cfg)
		clearCookie(w, "animus_oidc_verifier", s.cfg)
		clearCookie(w, "animus_oidc_nonce", s.cfg)
		clearCookie(w, "animus_return_to", s.cfg)

		http.Redirect(w, r, returnTo, http.StatusFound)
	}, nil
}

func (s *OIDCService) LogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.sessions != nil {
			sessionID := tokenFromCookie(r, s.cfg.SessionCookieName)
			if sessionID != "" {
				_, _ = s.sessions.RevokeSession(r.Context(), sessionID, "user", "logout", SessionRequestMeta{
					RequestID: r.Header.Get("X-Request-Id"),
					UserAgent: r.UserAgent(),
					RemoteIP:  ParseRemoteIP(r.RemoteAddr),
				})
			}
		}
		clearCookie(w, s.cfg.SessionCookieName, s.cfg)
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	}
}

func (s *OIDCService) SessionHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := s.Authenticate(r.Context(), r)
		if err != nil {
			if errors.Is(err, ErrUnauthenticated) {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
				return
			}
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid_token"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"subject": identity.Subject,
			"email":   identity.Email,
			"roles":   identity.Roles,
		})
	}
}

func tokenFromHeader(r *http.Request) string {
	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if authz == "" {
		return ""
	}
	parts := strings.SplitN(authz, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	if strings.ToLower(parts[0]) != "bearer" {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func tokenFromCookie(r *http.Request, name string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func randomBase64URL(nBytes int) (string, error) {
	if nBytes <= 0 {
		return "", errors.New("nBytes must be positive")
	}
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func pkceS256Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func SafeReturnTo(raw string, cfg Config) string {
	defaultPath := "/console"
	if strings.TrimSpace(raw) == "" {
		return defaultPath
	}
	u, err := url.Parse(raw)
	if err != nil {
		return defaultPath
	}
	if u.IsAbs() {
		if !isAllowedReturnToOrigin(u, cfg) {
			return defaultPath
		}
		return u.String()
	}
	if !strings.HasPrefix(u.Path, "/") {
		return defaultPath
	}
	if strings.HasPrefix(u.Path, "//") {
		return defaultPath
	}
	if u.RawQuery != "" {
		return u.Path + "?" + u.RawQuery
	}
	return u.Path
}

func isAllowedReturnToOrigin(u *url.URL, cfg Config) bool {
	origin := strings.ToLower(strings.TrimSpace(u.Scheme + "://" + u.Host))
	if origin == "" {
		return false
	}
	allowed := allowedReturnToOrigins(cfg)
	_, ok := allowed[origin]
	return ok
}

func allowedReturnToOrigins(cfg Config) map[string]struct{} {
	out := make(map[string]struct{})
	if strings.TrimSpace(cfg.PublicBaseURL) != "" {
		if origin := parseOrigin(cfg.PublicBaseURL); origin != "" {
			out[origin] = struct{}{}
		}
	}
	for _, raw := range cfg.AllowedReturnToOrigins {
		if origin := parseOrigin(raw); origin != "" {
			out[origin] = struct{}{}
		}
	}
	return out
}

func parseOrigin(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host)
}

func setShortCookie(w http.ResponseWriter, name string, value string, cfg Config) {
	setCookie(w, name, value, 10*time.Minute, cfg)
}

func setSessionCookie(w http.ResponseWriter, name string, value string, cfg Config) {
	setCookie(w, name, value, cfg.SessionCookieMaxAge, cfg)
}

func clearCookie(w http.ResponseWriter, name string, cfg Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cfg.SessionCookieSecure,
		SameSite: parseSameSite(cfg.SessionCookieSameSite),
	})
}

func setCookie(w http.ResponseWriter, name string, value string, ttl time.Duration, cfg Config) {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   cfg.SessionCookieSecure,
		SameSite: parseSameSite(cfg.SessionCookieSameSite),
	})
}

func parseSameSite(raw string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

func identityFromClaims(cfg Config, claims map[string]any) Identity {
	return Identity{
		Subject: extractSubjectClaim(claims),
		Email:   extractStringClaim(claims, cfg.EmailClaim),
		Roles:   resolveRolesFromClaims(cfg, claims),
	}
}

func resolveRolesFromClaims(cfg Config, claims map[string]any) []string {
	roles := extractRolesClaim(claims, cfg.RolesClaim)
	groups := extractRolesClaim(claims, cfg.GroupsClaim)
	mapped := mapGroupsToRoles(groups, cfg.GroupRoleMap)
	return mergeUnique(roles, groups, mapped)
}

func mapGroupsToRoles(groups []string, mapping map[string]string) []string {
	if len(groups) == 0 || len(mapping) == 0 {
		return nil
	}
	out := make([]string, 0, len(groups))
	seen := make(map[string]struct{})
	for _, group := range groups {
		key := strings.ToLower(strings.TrimSpace(group))
		if key == "" {
			continue
		}
		role, ok := mapping[key]
		if !ok || role == "" {
			continue
		}
		if _, ok := seen[role]; ok {
			continue
		}
		seen[role] = struct{}{}
		out = append(out, role)
	}
	return out
}

func mergeUnique(values ...[]string) []string {
	out := make([]string, 0)
	seen := make(map[string]struct{})
	for _, slice := range values {
		for _, item := range slice {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}

func extractStringClaim(claims map[string]any, key string) string {
	v, ok := claims[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func extractSubjectClaim(claims map[string]any) string {
	v, ok := claims["sub"]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func extractRolesClaim(claims map[string]any, key string) []string {
	v, ok := claims[key]
	if !ok {
		return nil
	}
	switch typed := v.(type) {
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			s, ok := item.(string)
			if !ok {
				continue
			}
			s = strings.ToLower(strings.TrimSpace(s))
			if s == "" {
				continue
			}
			out = append(out, s)
		}
		return out
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			s := strings.ToLower(strings.TrimSpace(item))
			if s == "" {
				continue
			}
			out = append(out, s)
		}
		return out
	case string:
		return parseCSV(typed)
	default:
		return nil
	}
}
