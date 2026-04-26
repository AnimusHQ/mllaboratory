package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIntegritySHA256_Deterministic(t *testing.T) {
	type input struct {
		A string `json:"a"`
		B int    `json:"b"`
	}
	in := input{A: "x", B: 1}

	a, err := integritySHA256(in)
	if err != nil {
		t.Fatalf("integritySHA256() err=%v", err)
	}
	b, err := integritySHA256(in)
	if err != nil {
		t.Fatalf("integritySHA256() err=%v", err)
	}
	if a != b {
		t.Fatalf("hash mismatch: %q vs %q", a, b)
	}
}

func TestSanitizeFilename(t *testing.T) {
	if got := sanitizeFilename(""); got != "dataset.bin" {
		t.Fatalf("sanitizeFilename(\"\")=%q, want dataset.bin", got)
	}
	if got := sanitizeFilename("../evil.txt"); got != "evil.txt" {
		t.Fatalf("sanitizeFilename(\"../evil.txt\")=%q, want evil.txt", got)
	}
	if got := sanitizeFilename("/tmp/data.csv"); got != "data.csv" {
		t.Fatalf("sanitizeFilename(\"/tmp/data.csv\")=%q, want data.csv", got)
	}
}

func TestDecodeJSON_RejectsExtraValue(t *testing.T) {
	req := httptest.NewRequest("POST", "http://example.test/", strings.NewReader("{\"name\":\"a\"} {\"name\":\"b\"}"))
	var dst createDatasetRequest
	if err := decodeJSON(req, &dst); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDecodeJSON_DisallowUnknownFields(t *testing.T) {
	req := httptest.NewRequest("POST", "http://example.test/", strings.NewReader("{\"name\":\"a\",\"extra\":1}"))
	var dst createDatasetRequest
	if err := decodeJSON(req, &dst); err == nil {
		t.Fatalf("expected error")
	}
}
