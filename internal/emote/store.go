package emote

import (
	"context"
	"fmt"

	"desent/internal/db"
)

type Emote struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Filename  string `json:"filename"`
	CreatedAt int64  `json:"created_at"`
}

type Store struct {
	db *db.DB
}

func NewStore(d *db.DB) *Store {
	return &Store{db: d}
}

func (s *Store) ListEmotes(ctx context.Context) ([]Emote, error) {
	rows, err := s.db.Read.QueryContext(ctx,
		"SELECT id, code, filename, created_at FROM emotes ORDER BY code")
	if err != nil {
		return nil, fmt.Errorf("emote: list: %w", err)
	}
	defer rows.Close()

	var emotes []Emote
	for rows.Next() {
		var e Emote
		if err := rows.Scan(&e.ID, &e.Code, &e.Filename, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("emote: scan: %w", err)
		}
		emotes = append(emotes, e)
	}
	return emotes, rows.Err()
}

func (s *Store) CreateEmote(ctx context.Context, code, filename string) (int64, error) {
	res, err := s.db.Write.ExecContext(ctx,
		"INSERT INTO emotes (code, filename) VALUES (?, ?)",
		code, filename,
	)
	if err != nil {
		return 0, fmt.Errorf("emote: create: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

func (s *Store) GetEmote(ctx context.Context, id int64) (*Emote, error) {
	var e Emote
	err := s.db.Read.QueryRowContext(ctx,
		"SELECT id, code, filename, created_at FROM emotes WHERE id = ?", id,
	).Scan(&e.ID, &e.Code, &e.Filename, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("emote: get: %w", err)
	}
	return &e, nil
}

func (s *Store) DeleteEmote(ctx context.Context, id int64) error {
	res, err := s.db.Write.ExecContext(ctx, "DELETE FROM emotes WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("emote: delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("emote: not found")
	}
	return nil
}
