package registryverify

import "testing"

func TestBuildDigestRef(t *testing.T) {
	ref, err := BuildDigestRef("ghcr.io/acme/runtime:latest", "sha256:deadbeef")
	if err != nil {
		t.Fatalf("build digest ref: %v", err)
	}
	if want := "ghcr.io/acme/runtime:latest@sha256:deadbeef"; ref != want {
		t.Fatalf("expected %s, got %s", want, ref)
	}
}

func TestBuildDigestRefRejectsNonSha(t *testing.T) {
	if _, err := BuildDigestRef("ghcr.io/acme/runtime:latest", "md5:deadbeef"); err == nil {
		t.Fatal("expected error for non sha256 digest")
	}
}

func TestBuildDigestRefPreservesDigestRef(t *testing.T) {
	ref, err := BuildDigestRef("ghcr.io/acme/runtime@sha256:deadbeef", "sha256:deadbeef")
	if err != nil {
		t.Fatalf("build digest ref: %v", err)
	}
	if want := "ghcr.io/acme/runtime@sha256:deadbeef"; ref != want {
		t.Fatalf("expected %s, got %s", want, ref)
	}
}
