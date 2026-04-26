package main

import "testing"

func TestParseImageDigestFromRef_OK(t *testing.T) {
	ref := "ghcr.io/acme/train:latest@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	digest, ok := parseImageDigestFromRef(ref)
	if !ok {
		t.Fatalf("expected ok")
	}
	if digest != "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" {
		t.Fatalf("unexpected digest: %q", digest)
	}
}

func TestParseImageDigestFromRef_AcceptsRawDigest(t *testing.T) {
	ref := "SHA256:0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF"
	digest, ok := parseImageDigestFromRef(ref)
	if !ok {
		t.Fatalf("expected ok")
	}
	if digest != "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" {
		t.Fatalf("unexpected digest: %q", digest)
	}
}

func TestParseImageDigestFromRef_RejectsUnpinned(t *testing.T) {
	_, ok := parseImageDigestFromRef("ghcr.io/acme/train:latest")
	if ok {
		t.Fatalf("expected not ok")
	}
}

func TestIsSHA256Digest_OK(t *testing.T) {
	if !isSHA256Digest("sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef") {
		t.Fatalf("expected ok")
	}
}

func TestIsSHA256Digest_RejectsInvalid(t *testing.T) {
	cases := []string{
		"",
		"sha256:",
		"sha256:abc",
		"sha256:0123456789abcdef",
		"md5:0123456789abcdef0123456789abcdef",
	}
	for _, tc := range cases {
		if isSHA256Digest(tc) {
			t.Fatalf("expected false for %q", tc)
		}
	}
}
