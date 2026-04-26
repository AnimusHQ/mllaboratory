package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const runTokenPrefix = "animus_run_v1"

var (
	ErrRunTokenInvalid = errors.New("run token is invalid")
	ErrRunTokenExpired = errors.New("run token is expired")
)

type RunTokenClaims struct {
	RunID            string `json:"run_id"`
	DatasetVersionID string `json:"dataset_version_id,omitempty"`
	IssuedAtUnix     int64  `json:"iat"`
	ExpiresAtUnix    int64  `json:"exp"`
}

func RunTokenSubject(claims RunTokenClaims) string {
	subject := "run:" + strings.TrimSpace(claims.RunID)
	if strings.TrimSpace(claims.DatasetVersionID) != "" {
		subject += ":dv:" + strings.TrimSpace(claims.DatasetVersionID)
	}
	return subject
}

func ParseRunTokenSubject(subject string) (runID string, datasetVersionID string, ok bool) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "", "", false
	}
	if !strings.HasPrefix(subject, "run:") {
		return "", "", false
	}

	rest := strings.TrimPrefix(subject, "run:")
	parts := strings.Split(rest, ":dv:")
	if len(parts) == 0 {
		return "", "", false
	}
	runID = strings.TrimSpace(parts[0])
	if runID == "" {
		return "", "", false
	}
	if len(parts) == 1 {
		return runID, "", true
	}
	if len(parts) != 2 {
		return "", "", false
	}
	datasetVersionID = strings.TrimSpace(parts[1])
	if datasetVersionID == "" {
		return "", "", false
	}
	return runID, datasetVersionID, true
}

func GenerateRunToken(secret string, claims RunTokenClaims, now time.Time) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", errors.New("secret is required")
	}
	claims.RunID = strings.TrimSpace(claims.RunID)
	claims.DatasetVersionID = strings.TrimSpace(claims.DatasetVersionID)
	if claims.RunID == "" {
		return "", errors.New("run_id is required")
	}

	if now.IsZero() {
		now = time.Now().UTC()
	}
	if claims.IssuedAtUnix == 0 {
		claims.IssuedAtUnix = now.UTC().Unix()
	}
	if claims.ExpiresAtUnix == 0 {
		return "", errors.New("exp is required")
	}
	if claims.ExpiresAtUnix <= now.UTC().Unix() {
		return "", errors.New("exp must be in the future")
	}

	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)
	sigB64, err := computeRunTokenSignature(secret, payloadB64)
	if err != nil {
		return "", err
	}
	return strings.Join([]string{runTokenPrefix, payloadB64, sigB64}, "."), nil
}

func VerifyRunToken(secret string, token string, now time.Time) (RunTokenClaims, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return RunTokenClaims{}, errors.New("secret is required")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return RunTokenClaims{}, ErrRunTokenInvalid
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return RunTokenClaims{}, ErrRunTokenInvalid
	}
	if parts[0] != runTokenPrefix {
		return RunTokenClaims{}, ErrRunTokenInvalid
	}
	payloadB64 := strings.TrimSpace(parts[1])
	sigB64 := strings.TrimSpace(parts[2])
	if payloadB64 == "" || sigB64 == "" {
		return RunTokenClaims{}, ErrRunTokenInvalid
	}

	expectedB64, err := computeRunTokenSignature(secret, payloadB64)
	if err != nil {
		return RunTokenClaims{}, err
	}
	expectedSig, err := base64.RawURLEncoding.DecodeString(expectedB64)
	if err != nil {
		return RunTokenClaims{}, ErrRunTokenInvalid
	}
	gotSig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return RunTokenClaims{}, ErrRunTokenInvalid
	}
	if !hmac.Equal(expectedSig, gotSig) {
		return RunTokenClaims{}, ErrRunTokenInvalid
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return RunTokenClaims{}, ErrRunTokenInvalid
	}
	var claims RunTokenClaims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return RunTokenClaims{}, ErrRunTokenInvalid
	}
	claims.RunID = strings.TrimSpace(claims.RunID)
	claims.DatasetVersionID = strings.TrimSpace(claims.DatasetVersionID)
	if claims.RunID == "" || claims.ExpiresAtUnix == 0 {
		return RunTokenClaims{}, ErrRunTokenInvalid
	}

	if now.IsZero() {
		now = time.Now().UTC()
	}
	if claims.ExpiresAtUnix <= now.UTC().Unix() {
		return RunTokenClaims{}, ErrRunTokenExpired
	}

	return claims, nil
}

func computeRunTokenSignature(secret string, payloadB64 string) (string, error) {
	payloadB64 = strings.TrimSpace(payloadB64)
	if payloadB64 == "" {
		return "", errors.New("payload is required")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte("animus-run-token-v1\n")); err != nil {
		return "", err
	}
	if _, err := mac.Write([]byte(payloadB64)); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
