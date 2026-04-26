package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

type DB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func normalizeTime(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t.UTC()
}

func encodeMetadata(meta domain.Metadata) ([]byte, error) {
	if meta == nil {
		meta = domain.Metadata{}
	}
	return json.Marshal(meta)
}

func decodeMetadata(raw []byte) (domain.Metadata, error) {
	if len(raw) == 0 {
		return domain.Metadata{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return domain.Metadata(out), nil
}

func requireIntegrity(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("integrity sha256 is required")
	}
	return nil
}

func handleNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return repo.ErrNotFound
	}
	return err
}
