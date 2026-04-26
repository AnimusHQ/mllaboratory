package postgres

import (
	"strings"
	"testing"
)

func TestModelVersionProvenanceInsertQueriesAreIdempotent(t *testing.T) {
	if !strings.Contains(insertModelVersionArtifactQuery, "ON CONFLICT (project_id, model_version_id, artifact_id) DO NOTHING") {
		t.Fatalf("expected idempotent artifact insert, got query: %s", insertModelVersionArtifactQuery)
	}
	if !strings.Contains(insertModelVersionDatasetQuery, "ON CONFLICT (project_id, model_version_id, dataset_version_id) DO NOTHING") {
		t.Fatalf("expected idempotent dataset insert, got query: %s", insertModelVersionDatasetQuery)
	}
}

func TestModelVersionProvenanceQueriesAreProjectScoped(t *testing.T) {
	queries := []string{insertModelVersionArtifactQuery, insertModelVersionDatasetQuery}
	for _, query := range queries {
		if !strings.Contains(query, "project_id") {
			t.Fatalf("expected project scoping in query: %s", query)
		}
	}
}
