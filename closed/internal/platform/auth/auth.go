package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/env"
)

type Mode string

const (
	ModeOIDC     Mode = "oidc"
	ModeSAML     Mode = "saml"
	ModeDev      Mode = "dev"
	ModeDisabled Mode = "disabled"
)

var ErrUnauthenticated = errors.New("unauthenticated")

type Config struct {
	Mode Mode

	RolesClaim  string
	GroupsClaim string
	EmailClaim  string

	SessionCookieName     string
	SessionCookieSecure   bool
	SessionCookieMaxAge   time.Duration
	SessionCookieSameSite string
	SessionMaxConcurrent  int
	RBACAllowDirectRoles  bool
	GroupRoleMap          map[string]string

	OIDCIssuerURL    string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCRedirectURL  string
	OIDCScopes       []string

	PublicBaseURL          string
	AllowedReturnToOrigins []string

	DevSubject string
	DevEmail   string
	DevRoles   []string
}

func ConfigFromEnv() (Config, error) {
	modeRaw := strings.ToLower(strings.TrimSpace(env.String("AUTH_MODE", string(ModeOIDC))))
	var mode Mode
	switch modeRaw {
	case string(ModeOIDC):
		mode = ModeOIDC
	case string(ModeSAML):
		mode = ModeSAML
	case string(ModeDev):
		mode = ModeDev
	case string(ModeDisabled):
		mode = ModeDisabled
	default:
		return Config{}, fmt.Errorf("AUTH_MODE must be one of: oidc, dev, disabled (got %q)", modeRaw)
	}

	sessionCookieSecure, err := env.Bool("AUTH_SESSION_COOKIE_SECURE", true)
	if err != nil {
		return Config{}, err
	}
	maxAgeSeconds, err := env.Int("AUTH_SESSION_MAX_AGE_SECONDS", 3600)
	if err != nil {
		return Config{}, err
	}
	sessionMaxConcurrent, err := env.Int("AUTH_SESSION_MAX_CONCURRENT", 5)
	if err != nil {
		return Config{}, err
	}
	rbacAllowDirect, err := env.Bool("AUTH_RBAC_ALLOW_DIRECT_ROLES", true)
	if err != nil {
		return Config{}, err
	}
	groupRoleMap, err := parseGroupRoleMap(env.String("AUTH_GROUP_ROLE_MAP", ""))
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Mode:                   mode,
		RolesClaim:             env.String("AUTH_ROLES_CLAIM", "roles"),
		GroupsClaim:            env.String("AUTH_GROUPS_CLAIM", "groups"),
		EmailClaim:             env.String("AUTH_EMAIL_CLAIM", "email"),
		SessionCookieName:      env.String("AUTH_SESSION_COOKIE_NAME", "animus_session"),
		SessionCookieSecure:    sessionCookieSecure,
		SessionCookieMaxAge:    time.Duration(maxAgeSeconds) * time.Second,
		SessionCookieSameSite:  env.String("AUTH_SESSION_COOKIE_SAMESITE", "Lax"),
		SessionMaxConcurrent:   sessionMaxConcurrent,
		RBACAllowDirectRoles:   rbacAllowDirect,
		GroupRoleMap:           groupRoleMap,
		OIDCIssuerURL:          env.String("OIDC_ISSUER_URL", ""),
		OIDCClientID:           env.String("OIDC_CLIENT_ID", ""),
		OIDCClientSecret:       env.String("OIDC_CLIENT_SECRET", ""),
		OIDCRedirectURL:        env.String("OIDC_REDIRECT_URL", ""),
		OIDCScopes:             parseScopes(env.String("OIDC_SCOPES", "openid profile email")),
		PublicBaseURL:          strings.TrimSpace(env.String("ANIMUS_PUBLIC_BASE_URL", "")),
		AllowedReturnToOrigins: parseCSV(env.String("ANIMUS_ALLOWED_RETURN_TO_ORIGINS", "")),
		DevSubject:             env.String("DEV_AUTH_SUBJECT", "dev-user"),
		DevEmail:               env.String("DEV_AUTH_EMAIL", "dev-user@example.local"),
		DevRoles:               parseCSV(env.String("DEV_AUTH_ROLES", "admin")),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(string(c.Mode)) == "" {
		return errors.New("AUTH_MODE is required")
	}
	if strings.TrimSpace(c.RolesClaim) == "" {
		return errors.New("AUTH_ROLES_CLAIM is required")
	}
	if len(c.GroupRoleMap) > 0 && strings.TrimSpace(c.GroupsClaim) == "" {
		return errors.New("AUTH_GROUPS_CLAIM is required when AUTH_GROUP_ROLE_MAP is set")
	}
	if strings.TrimSpace(c.EmailClaim) == "" {
		return errors.New("AUTH_EMAIL_CLAIM is required")
	}
	if strings.TrimSpace(c.SessionCookieName) == "" {
		return errors.New("AUTH_SESSION_COOKIE_NAME is required")
	}
	if c.SessionCookieMaxAge <= 0 {
		return errors.New("AUTH_SESSION_MAX_AGE_SECONDS must be positive")
	}
	if c.SessionMaxConcurrent < 0 {
		return errors.New("AUTH_SESSION_MAX_CONCURRENT must be >= 0")
	}
	if strings.TrimSpace(c.SessionCookieSameSite) == "" {
		return errors.New("AUTH_SESSION_COOKIE_SAMESITE is required")
	}

	switch c.Mode {
	case ModeOIDC:
		if strings.TrimSpace(c.OIDCIssuerURL) == "" {
			return errors.New("OIDC_ISSUER_URL is required when AUTH_MODE=oidc")
		}
		if strings.TrimSpace(c.OIDCClientID) == "" {
			return errors.New("OIDC_CLIENT_ID is required when AUTH_MODE=oidc")
		}
	case ModeSAML:
		// SAML is optional; handlers may be stubbed until configured.
	case ModeDev:
		if strings.TrimSpace(c.DevSubject) == "" {
			return errors.New("DEV_AUTH_SUBJECT is required when AUTH_MODE=dev")
		}
		if len(c.DevRoles) == 0 {
			return errors.New("DEV_AUTH_ROLES must be non-empty when AUTH_MODE=dev")
		}
	case ModeDisabled:
	default:
		return fmt.Errorf("unsupported auth mode: %q", c.Mode)
	}

	return nil
}

func (c Config) ValidateForLogin() error {
	if c.Mode != ModeOIDC {
		return fmt.Errorf("login requires AUTH_MODE=oidc (got %q)", c.Mode)
	}
	if strings.TrimSpace(c.OIDCClientSecret) == "" {
		return errors.New("OIDC_CLIENT_SECRET is required for login endpoints")
	}
	if strings.TrimSpace(c.OIDCRedirectURL) == "" {
		return errors.New("OIDC_REDIRECT_URL is required for login endpoints")
	}
	return nil
}

func parseScopes(value string) []string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return []string{"openid", "profile", "email"}
	}
	return fields
}

func parseGroupRoleMap(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	entries := strings.Split(raw, ",")
	out := make(map[string]string, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		sep := strings.IndexAny(entry, "=:")
		if sep <= 0 || sep >= len(entry)-1 {
			return nil, fmt.Errorf("invalid group-role mapping: %q", entry)
		}
		group := strings.ToLower(strings.TrimSpace(entry[:sep]))
		role := strings.ToLower(strings.TrimSpace(entry[sep+1:]))
		if group == "" || role == "" {
			return nil, fmt.Errorf("invalid group-role mapping: %q", entry)
		}
		switch role {
		case RoleViewer, RoleEditor, RoleAdmin:
			// ok
		default:
			return nil, fmt.Errorf("invalid group-role mapping role: %q", role)
		}
		out[group] = role
	}
	return out, nil
}

func parseCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		item := strings.ToLower(strings.TrimSpace(part))
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
