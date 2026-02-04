package redaction

import (
	"strings"
	"testing"
)

func TestRedactStringPatterns(t *testing.T) {
	input := "Bearer abcdef12345"
	out := RedactString(input)
	if out == input {
		t.Fatalf("expected redaction for bearer token")
	}
	if out != "[REDACTED]" {
		t.Fatalf("unexpected redaction output: %q", out)
	}
}

func TestRedactMetadataKeys(t *testing.T) {
	meta := map[string]any{
		"password": "s3cr3t",
		"nested": map[string]any{
			"api_key": "abcd",
		},
		"safe": "value",
	}
	out := RedactMetadata(meta)
	if out["password"] != "[REDACTED]" {
		t.Fatalf("expected password redacted")
	}
	nested := out["nested"].(map[string]any)
	if nested["api_key"] != "[REDACTED]" {
		t.Fatalf("expected nested api_key redacted")
	}
	if out["safe"] != "value" {
		t.Fatalf("expected safe value preserved")
	}
}

func TestRedactJSON(t *testing.T) {
	raw := []byte(`{"api_key":"abc","value":"ok"}`)
	out := string(RedactJSON(raw))
	if out == string(raw) {
		t.Fatalf("expected redacted json")
	}
	if out == "" || out == "null" {
		t.Fatalf("unexpected redacted output")
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("expected redacted token in output: %s", out)
	}
}

func TestRedactMapString(t *testing.T) {
	input := map[string]string{
		"api_key": "abc123",
		"safe":    "Bearer token-value",
	}
	out := RedactMapString(input)
	if out["api_key"] != "[REDACTED]" {
		t.Fatalf("expected api_key redacted")
	}
	if out["safe"] != "[REDACTED]" {
		t.Fatalf("expected value pattern redacted")
	}
}
