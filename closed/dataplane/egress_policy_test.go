package main

import (
	"errors"
	"testing"

	"github.com/animus-labs/animus-go/closed/internal/domain"
)

func TestValidateEgressPolicyDenyRequiresClassRef(t *testing.T) {
	err := validateEgressPolicy(egressModeDeny, domain.EnvLock{})
	if !errors.Is(err, errEgressPolicyRequired) {
		t.Fatalf("expected policy required error, got %v", err)
	}
}

func TestValidateEgressPolicyDenyWithClassRef(t *testing.T) {
	err := validateEgressPolicy(egressModeDeny, domain.EnvLock{NetworkClassRef: "egress-default"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateEgressPolicyAllow(t *testing.T) {
	err := validateEgressPolicy(egressModeAllow, domain.EnvLock{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
