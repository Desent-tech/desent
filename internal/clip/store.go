package clip

import (
	"context"
	"fmt"

	"desent/internal/db"
)

type Clip struct {
	ID        int64  `json:"id"`
	SessionID int64  `json:"session_id"`
	Title     string `json:"title"`
	Filename  string `json:"filename"`
	StartTime int    `json:"start_time"`
	Duration  int    `json:"duration"`
	CreatedBy int64  `json:"created_by"`
	CreatedAt int64  `json:"created_at"`
}

type Store struct {
	db *db.DB
}

func NewStore(d *db.DB) *Store {
	return &Store{db: d}
}

func (s *Store) CreateClip(ctx context.Context, sessionID, createdBy int64, title, filename string, startTime, duration int) (int64, error) {
	res, err := s.db.Write.ExecContext(ctx,
		"INSERT INTO clips (session_id, title, filename, start_time, duration, created_by) VALUES (?, ?, ?, ?, ?, ?)",
		sessionID, title, filename, startTime, duration, createdBy,
	)
	if err != nil {
		return 0, fmt.Errorf("clip: create: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

func (s *Store) ListClips(ctx context.Context, sessionID int64) ([]Clip, error) {
	rows, err := s.db.Read.QueryContext(ctx,
		"SELECT id, session_id, title, filename, start_time, duration, created_by, created_at FROM clips WHERE session_id = ? ORDER BY created_at DESC",
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("clip: list: %w", err)
	}
	defer rows.Close()

	var clips []Clip
	for rows.Next() {
		var c Clip
		if err := rows.Scan(&c.ID, &c.SessionID, &c.Title, &c.Filename, &c.StartTime, &c.Duration, &c.CreatedBy, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("clip: scan: %w", err)
		}
		clips = append(clips, c)
	}
	return clips, rows.Err()
}

func (s *Store) ListAllClips(ctx context.Context, limit int) ([]Clip, error) {
	rows, err := s.db.Read.QueryContext(ctx,
		"SELECT id, session_id, title, filename, start_time, duration, created_by, created_at FROM clips ORDER BY created_at DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("clip: list all: %w", err)
	}
	defer rows.Close()

	var clips []Clip
	for rows.Next() {
		var c Clip
		if err := rows.Scan(&c.ID, &c.SessionID, &c.Title, &c.Filename, &c.StartTime, &c.Duration, &c.CreatedBy, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("clip: scan: %w", err)
		}
		clips = append(clips, c)
	}
	return clips, rows.Err()
}

func (s *Store) GetClip(ctx context.Context, id int64) (*Clip, error) {
	var c Clip
	err := s.db.Read.QueryRowContext(ctx,
		"SELECT id, session_id, title, filename, start_time, duration, created_by, created_at FROM clips WHERE id = ?", id,
	).Scan(&c.ID, &c.SessionID, &c.Title, &c.Filename, &c.StartTime, &c.Duration, &c.CreatedBy, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("clip: get: %w", err)
	}
	return &c, nil
}

func (s *Store) DeleteClip(ctx context.Context, id int64) error {
	res, err := s.db.Write.ExecContext(ctx, "DELETE FROM clips WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("clip: delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("clip: not found")
	}
	return nil
}
