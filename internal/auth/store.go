package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"desent/internal/db"
)

var (
	ErrUserNotFound  = errors.New("auth: user not found")
	ErrUsernameTaken = errors.New("auth: username already taken")
)

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Role         string
}

type Store struct {
	db *db.DB
}

func NewStore(d *db.DB) *Store {
	return &Store{db: d}
}

func (s *Store) CreateUser(ctx context.Context, username, password string, bcryptCost int) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("auth: bcrypt: %w", err)
	}

	role := "viewer"
	var count int
	if err := s.db.Read.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return nil, fmt.Errorf("auth: count users: %w", err)
	}
	if count == 0 {
		role = "admin"
	}

	res, err := s.db.Write.ExecContext(ctx,
		"INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)",
		username, string(hash), role,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrUsernameTaken
		}
		return nil, fmt.Errorf("auth: insert user: %w", err)
	}

	id, _ := res.LastInsertId()
	return &User{ID: id, Username: username, PasswordHash: string(hash), Role: role}, nil
}

func (s *Store) GetByUsername(ctx context.Context, username string) (*User, error) {
	u := &User{}
	err := s.db.Read.QueryRowContext(ctx,
		"SELECT id, username, password_hash, role FROM users WHERE username = ?",
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth: get user: %w", err)
	}
	return u, nil
}

func (s *Store) GetByID(ctx context.Context, id int64) (*User, error) {
	u := &User{}
	err := s.db.Read.QueryRowContext(ctx,
		"SELECT id, username, password_hash, role FROM users WHERE id = ?",
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth: get user by id: %w", err)
	}
	return u, nil
}

func (s *Store) UpdatePassword(ctx context.Context, userID int64, newHash string) error {
	_, err := s.db.Write.ExecContext(ctx,
		"UPDATE users SET password_hash = ? WHERE id = ?",
		newHash, userID,
	)
	if err != nil {
		return fmt.Errorf("auth: update password: %w", err)
	}
	return nil
}
