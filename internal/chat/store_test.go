package chat

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

	// Create a test user for foreign key constraints
	d.Write.Exec("INSERT INTO users (username, password_hash, role) VALUES ('testuser', 'hash', 'viewer')")

	return d
}

func TestCreateAndCloseSessions(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	id, err := store.CreateSession(ctx, "Test Stream")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero session ID")
	}

	if err := store.CloseSession(ctx, id); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}

	sessions, err := store.GetSessions(ctx, 10)
	if err != nil {
		t.Fatalf("GetSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Title != "Test Stream" {
		t.Errorf("title = %q, want 'Test Stream'", sessions[0].Title)
	}
	if sessions[0].EndedAt == nil {
		t.Error("expected EndedAt to be set after close")
	}
}

func TestSaveAndGetMessages(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	sessionID, _ := store.CreateSession(ctx, "Chat Test")

	store.SaveMessage(ctx, sessionID, 1, "testuser", "hello")
	store.SaveMessage(ctx, sessionID, 1, "testuser", "world")

	msgs, err := store.GetMessages(ctx, sessionID, 10, 0)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Message != "hello" {
		t.Errorf("first message = %q, want 'hello'", msgs[0].Message)
	}
}

func TestDeleteMessage(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	sessionID, _ := store.CreateSession(ctx, "Delete Test")
	store.SaveMessage(ctx, sessionID, 1, "testuser", "to be deleted")
	store.SaveMessage(ctx, sessionID, 1, "testuser", "keep this")

	// Get messages to find the ID of the first message
	msgs, _ := store.GetMessages(ctx, sessionID, 10, 0)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	// Delete the first message
	if err := store.DeleteMessage(ctx, msgs[0].ID); err != nil {
		t.Fatalf("DeleteMessage: %v", err)
	}

	// Should now only see 1 message
	msgs, _ = store.GetMessages(ctx, sessionID, 10, 0)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message after delete, got %d", len(msgs))
	}
	if msgs[0].Message != "keep this" {
		t.Errorf("remaining message = %q, want 'keep this'", msgs[0].Message)
	}
}

func TestDeleteMessage_NotFound(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	err := store.DeleteMessage(ctx, 999)
	if err == nil {
		t.Fatal("expected error for nonexistent message")
	}
}

func TestTimeoutUser(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	// Create timeout user
	d.Write.Exec("INSERT INTO users (username, password_hash, role) VALUES ('mod', 'hash', 'moderator')")

	err := store.TimeoutUser(ctx, 1, 5, 2, "spam")
	if err != nil {
		t.Fatalf("TimeoutUser: %v", err)
	}

	timedOut, err := store.IsTimedOut(ctx, 1)
	if err != nil {
		t.Fatalf("IsTimedOut: %v", err)
	}
	if !timedOut {
		t.Error("expected user to be timed out")
	}
}

func TestIsTimedOut_NotTimedOut(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	timedOut, err := store.IsTimedOut(ctx, 1)
	if err != nil {
		t.Fatalf("IsTimedOut: %v", err)
	}
	if timedOut {
		t.Error("expected user NOT to be timed out")
	}
}

func TestGetMessages_Pagination(t *testing.T) {
	d := setupTestDB(t)
	store := NewStore(d)
	ctx := context.Background()

	sessionID, _ := store.CreateSession(ctx, "Pagination Test")

	for i := 0; i < 5; i++ {
		store.SaveMessage(ctx, sessionID, 1, "testuser", "msg")
	}

	// Get last 3 messages
	msgs, _ := store.GetMessages(ctx, sessionID, 3, 0)
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}

	// Get messages before the first one we got
	before := msgs[0].ID
	older, _ := store.GetMessages(ctx, sessionID, 10, before)
	if len(older) != 2 {
		t.Fatalf("expected 2 older messages, got %d", len(older))
	}
}
