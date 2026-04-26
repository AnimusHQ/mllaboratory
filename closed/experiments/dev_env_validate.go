package main

import (
	"strings"
	"unicode"

	"github.com/animus-labs/animus-go/closed/internal/domain"
)

func validDevEnvRefType(refType string) bool {
	switch strings.ToLower(strings.TrimSpace(refType)) {
	case domain.DevEnvRefTypeBranch, domain.DevEnvRefTypeTag, domain.DevEnvRefTypeCommit:
		return true
	default:
		return false
	}
}

func validCommitSHA(raw string) bool {
	raw = strings.TrimSpace(raw)
	if len(raw) < 7 || len(raw) > 64 {
		return false
	}
	for _, r := range raw {
		if !unicode.IsDigit(r) && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}
