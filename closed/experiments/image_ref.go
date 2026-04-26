package main

import (
	"encoding/hex"
	"strings"
)

func parseImageDigestFromRef(ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", false
	}
	if isSHA256Digest(ref) {
		return strings.ToLower(strings.TrimSpace(ref)), true
	}
	at := strings.LastIndex(ref, "@")
	if at <= 0 || at == len(ref)-1 {
		return "", false
	}
	if strings.TrimSpace(ref[:at]) == "" {
		return "", false
	}
	digest := strings.ToLower(strings.TrimSpace(ref[at+1:]))
	if !isSHA256Digest(digest) {
		return "", false
	}
	return digest, true
}

func isSHA256Digest(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if !strings.HasPrefix(value, "sha256:") {
		return false
	}
	hexPart := strings.TrimPrefix(value, "sha256:")
	if len(hexPart) != 64 {
		return false
	}
	_, err := hex.DecodeString(hexPart)
	return err == nil
}
