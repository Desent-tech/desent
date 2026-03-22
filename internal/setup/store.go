package setup

import (
	"context"
	"fmt"

	"desent/internal/db"
)

type Store struct {
	db *db.DB
}

func NewStore(d *db.DB) *Store {
	return &Store{db: d}
}

func (s *Store) SetupRequired(ctx context.Context) (bool, error) {
	var count int
	if err := s.db.Read.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return false, fmt.Errorf("setup: count users: %w", err)
	}
	return count == 0, nil
}
