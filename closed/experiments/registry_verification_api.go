package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/integrations/registryverify"
	"github.com/animus-labs/animus-go/closed/internal/repo/postgres"
)

type registryVerificationResponse struct {
	ProjectID      string          `json:"projectId"`
	ImageDigestRef string          `json:"imageDigestRef"`
	PolicyMode     string          `json:"policyMode"`
	Provider       string          `json:"provider"`
	Status         string          `json:"status"`
	Signed         bool            `json:"signed"`
	Verified       bool            `json:"verified"`
	FailureReason  string          `json:"failureReason,omitempty"`
	Details        json.RawMessage `json:"details,omitempty"`
	CreatedAt      string          `json:"createdAt"`
	VerifiedAt     string          `json:"verifiedAt,omitempty"`
}

type registryVerificationListResponse struct {
	Items []registryVerificationResponse `json:"items"`
}

func (api *experimentsAPI) handleListRegistryVerifications(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.PathValue("project_id"))
	if projectID == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	store := api.imageVerificationStore()
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	records, err := store.List(r.Context(), postgres.ImageVerificationFilter{
		ProjectID:      projectID,
		ImageDigestRef: strings.TrimSpace(r.URL.Query().Get("image_digest_ref")),
		Limit:          limit,
	})
	if err != nil {
		api.writeRepoError(w, r, err)
		return
	}
	items := make([]registryVerificationResponse, 0, len(records))
	for _, record := range records {
		items = append(items, toRegistryVerificationResponse(record))
	}
	api.writeJSON(w, http.StatusOK, registryVerificationListResponse{Items: items})
}

func toRegistryVerificationResponse(record registryverify.Record) registryVerificationResponse {
	response := registryVerificationResponse{
		ProjectID:      record.ProjectID,
		ImageDigestRef: record.ImageDigestRef,
		PolicyMode:     record.PolicyMode,
		Provider:       record.Provider,
		Status:         string(record.Status),
		Signed:         record.Signed,
		Verified:       record.Verified,
		FailureReason:  record.FailureReason,
		Details:        record.Details,
		CreatedAt:      record.CreatedAt.UTC().Format(time.RFC3339),
	}
	if record.VerifiedAt != nil && !record.VerifiedAt.IsZero() {
		response.VerifiedAt = record.VerifiedAt.UTC().Format(time.RFC3339)
	}
	return response
}
