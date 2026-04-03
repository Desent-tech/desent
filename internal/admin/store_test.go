package admin

import (
	"context"
	"testing"

	"desent/internal/db"
)

func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	if err := db.Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return d
}

func createTestUser(t *testing.T, d *db.DB, username, role string) int64 {
	t.Helper()
	res, err := d.Write.Exec(
		"INSERT INTO users (username, password_hash, role) VALUES (?, 'hash', ?)",
		username, role,
	)
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func TestGetSettings_Default(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	settings, err := store.GetSettings(ctx)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if settings["stream_title"] != "Live Stream" {
		t.Errorf("stream_title = %q, want 'Live Stream'", settings["stream_title"])
	}
}

func TestUpdateSettings(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	err := store.UpdateSettings(ctx, map[string]string{"stream_title": "My Stream"})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	settings, _ := store.GetSettings(ctx)
	if settings["stream_title"] != "My Stream" {
		t.Errorf("stream_title = %q, want 'My Stream'", settings["stream_title"])
	}
}

func TestUpdateSettings_NewKey(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	store.UpdateSettings(ctx, map[string]string{"custom_key": "custom_value"})

	settings, _ := store.GetSettings(ctx)
	if settings["custom_key"] != "custom_value" {
		t.Errorf("custom_key = %q, want 'custom_value'", settings["custom_key"])
	}
}

func TestListUsers(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	createTestUser(t, d, "admin1", "admin")
	createTestUser(t, d, "viewer1", "viewer")

	users, err := store.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
}

func TestBanAndUnban(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	adminID := createTestUser(t, d, "admin1", "admin")
	viewerID := createTestUser(t, d, "viewer1", "viewer")

	// Ban
	err := store.BanUser(ctx, viewerID, adminID, "bad behavior")
	if err != nil {
		t.Fatalf("BanUser: %v", err)
	}

	banned, err := store.IsBanned(ctx, viewerID)
	if err != nil {
		t.Fatalf("IsBanned: %v", err)
	}
	if !banned {
		t.Error("expected user to be banned")
	}

	// Check users list shows ban
	users, _ := store.ListUsers(ctx)
	for _, u := range users {
		if u.ID == viewerID {
			if !u.Banned {
				t.Error("expected user to show as banned in list")
			}
			if u.BanReason != "bad behavior" {
				t.Errorf("BanReason = %q, want 'bad behavior'", u.BanReason)
			}
		}
	}

	// Unban
	err = store.UnbanUser(ctx, viewerID)
	if err != nil {
		t.Fatalf("UnbanUser: %v", err)
	}

	banned, _ = store.IsBanned(ctx, viewerID)
	if banned {
		t.Error("expected user to be unbanned")
	}
}

func TestBanUser_Duplicate(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	adminID := createTestUser(t, d, "admin1", "admin")
	viewerID := createTestUser(t, d, "viewer1", "viewer")

	store.BanUser(ctx, viewerID, adminID, "first ban")

	err := store.BanUser(ctx, viewerID, adminID, "second ban")
	if err == nil {
		t.Fatal("expected error for duplicate ban")
	}
}

func TestUnbanUser_NotBanned(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	createTestUser(t, d, "viewer1", "viewer")

	err := store.UnbanUser(ctx, 1)
	if err == nil {
		t.Fatal("expected error for unbanning non-banned user")
	}
}

func TestUpdateUserRole(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	viewerID := createTestUser(t, d, "viewer1", "viewer")

	err := store.UpdateUserRole(ctx, viewerID, "moderator")
	if err != nil {
		t.Fatalf("UpdateUserRole: %v", err)
	}

	users, _ := store.ListUsers(ctx)
	for _, u := range users {
		if u.ID == viewerID && u.Role != "moderator" {
			t.Errorf("role = %q, want moderator", u.Role)
		}
	}
}

func TestUpdateUserRole_InvalidRole(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	viewerID := createTestUser(t, d, "viewer1", "viewer")

	err := store.UpdateUserRole(ctx, viewerID, "superadmin")
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
}

func TestUpdateUserRole_CannotChangeAdmin(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	adminID := createTestUser(t, d, "admin1", "admin")

	err := store.UpdateUserRole(ctx, adminID, "viewer")
	if err == nil {
		t.Fatal("expected error when changing admin role")
	}
}

func TestGetStreamTitle(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	// Default title
	title := store.GetStreamTitle(ctx)
	if title != "Live Stream" {
		t.Errorf("default title = %q, want 'Live Stream'", title)
	}

	// Custom title
	store.UpdateSettings(ctx, map[string]string{"stream_title": "My Custom Stream"})
	title = store.GetStreamTitle(ctx)
	if title != "My Custom Stream" {
		t.Errorf("custom title = %q, want 'My Custom Stream'", title)
	}
}
