package auth

import (
	"context"
	"errors"
	"net/http"
)

type SAMLService struct {
	cfg      Config
	sessions *SessionManager
}

func NewSAMLService(cfg Config, sessions *SessionManager) (*SAMLService, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.Mode != ModeSAML {
		return nil, errors.New("auth mode must be saml")
	}
	return &SAMLService{cfg: cfg, sessions: sessions}, nil
}

func (s *SAMLService) Authenticate(ctx context.Context, r *http.Request) (Identity, error) {
	if s.sessions == nil {
		return Identity{}, ErrUnauthenticated
	}
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

func (s *SAMLService) LoginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotImplemented, map[string]any{"error": "saml_not_configured"})
	}
}

func (s *SAMLService) CallbackHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotImplemented, map[string]any{"error": "saml_not_configured"})
	}
}

func (s *SAMLService) LogoutHandler() http.HandlerFunc {
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

func (s *SAMLService) SessionHandler() http.HandlerFunc {
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
