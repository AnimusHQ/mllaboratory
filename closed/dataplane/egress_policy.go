package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/animus-labs/animus-go/closed/internal/domain"
)

const (
	egressModeAllow = "allow"
	egressModeDeny  = "deny"
)

var errEgressPolicyRequired = errors.New("network_class_ref_required")

func normalizeEgressMode(raw string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	if mode == "" {
		mode = egressModeDeny
	}
	switch mode {
	case egressModeAllow, egressModeDeny:
		return mode, nil
	default:
		return "", fmt.Errorf("unsupported egress mode: %q", raw)
	}
}

func validateEgressPolicy(mode string, envLock domain.EnvLock) error {
	normalized, err := normalizeEgressMode(mode)
	if err != nil {
		return err
	}
	if normalized == egressModeAllow {
		return nil
	}
	if strings.TrimSpace(envLock.NetworkClassRef) == "" {
		return errEgressPolicyRequired
	}
	return nil
}
