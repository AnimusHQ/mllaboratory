package testutil

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func RepoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	current := cwd
	for {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	t.Fatalf("repo root not found from %s", cwd)
	return ""
}

func ApplyMigrations(t *testing.T, db *sql.DB, repoRoot string) {
	t.Helper()
	if db == nil {
		t.Fatalf("db is nil")
	}
	if repoRoot == "" {
		repoRoot = RepoRoot(t)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
  version BIGINT PRIMARY KEY,
  name TEXT NOT NULL,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}

	migrationsDir := filepath.Join(repoRoot, "closed", "migrations")
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		t.Fatalf("list migrations: %v", err)
	}
	sort.Strings(files)
	for _, file := range files {
		base := filepath.Base(file)
		verRaw := strings.SplitN(base, "_", 2)[0]
		if verRaw == "" {
			continue
		}
		ver, err := strconv.Atoi(verRaw)
		if err != nil {
			t.Fatalf("invalid migration version %s: %v", base, err)
		}
		var exists int
		if err := db.QueryRow("SELECT 1 FROM schema_migrations WHERE version = $1 LIMIT 1", ver).Scan(&exists); err == nil && exists == 1 {
			continue
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read migration %s: %v", file, err)
		}
		if _, err := db.Exec(string(raw)); err != nil {
			t.Fatalf("apply migration %s: %v", base, err)
		}
		if _, err := db.Exec("INSERT INTO schema_migrations(version, name) VALUES ($1, $2)", ver, strings.TrimSuffix(base, ".up.sql")); err != nil {
			t.Fatalf("record migration %s: %v", base, err)
		}
	}
}

func RequireEnv(t *testing.T, key string) string {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		t.Fatalf("missing required env %s", key)
	}
	return value
}

func EnsureDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func WriteText(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func ReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func FormatSHA256(value string) string {
	return fmt.Sprintf("%064s", value)
}
