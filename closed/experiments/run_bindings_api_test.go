package main

import (
	"testing"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
)

func TestAssemblePolicySnapshotUsesEnvLockRefs(t *testing.T) {
	now := time.Date(2026, 2, 2, 12, 0, 0, 0, time.UTC)
	envLock := domain.EnvLock{
		EnvironmentDefinitionID: "envdef-1",
		NetworkClassRef:         "net-class",
		SecretAccessClassRef:    "secret-class",
	}
	identity := auth.Identity{
		Subject: "actor",
		Roles:   []string{"admin"},
	}
	policies := []policyVersionRecord{
		{
			PolicyID:        "pol-1",
			PolicyName:      "Policy One",
			PolicyVersionID: "polv-1",
			Version:         1,
			SpecSHA256:      "sha",
			Status:          "active",
		},
	}

	snapshot := assemblePolicySnapshot("proj-1", identity, envLock, policies, now)

	if snapshot.Templates.Mode != "locked" || len(snapshot.Templates.AllowedTemplateIDs) != 1 || snapshot.Templates.AllowedTemplateIDs[0] != "envdef-1" {
		t.Fatalf("unexpected templates snapshot: %+v", snapshot.Templates)
	}
	if snapshot.Network.Mode != "class_ref" || snapshot.Network.ClassRef != "net-class" {
		t.Fatalf("unexpected network snapshot: %+v", snapshot.Network)
	}
	if snapshot.Secrets.Mode != "class_ref" || snapshot.Secrets.ClassRef != "secret-class" {
		t.Fatalf("unexpected secrets snapshot: %+v", snapshot.Secrets)
	}
	if snapshot.RBAC.Decision != "allow" {
		t.Fatalf("unexpected rbac decision: %s", snapshot.RBAC.Decision)
	}
	if snapshot.CapturedAt != now {
		t.Fatalf("unexpected capturedAt: %s", snapshot.CapturedAt)
	}
	if snapshot.SnapshotVersion != policySnapshotVersion {
		t.Fatalf("unexpected snapshot version: %s", snapshot.SnapshotVersion)
	}
}
