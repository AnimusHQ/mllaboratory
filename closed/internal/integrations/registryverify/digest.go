package registryverify

import (
	"fmt"
	"strings"
)

func BuildDigestRef(imageRef, digest string) (string, error) {
	ref := strings.TrimSpace(imageRef)
	dig := strings.TrimSpace(digest)
	if ref == "" || dig == "" {
		return "", fmt.Errorf("image ref and digest required")
	}
	if strings.Contains(ref, "@sha256:") {
		return ref, nil
	}
	if !strings.HasPrefix(dig, "sha256:") {
		return "", fmt.Errorf("digest must be sha256")
	}
	return ref + "@" + dig, nil
}

func IsDigestPinned(imageDigestRef string) bool {
	return strings.Contains(strings.TrimSpace(imageDigestRef), "@sha256:")
}
