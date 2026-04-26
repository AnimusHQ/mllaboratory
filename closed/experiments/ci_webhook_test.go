package main

import (
	"encoding/base64"
	"testing"
)

func TestVerifyCIWebhookSignature_OK(t *testing.T) {
	secret := "test-secret"
	ts := "1734200000"
	method := "POST"
	body := []byte(`{"run_id":"run-123","provider":"github_actions","context":{"sha":"abc"}}`)

	mac, err := computeCIWebhookMAC(secret, ts, method, body)
	if err != nil {
		t.Fatalf("computeCIWebhookMAC failed: %v", err)
	}
	sig := base64.RawURLEncoding.EncodeToString(mac)

	if err := verifyCIWebhookSignature(secret, ts, method, body, sig); err != nil {
		t.Fatalf("verifyCIWebhookSignature failed: %v", err)
	}
}

func TestVerifyCIWebhookSignature_BadSignature(t *testing.T) {
	secret := "test-secret"
	ts := "1734200000"
	method := "POST"
	body := []byte(`{"run_id":"run-123"}`)

	if err := verifyCIWebhookSignature(secret, ts, method, body, "bad"); err == nil {
		t.Fatalf("expected error")
	}
}
