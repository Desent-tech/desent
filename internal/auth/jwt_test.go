package auth

import (
	"testing"
	"time"
)

func TestGenerateAndValidate(t *testing.T) {
	ts := NewTokenService("test-secret-key")
	user := &User{ID: 1, Username: "alice", Role: "admin"}

	token, err := ts.Generate(user)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := ts.Validate(token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if claims.UserID != 1 {
		t.Errorf("UserID = %d, want 1", claims.UserID)
	}
	if claims.Username != "alice" {
		t.Errorf("Username = %q, want alice", claims.Username)
	}
	if claims.Role != "admin" {
		t.Errorf("Role = %q, want admin", claims.Role)
	}
}

func TestValidateInvalidToken(t *testing.T) {
	ts := NewTokenService("test-secret-key")

	_, err := ts.Validate("not-a-valid-token")
	if err != ErrInvalidToken {
		t.Errorf("err = %v, want ErrInvalidToken", err)
	}
}

func TestValidateWrongSecret(t *testing.T) {
	ts1 := NewTokenService("secret-one")
	ts2 := NewTokenService("secret-two")

	user := &User{ID: 1, Username: "bob", Role: "viewer"}
	token, _ := ts1.Generate(user)

	_, err := ts2.Validate(token)
	if err != ErrInvalidToken {
		t.Errorf("err = %v, want ErrInvalidToken", err)
	}
}

func TestValidateExpiredToken(t *testing.T) {
	ts := &TokenService{
		secret: []byte("test-secret"),
		ttl:    -1 * time.Hour, // already expired
	}

	user := &User{ID: 1, Username: "charlie", Role: "viewer"}
	token, _ := ts.Generate(user)

	_, err := ts.Validate(token)
	if err != ErrInvalidToken {
		t.Errorf("err = %v, want ErrInvalidToken", err)
	}
}
