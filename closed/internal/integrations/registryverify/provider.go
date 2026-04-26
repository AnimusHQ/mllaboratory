package registryverify

import (
	"context"
	"errors"
	"strings"
	"time"
)

type Provider interface {
	Name() string
	VerifyImageSignature(ctx context.Context, imageDigestRef string, opts VerifyOptions) (VerificationResult, error)
}

type NoopProvider struct{}

func (NoopProvider) Name() string {
	return ProviderNoop
}

func (NoopProvider) VerifyImageSignature(ctx context.Context, imageDigestRef string, opts VerifyOptions) (VerificationResult, error) {
	return VerificationResult{
		ImageDigestRef: imageDigestRef,
		Verified:       false,
		Signed:         false,
		Provider:       ProviderNoop,
		VerifiedAt:     time.Now().UTC(),
		FailureReason:  "provider_unconfigured",
	}, nil
}

type CosignStubProvider struct{}

func (CosignStubProvider) Name() string {
	return ProviderCosignStub
}

func (CosignStubProvider) VerifyImageSignature(ctx context.Context, imageDigestRef string, opts VerifyOptions) (VerificationResult, error) {
	if !IsDigestPinned(imageDigestRef) {
		return VerificationResult{ImageDigestRef: imageDigestRef, Verified: false, Signed: false, Provider: ProviderCosignStub, VerifiedAt: time.Now().UTC(), FailureReason: "invalid_digest_ref"}, nil
	}
	if strings.TrimSpace(imageDigestRef) == "" {
		return VerificationResult{}, errors.New("image digest ref required")
	}
	return VerificationResult{
		ImageDigestRef: imageDigestRef,
		Verified:       true,
		Signed:         true,
		Provider:       ProviderCosignStub,
		VerifiedAt:     time.Now().UTC(),
	}, nil
}

func ProviderForName(name string) Provider {
	switch normalizeProvider(name) {
	case ProviderCosignStub:
		return CosignStubProvider{}
	case ProviderNoop:
		return NoopProvider{}
	default:
		return NoopProvider{}
	}
}
