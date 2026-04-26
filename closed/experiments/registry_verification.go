package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/integrations/registryverify"
	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/repo/postgres"
)

const (
	auditImageVerificationRequested = "image.verification.requested"
	auditImageVerified              = "image.verified"
	auditImageVerificationFailed    = "image.verification_failed"
	auditEnvLockCreationBlocked     = "environment.lock.creation_blocked"

	registryBlockReasonUnverified = "REGISTRY_UNVERIFIED"
	registryBlockReasonUnsigned   = "UNSIGNED_DENIED"
)

type imageVerificationStore interface {
	Upsert(ctx context.Context, record registryverify.Record) (registryverify.Record, error)
	List(ctx context.Context, filter postgres.ImageVerificationFilter) ([]registryverify.Record, error)
	GetLatestByImage(ctx context.Context, projectID, imageDigestRef string) (registryverify.Record, error)
}

func (api *experimentsAPI) imageVerificationStore() imageVerificationStore {
	if api == nil {
		return nil
	}
	if api.registryStoreOverride != nil {
		return api.registryStoreOverride
	}
	return postgres.NewImageVerificationStore(api.db)
}

func (api *experimentsAPI) registryProvider(name string) registryverify.Provider {
	if api != nil {
		if api.registryProviders != nil {
			if provider, ok := api.registryProviders[registryverify.NormalizeProvider(name)]; ok {
				return provider
			}
		}
	}
	return registryverify.ProviderForName(name)
}

func (api *experimentsAPI) verifyRegistryImages(ctx context.Context, identity auth.Identity, projectID, lockID string, images []domain.EnvironmentImage, requestID string) (bool, string, error) {
	if len(images) == 0 {
		return true, "", nil
	}
	policy, err := api.registryPolicyResolver.Resolve(ctx, projectID)
	if err != nil {
		return false, "", err
	}
	policy = policy.Normalize()
	store := api.imageVerificationStore()
	if store == nil {
		return false, "", errors.New("image verification store unavailable")
	}
	blocked := false
	blockReason := ""
	for _, image := range images {
		digestRef, err := registryverify.BuildDigestRef(image.Ref, image.Digest)
		if err != nil {
			return false, "", err
		}
		api.appendRegistryAudit(ctx, auditlog.Event{
			OccurredAt:   time.Now().UTC(),
			Actor:        strings.TrimSpace(identity.Subject),
			Action:       auditImageVerificationRequested,
			ResourceType: "image_verification",
			ResourceID:   digestRef,
			RequestID:    requestID,
			Payload: map[string]any{
				"project_id":          projectID,
				"image_digest":        digestRef,
				"policy_mode":         policy.Mode,
				"provider":            policy.Provider,
				"environment_lock_id": lockID,
			},
		})

		record, shouldBlock, reason, err := api.verifyRegistryImage(ctx, policy, projectID, digestRef)
		if err != nil {
			return false, "", err
		}
		if _, err := store.Upsert(ctx, record); err != nil {
			return false, "", err
		}

		action := auditImageVerified
		if record.Status != registryverify.StatusVerified {
			action = auditImageVerificationFailed
		}
		api.appendRegistryAudit(ctx, auditlog.Event{
			OccurredAt:   time.Now().UTC(),
			Actor:        strings.TrimSpace(identity.Subject),
			Action:       action,
			ResourceType: "image_verification",
			ResourceID:   digestRef,
			RequestID:    requestID,
			Payload: map[string]any{
				"project_id":          projectID,
				"image_digest":        digestRef,
				"policy_mode":         record.PolicyMode,
				"provider":            record.Provider,
				"status":              string(record.Status),
				"signed":              record.Signed,
				"verified":            record.Verified,
				"failure_reason":      record.FailureReason,
				"environment_lock_id": lockID,
			},
		})

		if shouldBlock {
			blocked = true
			if blockReason == "" {
				blockReason = reason
			}
		}
	}
	if blocked {
		api.appendRegistryAudit(ctx, auditlog.Event{
			OccurredAt:   time.Now().UTC(),
			Actor:        strings.TrimSpace(identity.Subject),
			Action:       auditEnvLockCreationBlocked,
			ResourceType: "environment_lock",
			ResourceID:   strings.TrimSpace(lockID),
			RequestID:    requestID,
			Payload: map[string]any{
				"project_id":          projectID,
				"environment_lock_id": lockID,
				"reason":              blockReason,
			},
		})
		return false, blockReason, nil
	}
	return true, "", nil
}

func (api *experimentsAPI) verifyRegistryImage(ctx context.Context, policy registryverify.Policy, projectID, imageDigestRef string) (registryverify.Record, bool, string, error) {
	provider := api.registryProvider(policy.Provider)
	result := registryverify.VerificationResult{
		ImageDigestRef: imageDigestRef,
		Provider:       provider.Name(),
		VerifiedAt:     time.Now().UTC(),
	}

	skipped := policy.Mode == registryverify.ModeAllowUnsigned && provider.Name() == registryverify.ProviderNoop
	if skipped {
		result.FailureReason = "verification_skipped"
	} else {
		verifyCtx := ctx
		if api.registryVerifyTimeout > 0 {
			var cancel context.CancelFunc
			verifyCtx, cancel = context.WithTimeout(ctx, api.registryVerifyTimeout)
			defer cancel()
		}
		providerResult, err := provider.VerifyImageSignature(verifyCtx, imageDigestRef, registryverify.VerifyOptions{Policy: policy})
		if err != nil {
			result.FailureReason = "provider_error"
			result.Details = map[string]any{"error": "provider_error"}
		} else {
			result = providerResult
			if strings.TrimSpace(result.Provider) == "" {
				result.Provider = provider.Name()
			}
			if strings.TrimSpace(result.ImageDigestRef) == "" {
				result.ImageDigestRef = imageDigestRef
			}
			if result.VerifiedAt.IsZero() {
				result.VerifiedAt = time.Now().UTC()
			}
		}
	}
	if result.VerifiedAt.IsZero() {
		result.VerifiedAt = time.Now().UTC()
	}
	if !result.Verified && result.FailureReason == "" {
		if !result.Signed {
			result.FailureReason = "unsigned"
		} else {
			result.FailureReason = "verification_failed"
		}
	}

	status := registryverify.StatusFailed
	if result.Verified {
		status = registryverify.StatusVerified
	} else if policy.Mode == registryverify.ModeAllowUnsigned && !result.Signed && isUnsignedFailure(result.FailureReason) {
		status = registryverify.StatusSkipped
	}

	shouldBlock := false
	blockReason := ""
	if policy.Mode == registryverify.ModeDenyUnsigned && !result.Verified {
		shouldBlock = true
		if !result.Signed {
			blockReason = registryBlockReasonUnsigned
		} else {
			blockReason = registryBlockReasonUnverified
		}
	}

	verifiedAt := result.VerifiedAt
	record := registryverify.Record{
		ProjectID:      projectID,
		ImageDigestRef: imageDigestRef,
		PolicyMode:     policy.Mode,
		Provider:       result.Provider,
		Status:         status,
		Signed:         result.Signed,
		Verified:       result.Verified,
		FailureReason:  result.FailureReason,
		Details:        registryverify.SanitizeDetails(result.Details),
		CreatedAt:      time.Now().UTC(),
		VerifiedAt:     &verifiedAt,
	}
	return record, shouldBlock, blockReason, nil
}

func (api *experimentsAPI) appendRegistryAudit(ctx context.Context, event auditlog.Event) {
	if api == nil || api.db == nil {
		return
	}
	_, _ = auditlog.Insert(ctx, api.db, event)
}

func isUnsignedFailure(reason string) bool {
	switch strings.TrimSpace(reason) {
	case "unsigned", "provider_unconfigured", "verification_skipped":
		return true
	default:
		return false
	}
}
