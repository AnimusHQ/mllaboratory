package auditexport

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

func signPayload(secret string, payload []byte) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", errors.New("signing secret is required")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	sum := mac.Sum(nil)
	return "sha256=" + hex.EncodeToString(sum), nil
}
