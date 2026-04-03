package auth

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

func TestCreateUser_FirstUserIsAdmin(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	user, err := store.CreateUser(ctx, "admin1", "password123", 4)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if user.Role != "admin" {
		t.Errorf("first user role = %q, want admin", user.Role)
	}
}

func TestCreateUser_SecondUserIsViewer(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	store.CreateUser(ctx, "admin1", "password123", 4)

	user, err := store.CreateUser(ctx, "viewer1", "password123", 4)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if user.Role != "viewer" {
		t.Errorf("second user role = %q, want viewer", user.Role)
	}
}

func TestCreateUser_DuplicateUsername(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	store.CreateUser(ctx, "alice", "password123", 4)

	_, err := store.CreateUser(ctx, "alice", "password456", 4)
	if err != ErrUsernameTaken {
		t.Errorf("err = %v, want ErrUsernameTaken", err)
	}
}

func TestGetByUsername(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	created, _ := store.CreateUser(ctx, "bob", "password123", 4)

	found, err := store.GetByUsername(ctx, "bob")
	if err != nil {
		t.Fatalf("GetByUsername: %v", err)
	}
	if found.ID != created.ID {
		t.Errorf("ID = %d, want %d", found.ID, created.ID)
	}
}

func TestGetByUsername_NotFound(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	_, err := store.GetByUsername(ctx, "nonexistent")
	if err != ErrUserNotFound {
		t.Errorf("err = %v, want ErrUserNotFound", err)
	}
}

func TestGetByID(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	created, _ := store.CreateUser(ctx, "carol", "password123", 4)

	found, err := store.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if found.Username != "carol" {
		t.Errorf("Username = %q, want carol", found.Username)
	}
}

func TestUpdatePassword(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	user, _ := store.CreateUser(ctx, "dave", "password123", 4)
	oldHash := user.PasswordHash

	err := store.UpdatePassword(ctx, user.ID, "newhash")
	if err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}

	updated, _ := store.GetByID(ctx, user.ID)
	if updated.PasswordHash == oldHash {
		t.Error("password hash should have changed")
	}
	if updated.PasswordHash != "newhash" {
		t.Errorf("PasswordHash = %q, want newhash", updated.PasswordHash)
	}
}
