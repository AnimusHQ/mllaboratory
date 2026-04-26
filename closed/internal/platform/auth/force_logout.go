package auth

import (
	"encoding/json"
	"net/http"
	"strings"
)

type forceLogoutRequest struct {
	SessionID string `json:"session_id,omitempty"`
	Subject   string `json:"subject,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

func ForceLogoutHandler(manager *SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if manager == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "session_unavailable"})
			return
		}
		actor := ""
		if identity, ok := IdentityFromContext(r.Context()); ok {
			actor = strings.TrimSpace(identity.Subject)
		}

		var req forceLogoutRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
			return
		}

		sessionID := strings.TrimSpace(req.SessionID)
		subject := strings.TrimSpace(req.Subject)
		reason := strings.TrimSpace(req.Reason)
		if sessionID == "" && subject == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "session_id_or_subject_required"})
			return
		}

		meta := SessionRequestMeta{
			RequestID: r.Header.Get("X-Request-Id"),
			UserAgent: r.UserAgent(),
			RemoteIP:  ParseRemoteIP(r.RemoteAddr),
			Actor:     actor,
		}

		if sessionID != "" {
			if reason == "" {
				reason = "forced"
			}
			updated, err := manager.RevokeSession(r.Context(), sessionID, "admin", reason, meta)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "force_logout_failed"})
				return
			}
			revoked := 0
			if updated {
				revoked = 1
			}
			writeJSON(w, http.StatusOK, map[string]any{"revoked": revoked})
			return
		}

		if reason == "" {
			reason = "forced"
		}
		count, err := manager.RevokeBySubject(r.Context(), subject, "admin", reason, meta)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "force_logout_failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"revoked": count})
	}
}
