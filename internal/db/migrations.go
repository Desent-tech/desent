package db

import (
	"fmt"
	"strings"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    role          TEXT    NOT NULL DEFAULT 'viewer',
    created_at    INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT OR IGNORE INTO settings (key, value) VALUES
    ('stream_title', 'Live Stream'),
    ('stream_description', '');

CREATE TABLE IF NOT EXISTS stream_sessions (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at INTEGER NOT NULL DEFAULT (unixepoch()),
    ended_at   INTEGER
);

CREATE TABLE IF NOT EXISTS chat_messages (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id INTEGER NOT NULL REFERENCES stream_sessions(id),
    user_id    INTEGER NOT NULL,
    username   TEXT    NOT NULL,
    message    TEXT    NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_session
    ON chat_messages(session_id, created_at);

CREATE TABLE IF NOT EXISTS bans (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL UNIQUE REFERENCES users(id),
    reason     TEXT    NOT NULL DEFAULT '',
    banned_by  INTEGER NOT NULL REFERENCES users(id),
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
`

func Migrate(d *DB) error {
	if _, err := d.Write.Exec(schema); err != nil {
		return fmt.Errorf("db: migrate: %w", err)
	}

	// Add title column to stream_sessions (idempotent for existing DBs).
	_, err := d.Write.Exec("ALTER TABLE stream_sessions ADD COLUMN title TEXT NOT NULL DEFAULT ''")
	if err != nil && !isDuplicateColumn(err) {
		return fmt.Errorf("db: migrate title column: %w", err)
	}

	// Add soft-delete column to chat_messages.
	_, err = d.Write.Exec("ALTER TABLE chat_messages ADD COLUMN deleted_at INTEGER")
	if err != nil && !isDuplicateColumn(err) {
		return fmt.Errorf("db: migrate deleted_at column: %w", err)
	}

	// Timeouts table.
	_, err = d.Write.Exec(`
		CREATE TABLE IF NOT EXISTS timeouts (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id        INTEGER NOT NULL REFERENCES users(id),
			reason         TEXT    NOT NULL DEFAULT '',
			timed_out_by   INTEGER NOT NULL REFERENCES users(id),
			created_at     INTEGER NOT NULL DEFAULT (unixepoch()),
			expires_at     INTEGER NOT NULL
		)`)
	if err != nil {
		return fmt.Errorf("db: migrate timeouts table: %w", err)
	}
	_, _ = d.Write.Exec("CREATE INDEX IF NOT EXISTS idx_timeouts_expires ON timeouts(user_id, expires_at)")

	// VOD path column on stream_sessions.
	_, err = d.Write.Exec("ALTER TABLE stream_sessions ADD COLUMN vod_path TEXT NOT NULL DEFAULT ''")
	if err != nil && !isDuplicateColumn(err) {
		return fmt.Errorf("db: migrate vod_path column: %w", err)
	}

	// Category column on stream_sessions.
	_, err = d.Write.Exec("ALTER TABLE stream_sessions ADD COLUMN category TEXT NOT NULL DEFAULT ''")
	if err != nil && !isDuplicateColumn(err) {
		return fmt.Errorf("db: migrate category column: %w", err)
	}

	// Tags column on stream_sessions.
	_, err = d.Write.Exec("ALTER TABLE stream_sessions ADD COLUMN tags TEXT NOT NULL DEFAULT ''")
	if err != nil && !isDuplicateColumn(err) {
		return fmt.Errorf("db: migrate tags column: %w", err)
	}

	// Default settings for category and tags.
	_, _ = d.Write.Exec("INSERT OR IGNORE INTO settings (key, value) VALUES ('stream_category', '')")
	_, _ = d.Write.Exec("INSERT OR IGNORE INTO settings (key, value) VALUES ('stream_tags', '')")

	// Emotes table.
	_, err = d.Write.Exec(`
		CREATE TABLE IF NOT EXISTS emotes (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			code       TEXT    NOT NULL UNIQUE,
			filename   TEXT    NOT NULL,
			created_at INTEGER NOT NULL DEFAULT (unixepoch())
		)`)
	if err != nil {
		return fmt.Errorf("db: migrate emotes table: %w", err)
	}

	// Clips table.
	_, err = d.Write.Exec(`
		CREATE TABLE IF NOT EXISTS clips (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER NOT NULL REFERENCES stream_sessions(id),
			title      TEXT    NOT NULL DEFAULT '',
			filename   TEXT    NOT NULL,
			start_time INTEGER NOT NULL,
			duration   INTEGER NOT NULL,
			created_by INTEGER NOT NULL REFERENCES users(id),
			created_at INTEGER NOT NULL DEFAULT (unixepoch())
		)`)
	if err != nil {
		return fmt.Errorf("db: migrate clips table: %w", err)
	}

	return nil
}

func isDuplicateColumn(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate column")
}
