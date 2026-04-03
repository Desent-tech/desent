package chat

import (
	"context"
	"fmt"
	"time"

	"desent/internal/db"
)

type StreamSession struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	StartedAt int64  `json:"started_at"`
	EndedAt   *int64 `json:"ended_at,omitempty"`
	VODPath   string `json:"vod_path,omitempty"`
	Category  string `json:"category,omitempty"`
	Tags      string `json:"tags,omitempty"`
}

type ChatMessage struct {
	ID        int64  `json:"id"`
	SessionID int64  `json:"session_id"`
	UserID    int64  `json:"user_id"`
	Username  string `json:"username"`
	Message   string `json:"message"`
	CreatedAt int64  `json:"created_at"`
}

type Store struct {
	db *db.DB
}

func NewStore(d *db.DB) *Store {
	return &Store{db: d}
}

func (s *Store) CreateSession(ctx context.Context, title, category, tags string) (int64, error) {
	res, err := s.db.Write.ExecContext(ctx,
		"INSERT INTO stream_sessions (title, category, tags) VALUES (?, ?, ?)",
		title, category, tags,
	)
	if err != nil {
		return 0, fmt.Errorf("chat: create session: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

func (s *Store) SetVODPath(ctx context.Context, sessionID int64, vodPath string) error {
	_, err := s.db.Write.ExecContext(ctx,
		"UPDATE stream_sessions SET vod_path = ? WHERE id = ?",
		vodPath, sessionID,
	)
	if err != nil {
		return fmt.Errorf("chat: set vod path: %w", err)
	}
	return nil
}

func (s *Store) CloseSession(ctx context.Context, sessionID int64) error {
	now := time.Now().Unix()
	_, err := s.db.Write.ExecContext(ctx,
		"UPDATE stream_sessions SET ended_at = ? WHERE id = ?",
		now, sessionID,
	)
	if err != nil {
		return fmt.Errorf("chat: close session: %w", err)
	}
	return nil
}

func (s *Store) SaveMessage(ctx context.Context, sessionID, userID int64, username, message string) error {
	_, err := s.db.Write.ExecContext(ctx,
		"INSERT INTO chat_messages (session_id, user_id, username, message) VALUES (?, ?, ?, ?)",
		sessionID, userID, username, message,
	)
	if err != nil {
		return fmt.Errorf("chat: save message: %w", err)
	}
	return nil
}

func (s *Store) DeleteMessage(ctx context.Context, msgID int64) error {
	res, err := s.db.Write.ExecContext(ctx,
		"UPDATE chat_messages SET deleted_at = unixepoch() WHERE id = ? AND deleted_at IS NULL",
		msgID,
	)
	if err != nil {
		return fmt.Errorf("chat: delete message: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("chat: message %d not found", msgID)
	}
	return nil
}

func (s *Store) TimeoutUser(ctx context.Context, userID int64, durationMin int, timedOutBy int64, reason string) error {
	expiresAt := time.Now().Add(time.Duration(durationMin) * time.Minute).Unix()
	_, err := s.db.Write.ExecContext(ctx,
		"INSERT INTO timeouts (user_id, reason, timed_out_by, expires_at) VALUES (?, ?, ?, ?)",
		userID, reason, timedOutBy, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("chat: timeout user: %w", err)
	}
	return nil
}

func (s *Store) IsTimedOut(ctx context.Context, userID int64) (bool, error) {
	var exists bool
	err := s.db.Read.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM timeouts WHERE user_id = ? AND expires_at > unixepoch())",
		userID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("chat: check timeout: %w", err)
	}
	return exists, nil
}

func (s *Store) GetMessages(ctx context.Context, sessionID int64, limit int, beforeID int64) ([]ChatMessage, error) {
	query := "SELECT id, session_id, user_id, username, message, created_at FROM chat_messages WHERE session_id = ? AND deleted_at IS NULL"
	args := []any{sessionID}

	if beforeID > 0 {
		query += " AND id < ?"
		args = append(args, beforeID)
	}
	query += " ORDER BY created_at DESC, id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Read.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("chat: get messages: %w", err)
	}
	defer rows.Close()

	var msgs []ChatMessage
	for rows.Next() {
		var m ChatMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.UserID, &m.Username, &m.Message, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("chat: scan message: %w", err)
		}
		msgs = append(msgs, m)
	}

	// Reverse to chronological order
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, rows.Err()
}

func (s *Store) GetSessions(ctx context.Context, limit int) ([]StreamSession, error) {
	rows, err := s.db.Read.QueryContext(ctx,
		"SELECT id, title, started_at, ended_at, vod_path, category, tags FROM stream_sessions ORDER BY id DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("chat: get sessions: %w", err)
	}
	defer rows.Close()

	var sessions []StreamSession
	for rows.Next() {
		var s StreamSession
		if err := rows.Scan(&s.ID, &s.Title, &s.StartedAt, &s.EndedAt, &s.VODPath, &s.Category, &s.Tags); err != nil {
			return nil, fmt.Errorf("chat: scan session: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// GetVODPath returns the VOD path for a session.
func (s *Store) GetVODPath(ctx context.Context, sessionID int64) (string, error) {
	var vodPath string
	err := s.db.Read.QueryRowContext(ctx,
		"SELECT vod_path FROM stream_sessions WHERE id = ?", sessionID,
	).Scan(&vodPath)
	if err != nil {
		return "", fmt.Errorf("chat: get vod path: %w", err)
	}
	return vodPath, nil
}

// GetSession returns a single session by ID.
func (s *Store) GetSession(ctx context.Context, sessionID int64) (*StreamSession, error) {
	var sess StreamSession
	err := s.db.Read.QueryRowContext(ctx,
		"SELECT id, title, started_at, ended_at, vod_path, category, tags FROM stream_sessions WHERE id = ?",
		sessionID,
	).Scan(&sess.ID, &sess.Title, &sess.StartedAt, &sess.EndedAt, &sess.VODPath, &sess.Category, &sess.Tags)
	if err != nil {
		return nil, fmt.Errorf("chat: get session: %w", err)
	}
	return &sess, nil
}
