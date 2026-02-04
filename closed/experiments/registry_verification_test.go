package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/integrations/registryverify"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/repo/postgres"
)

type stubRegistryProvider struct {
	result registryverify.VerificationResult
	err    error
}

func (s stubRegistryProvider) Name() string {
	return "stub"
}

func (s stubRegistryProvider) VerifyImageSignature(ctx context.Context, imageDigestRef string, opts registryverify.VerifyOptions) (registryverify.VerificationResult, error) {
	if s.err != nil {
		return registryverify.VerificationResult{}, s.err
	}
	return s.result, nil
}

type stubImageVerificationStore struct {
	records []registryverify.Record
}

func (s *stubImageVerificationStore) Upsert(ctx context.Context, record registryverify.Record) (registryverify.Record, error) {
	s.records = append(s.records, record)
	return record, nil
}

func (s *stubImageVerificationStore) List(ctx context.Context, filter postgres.ImageVerificationFilter) ([]registryverify.Record, error) {
	return s.records, nil
}

func (s *stubImageVerificationStore) GetLatestByImage(ctx context.Context, projectID, imageDigestRef string) (registryverify.Record, error) {
	if len(s.records) == 0 {
		return registryverify.Record{}, errors.New("not found")
	}
	return s.records[len(s.records)-1], nil
}

func TestRegistryVerifyAllowUnsignedUnsigned(t *testing.T) {
	store := &stubImageVerificationStore{}
	provider := stubRegistryProvider{result: registryverify.VerificationResult{Verified: false, Signed: false, Provider: "stub", VerifiedAt: time.Now().UTC(), FailureReason: "unsigned"}}
	api := &experimentsAPI{
		registryPolicyResolver: registryverify.PolicyResolver{Default: registryverify.Policy{Mode: registryverify.ModeAllowUnsigned, Provider: "stub"}},
		registryProviders:      map[string]registryverify.Provider{"stub": provider},
		registryStoreOverride:  store,
	}
	images := []domain.EnvironmentImage{{Name: "runtime", Ref: "ghcr.io/acme/runtime:latest", Digest: "sha256:aaaaaaaa"}}
	allowed, reason, err := api.verifyRegistryImages(context.Background(), auth.Identity{Subject: "tester"}, "proj-1", "lock-1", images, "req-1")
	if err != nil {
		t.Fatalf("verify registry images: %v", err)
	}
	if !allowed || reason != "" {
		t.Fatalf("expected allowed, got allowed=%v reason=%q", allowed, reason)
	}
	if len(store.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(store.records))
	}
	if store.records[0].Status != registryverify.StatusSkipped {
		t.Fatalf("expected skipped status, got %s", store.records[0].Status)
	}
}

func TestRegistryVerifyOnlyProviderFailure(t *testing.T) {
	store := &stubImageVerificationStore{}
	provider := stubRegistryProvider{err: errors.New("boom")}
	api := &experimentsAPI{
		registryPolicyResolver: registryverify.PolicyResolver{Default: registryverify.Policy{Mode: registryverify.ModeVerifyOnly, Provider: "stub"}},
		registryProviders:      map[string]registryverify.Provider{"stub": provider},
		registryStoreOverride:  store,
	}
	images := []domain.EnvironmentImage{{Name: "runtime", Ref: "ghcr.io/acme/runtime:latest", Digest: "sha256:bbbbbbbb"}}
	allowed, reason, err := api.verifyRegistryImages(context.Background(), auth.Identity{Subject: "tester"}, "proj-1", "lock-2", images, "req-2")
	if err != nil {
		t.Fatalf("verify registry images: %v", err)
	}
	if !allowed || reason != "" {
		t.Fatalf("expected allowed, got allowed=%v reason=%q", allowed, reason)
	}
	if len(store.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(store.records))
	}
	if store.records[0].Status != registryverify.StatusFailed {
		t.Fatalf("expected failed status, got %s", store.records[0].Status)
	}
}

func TestRegistryDenyUnsignedBlocks(t *testing.T) {
	store := &stubImageVerificationStore{}
	provider := stubRegistryProvider{result: registryverify.VerificationResult{Verified: false, Signed: false, Provider: "stub", VerifiedAt: time.Now().UTC(), FailureReason: "unsigned"}}
	api := &experimentsAPI{
		registryPolicyResolver: registryverify.PolicyResolver{Default: registryverify.Policy{Mode: registryverify.ModeDenyUnsigned, Provider: "stub"}},
		registryProviders:      map[string]registryverify.Provider{"stub": provider},
		registryStoreOverride:  store,
	}
	images := []domain.EnvironmentImage{{Name: "runtime", Ref: "ghcr.io/acme/runtime:latest", Digest: "sha256:cccccccc"}}
	allowed, reason, err := api.verifyRegistryImages(context.Background(), auth.Identity{Subject: "tester"}, "proj-1", "lock-3", images, "req-3")
	if err != nil {
		t.Fatalf("verify registry images: %v", err)
	}
	if allowed || reason != registryBlockReasonUnsigned {
		t.Fatalf("expected blocked unsigned, got allowed=%v reason=%q", allowed, reason)
	}
	if len(store.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(store.records))
	}
	if store.records[0].Status != registryverify.StatusFailed {
		t.Fatalf("expected failed status, got %s", store.records[0].Status)
	}
}
