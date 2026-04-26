//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/repo/postgres"
	objectstore "github.com/animus-labs/animus-go/closed/internal/storage/objectstore"
	"github.com/animus-labs/animus-go/closed/internal/testutil"
)

func TestIntegration_PostgresAndMinio(t *testing.T) {
	cfg := testutil.RequireIntegrationConfig(t)
	seeded := testutil.NewRand(t)
	repoRoot := testutil.RepoRoot(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	db := testutil.OpenIntegrationDB(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()
	testutil.ApplyMigrations(t, db, repoRoot)

	projectID := fmt.Sprintf("it-proj-%d", seeded.Intn(100000))
	store := postgres.NewProjectStore(db)
	project := domain.Project{
		ID:              projectID,
		Name:            "integration-project",
		Description:     "integration",
		Metadata:        domain.Metadata{"source": "integration"},
		CreatedAt:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		CreatedBy:       "integration-test",
		IntegritySHA256: strings.Repeat("a", 64),
	}
	if err := store.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	loaded, err := store.Get(ctx, projectID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if loaded.ID != projectID {
		t.Fatalf("project id mismatch: %s", loaded.ID)
	}

	client := testutil.MinioClientFromConfig(t, cfg)
	storeObj, err := objectstore.NewMinioStoreWithClient(client)
	if err != nil {
		t.Fatalf("minio store: %v", err)
	}

	payload := []byte("integration-object")
	key := fmt.Sprintf("%s/objects/%d", projectID, seeded.Intn(100000))
	if err := storeObj.Put(ctx, cfg.BucketArtifacts, key, bytes.NewReader(payload), int64(len(payload)), "text/plain"); err != nil {
		t.Fatalf("put object: %v", err)
	}

	reader, info, err := storeObj.Get(ctx, cfg.BucketArtifacts, key)
	if err != nil {
		t.Fatalf("get object: %v", err)
	}
	defer func() { _ = reader.Close() }()
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read object: %v", err)
	}
	if string(body) != string(payload) {
		t.Fatalf("payload mismatch: %s", string(body))
	}
	if info.Key == "" || info.Size == 0 {
		t.Fatalf("object info missing")
	}

	if err := storeObj.Delete(ctx, cfg.BucketArtifacts, key); err != nil {
		t.Fatalf("delete object: %v", err)
	}
}
