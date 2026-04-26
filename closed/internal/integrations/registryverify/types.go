package registryverify

import (
	"strings"
	"time"
)

type Policy struct {
	Mode     string
	Provider string
}

type VerifyOptions struct {
	Policy Policy
}

type VerificationResult struct {
	ImageDigestRef string
	Verified       bool
	Signed         bool
	Provider       string
	VerifiedAt     time.Time
	FailureReason  string
	Details        map[string]any
}

type Status string

const (
	StatusVerified Status = "VERIFIED"
	StatusFailed   Status = "FAILED"
	StatusSkipped  Status = "SKIPPED"
)

const (
	ModeAllowUnsigned = "allow_unsigned"
	ModeDenyUnsigned  = "deny_unsigned"
	ModeVerifyOnly    = "verify_only"
)

const (
	ProviderNoop       = "noop"
	ProviderCosignStub = "cosign_stub"
)

func (p Policy) Normalize() Policy {
	return Policy{
		Mode:     normalizeMode(p.Mode),
		Provider: normalizeProvider(p.Provider),
	}
}

func normalizeMode(mode string) string {
	switch normalize(mode) {
	case ModeAllowUnsigned:
		return ModeAllowUnsigned
	case ModeDenyUnsigned:
		return ModeDenyUnsigned
	case ModeVerifyOnly:
		return ModeVerifyOnly
	default:
		return ModeAllowUnsigned
	}
}

func normalizeProvider(provider string) string {
	value := normalize(provider)
	if value == "" {
		return ProviderNoop
	}
	return value
}

func NormalizeProvider(provider string) string {
	return normalizeProvider(provider)
}

func normalize(value string) string {
	out := strings.ToLower(strings.TrimSpace(value))
	return out
}
