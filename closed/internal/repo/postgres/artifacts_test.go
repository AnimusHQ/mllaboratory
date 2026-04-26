package postgres

import (
	"strings"
	"testing"

	"github.com/animus-labs/animus-go/closed/internal/repo"
)

func TestBuildArtifactListQueryRequiresProjectID(t *testing.T) {
	_, _, err := buildArtifactListQuery(repo.ArtifactFilter{})
	if err == nil {
		t.Fatalf("expected error for missing project id")
	}
}

func TestBuildArtifactListQueryIncludesProjectID(t *testing.T) {
	query, args, err := buildArtifactListQuery(repo.ArtifactFilter{ProjectID: "proj-123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) == 0 || args[0] != "proj-123" {
		t.Fatalf("expected project id as first arg, got %v", args)
	}
	if !strings.Contains(query, "project_id = $1") {
		t.Fatalf("expected project_id predicate in query, got %s", query)
	}
}

func TestBuildArtifactListQueryWithKindAndLimit(t *testing.T) {
	query, args, err := buildArtifactListQuery(repo.ArtifactFilter{ProjectID: "proj-123", Kind: "model", Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(args))
	}
	if !strings.Contains(query, "kind = $2") {
		t.Fatalf("expected kind predicate in query, got %s", query)
	}
	if !strings.Contains(query, "LIMIT $3") {
		t.Fatalf("expected limit in query, got %s", query)
	}
}
