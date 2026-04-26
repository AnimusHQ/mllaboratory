package auth

import (
	"testing"
	"time"
)

func TestRunToken_RoundTrip(t *testing.T) {
	secret := "test-secret"
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)

	token, err := GenerateRunToken(secret, RunTokenClaims{
		RunID:            "run-123",
		DatasetVersionID: "dv-456",
		ExpiresAtUnix:    now.Add(30 * time.Minute).Unix(),
	}, now)
	if err != nil {
		t.Fatalf("GenerateRunToken: %v", err)
	}

	claims, err := VerifyRunToken(secret, token, now.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("VerifyRunToken: %v", err)
	}
	if claims.RunID != "run-123" {
		t.Fatalf("RunID=%q, want %q", claims.RunID, "run-123")
	}
	if claims.DatasetVersionID != "dv-456" {
		t.Fatalf("DatasetVersionID=%q, want %q", claims.DatasetVersionID, "dv-456")
	}
	if claims.IssuedAtUnix != now.Unix() {
		t.Fatalf("IssuedAtUnix=%d, want %d", claims.IssuedAtUnix, now.Unix())
	}
}

func TestRunToken_Expired(t *testing.T) {
	secret := "test-secret"
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)

	token, err := GenerateRunToken(secret, RunTokenClaims{
		RunID:         "run-123",
		ExpiresAtUnix: now.Add(1 * time.Minute).Unix(),
	}, now)
	if err != nil {
		t.Fatalf("GenerateRunToken: %v", err)
	}

	_, err = VerifyRunToken(secret, token, now.Add(2*time.Minute))
	if err == nil {
		t.Fatalf("VerifyRunToken: expected error")
	}
	if err != ErrRunTokenExpired {
		t.Fatalf("VerifyRunToken error=%v, want %v", err, ErrRunTokenExpired)
	}
}

func TestRunTokenSubject_Parse(t *testing.T) {
	subject := RunTokenSubject(RunTokenClaims{RunID: "run-123", DatasetVersionID: "dv-456"})
	runID, datasetVersionID, ok := ParseRunTokenSubject(subject)
	if !ok {
		t.Fatalf("ParseRunTokenSubject ok=false")
	}
	if runID != "run-123" {
		t.Fatalf("runID=%q, want %q", runID, "run-123")
	}
	if datasetVersionID != "dv-456" {
		t.Fatalf("datasetVersionID=%q, want %q", datasetVersionID, "dv-456")
	}
}
