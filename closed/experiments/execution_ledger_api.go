package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type executionLedgerRecord struct {
	LedgerID      string          `json:"ledger_id"`
	RunID         string          `json:"run_id"`
	ExecutionID   string          `json:"execution_id"`
	ExecutionHash string          `json:"execution_hash"`
	Entry         json.RawMessage `json:"entry"`
	EntrySHA256   string          `json:"entry_sha256"`
	ReplayBundle  json.RawMessage `json:"replay_bundle"`
	CreatedAt     time.Time       `json:"created_at"`
	CreatedBy     string          `json:"created_by"`
}

type executionLedgerExportResponse struct {
	Entries  []executionLedgerRecord `json:"entries"`
	Checksum string                  `json:"checksum"`
}

func (api *experimentsAPI) handleListExecutionLedger(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(r.URL.Query().Get("run_id"))
	limit := clampInt(parseIntQuery(r, "limit", 100), 1, 500)

	fromTime, fromOk, err := parseTimeQuery(r, "from")
	if err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_from")
		return
	}
	toTime, toOk, err := parseTimeQuery(r, "to")
	if err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_to")
		return
	}

	clauses := []string{}
	args := []any{}

	if runID != "" {
		clauses = append(clauses, "run_id = $"+strconv.Itoa(len(args)+1))
		args = append(args, runID)
	}
	if fromOk {
		clauses = append(clauses, "created_at >= $"+strconv.Itoa(len(args)+1))
		args = append(args, fromTime.UTC())
	}
	if toOk {
		clauses = append(clauses, "created_at <= $"+strconv.Itoa(len(args)+1))
		args = append(args, toTime.UTC())
	}

	query := `SELECT ledger_id,
				run_id,
				execution_id,
				execution_hash,
				entry,
				entry_sha256,
				replay_bundle,
				created_at,
				created_by
		 FROM execution_ledger_entries`
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(len(args)+1)
	args = append(args, limit)

	rows, err := api.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer rows.Close()

	entries := []executionLedgerRecord{}
	for rows.Next() {
		var (
			entry    executionLedgerRecord
			entryRaw []byte
			replay   []byte
		)
		if err := rows.Scan(
			&entry.LedgerID,
			&entry.RunID,
			&entry.ExecutionID,
			&entry.ExecutionHash,
			&entryRaw,
			&entry.EntrySHA256,
			&replay,
			&entry.CreatedAt,
			&entry.CreatedBy,
		); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		entry.Entry = normalizeJSON(entryRaw)
		entry.ReplayBundle = normalizeJSON(replay)
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	checksum, err := executionLedgerChecksum(entries)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, executionLedgerExportResponse{
		Entries:  entries,
		Checksum: checksum,
	})
}

func (api *experimentsAPI) handleGetExecutionLedger(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(r.PathValue("run_id"))
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}

	var (
		entry    executionLedgerRecord
		entryRaw []byte
		replay   []byte
	)
	err := api.db.QueryRowContext(
		r.Context(),
		`SELECT ledger_id,
				run_id,
				execution_id,
				execution_hash,
				entry,
				entry_sha256,
				replay_bundle,
				created_at,
				created_by
		 FROM execution_ledger_entries
		 WHERE run_id = $1`,
		runID,
	).Scan(
		&entry.LedgerID,
		&entry.RunID,
		&entry.ExecutionID,
		&entry.ExecutionHash,
		&entryRaw,
		&entry.EntrySHA256,
		&replay,
		&entry.CreatedAt,
		&entry.CreatedBy,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	entry.Entry = normalizeJSON(entryRaw)
	entry.ReplayBundle = normalizeJSON(replay)

	checksum, err := executionLedgerChecksum([]executionLedgerRecord{entry})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, executionLedgerExportResponse{
		Entries:  []executionLedgerRecord{entry},
		Checksum: checksum,
	})
}

func executionLedgerChecksum(entries []executionLedgerRecord) (string, error) {
	blob, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	return sha256HexBytes(blob), nil
}

func parseTimeQuery(r *http.Request, key string) (time.Time, bool, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return time.Time{}, false, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			return time.Time{}, false, err
		}
	}
	return parsed.UTC(), true, nil
}
