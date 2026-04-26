package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	HeaderSubject = "X-Animus-Subject"
	HeaderEmail   = "X-Animus-Email"
	HeaderRoles   = "X-Animus-Roles"

	HeaderInternalAuthTimestamp = "X-Animus-Auth-Ts"
	HeaderInternalAuthSignature = "X-Animus-Auth-Sig"
)

func ComputeInternalAuthSignature(secret string, ts string, method string, path string, requestID string, subject string, email string, roles string) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", errors.New("internal auth secret is required")
	}
	if strings.TrimSpace(ts) == "" {
		return "", errors.New("timestamp is required")
	}
	msg := internalAuthCanonical(ts, method, path, requestID, subject, email, roles)
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(msg)); err != nil {
		return "", fmt.Errorf("hmac: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func VerifyInternalAuthSignature(secret string, ts string, method string, path string, requestID string, subject string, email string, roles string, signature string) error {
	expected, err := ComputeInternalAuthSignature(secret, ts, method, path, requestID, subject, email, roles)
	if err != nil {
		return err
	}
	signature = strings.TrimSpace(signature)
	if signature == "" {
		return errors.New("signature is required")
	}
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return errors.New("invalid signature")
	}
	return nil
}

func VerifyInternalAuthTimestamp(ts string, now time.Time, maxSkew time.Duration) error {
	ts = strings.TrimSpace(ts)
	if ts == "" {
		return errors.New("timestamp is required")
	}
	parsed, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}
	if maxSkew <= 0 {
		return nil
	}

	tsTime := time.Unix(parsed, 0).UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if tsTime.After(now.Add(maxSkew)) || tsTime.Before(now.Add(-maxSkew)) {
		return errors.New("timestamp outside allowed skew")
	}
	return nil
}

func internalAuthCanonical(ts string, method string, path string, requestID string, subject string, email string, roles string) string {
	parts := []string{
		strings.TrimSpace(ts),
		strings.ToUpper(strings.TrimSpace(method)),
		strings.TrimSpace(path),
		strings.TrimSpace(requestID),
		strings.TrimSpace(subject),
		strings.TrimSpace(email),
		strings.TrimSpace(roles),
	}
	return strings.Join(parts, "\n")
}
