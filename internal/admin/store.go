package admin

import (
	"context"
	"fmt"

	"desent/internal/db"
)

type UserInfo struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	CreatedAt int64  `json:"created_at"`
	Banned    bool   `json:"banned"`
	BanReason string `json:"ban_reason,omitempty"`
}

type Store struct {
	db *db.DB
}

func NewStore(d *db.DB) *Store {
	return &Store{db: d}
}

func (s *Store) GetSettings(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.Read.QueryContext(ctx, "SELECT key, value FROM settings")
	if err != nil {
		return nil, fmt.Errorf("admin: get settings: %w", err)
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("admin: scan setting: %w", err)
		}
		settings[k] = v
	}
	return settings, rows.Err()
}

func (s *Store) UpdateSettings(ctx context.Context, kv map[string]string) error {
	for k, v := range kv {
		_, err := s.db.Write.ExecContext(ctx,
			"INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
			k, v,
		)
		if err != nil {
			return fmt.Errorf("admin: update setting %q: %w", k, err)
		}
	}
	return nil
}

func (s *Store) ListUsers(ctx context.Context) ([]UserInfo, error) {
	rows, err := s.db.Read.QueryContext(ctx, `
		SELECT u.id, u.username, u.role, u.created_at,
		       CASE WHEN b.id IS NOT NULL THEN 1 ELSE 0 END,
		       COALESCE(b.reason, '')
		FROM users u
		LEFT JOIN bans b ON b.user_id = u.id
		ORDER BY u.id`)
	if err != nil {
		return nil, fmt.Errorf("admin: list users: %w", err)
	}
	defer rows.Close()

	var users []UserInfo
	for rows.Next() {
		var u UserInfo
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt, &u.Banned, &u.BanReason); err != nil {
			return nil, fmt.Errorf("admin: scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) BanUser(ctx context.Context, userID, bannedBy int64, reason string) error {
	_, err := s.db.Write.ExecContext(ctx,
		"INSERT INTO bans (user_id, banned_by, reason) VALUES (?, ?, ?)",
		userID, bannedBy, reason,
	)
	if err != nil {
		return fmt.Errorf("admin: ban user: %w", err)
	}
	return nil
}

func (s *Store) UnbanUser(ctx context.Context, userID int64) error {
	res, err := s.db.Write.ExecContext(ctx, "DELETE FROM bans WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("admin: unban user: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("admin: user %d is not banned", userID)
	}
	return nil
}

func (s *Store) GetStreamTitle(ctx context.Context) string {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return "Live Stream"
	}
	if v, ok := settings["stream_title"]; ok && v != "" {
		return v
	}
	return "Live Stream"
}

func (s *Store) UpdateUserRole(ctx context.Context, userID int64, role string) error {
	if role != "viewer" && role != "moderator" {
		return fmt.Errorf("admin: invalid role %q (must be viewer or moderator)", role)
	}
	res, err := s.db.Write.ExecContext(ctx,
		"UPDATE users SET role = ? WHERE id = ? AND role != 'admin'",
		role, userID,
	)
	if err != nil {
		return fmt.Errorf("admin: update role: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("admin: user not found or is admin")
	}
	return nil
}

func (s *Store) IsBanned(ctx context.Context, userID int64) (bool, error) {
	var exists bool
	err := s.db.Read.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM bans WHERE user_id = ?)", userID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("admin: check ban: %w", err)
	}
	return exists, nil
}
