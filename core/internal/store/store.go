// Package store handles per-session persistence using a JSON meta file and a
// SQLite database for message history.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const metaFile = "meta.json"
const dbFile = "history.db"

// SessionStore manages persistence for one agent session.
// It writes into a dedicated directory (typically <workDir>/.biene/).
type SessionStore struct {
	dir string
	db  *sql.DB
}

// Open opens (or creates) a SessionStore in dir.
// The directory is created if it doesn't exist.
func Open(dir string) (*SessionStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("store: mkdir %s: %w", dir, err)
	}

	db, err := sql.Open("sqlite", filepath.Join(dir, dbFile))
	if err != nil {
		return nil, fmt.Errorf("store: open db: %w", err)
	}

	// Single-writer model is sufficient; WAL improves concurrent reads.
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: wal: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	return &SessionStore{dir: dir, db: db}, nil
}

// Close releases the database connection.
func (s *SessionStore) Close() error {
	return s.db.Close()
}

// SaveMeta atomically writes v (marshalled to JSON) as meta.json.
func (s *SessionStore) SaveMeta(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("store: marshal meta: %w", err)
	}
	tmp := filepath.Join(s.dir, metaFile+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("store: write meta: %w", err)
	}
	return os.Rename(tmp, filepath.Join(s.dir, metaFile))
}

// LoadMeta reads meta.json and unmarshals it into v.
func (s *SessionStore) LoadMeta(v any) error {
	data, err := os.ReadFile(filepath.Join(s.dir, metaFile))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// AppendDisplayMessage inserts one display message (as raw JSON) into the DB.
// Duplicate IDs are ignored (INSERT OR IGNORE).
func (s *SessionStore) AppendDisplayMessage(id string, data json.RawMessage) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO display_messages (id, data) VALUES (?, ?)`,
		id, string(data),
	)
	return err
}

// UpdateDisplayMessage replaces an existing display message payload by ID.
func (s *SessionStore) UpdateDisplayMessage(id string, data json.RawMessage) error {
	_, err := s.db.Exec(
		`UPDATE display_messages SET data = ? WHERE id = ?`,
		string(data),
		id,
	)
	return err
}

// LoadDisplayMessages returns all display messages in insertion order.
func (s *SessionStore) LoadDisplayMessages() ([]json.RawMessage, error) {
	rows, err := s.db.Query(`SELECT data FROM display_messages ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []json.RawMessage
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(raw))
	}
	return out, rows.Err()
}

// ReplaceAPIMessages deletes all existing api messages and inserts the new set.
func (s *SessionStore) ReplaceAPIMessages(messages []json.RawMessage) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`DELETE FROM api_messages`); err != nil {
		return err
	}
	for _, msg := range messages {
		if _, err := tx.Exec(`INSERT INTO api_messages (data) VALUES (?)`, string(msg)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// LoadAPIMessages returns all api messages in insertion order.
func (s *SessionStore) LoadAPIMessages() ([]json.RawMessage, error) {
	rows, err := s.db.Query(`SELECT data FROM api_messages ORDER BY seq`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []json.RawMessage
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(raw))
	}
	return out, rows.Err()
}

// MetaExists reports whether meta.json is present in dir.
func MetaExists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, metaFile))
	return err == nil
}

// ── Schema ────────────────────────────────────────────────────────────────

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS display_messages (
			rowid INTEGER PRIMARY KEY AUTOINCREMENT,
			id    TEXT UNIQUE NOT NULL,
			data  TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS api_messages (
			seq  INTEGER PRIMARY KEY AUTOINCREMENT,
			data TEXT NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	return nil
}
