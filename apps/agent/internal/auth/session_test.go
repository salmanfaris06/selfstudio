package auth

import (
	"errors"
	"testing"
	"time"
)

func TestManagerLoginSuccessCreatesSession(t *testing.T) {
	manager, err := NewManager("123456")
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	token, expiresAt, err := manager.Login("123456")
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if token == "" {
		t.Fatal("Login returned empty token")
	}
	if expiresAt.IsZero() {
		t.Fatal("Login returned zero expiry")
	}
	if !manager.Authenticated(token) {
		t.Fatal("token should be authenticated")
	}
}

func TestManagerLoginRejectsInvalidPIN(t *testing.T) {
	manager, err := NewManager("123456")
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	token, _, err := manager.Login("000000")
	if !errors.Is(err, ErrInvalidPIN) {
		t.Fatalf("Login error = %v, want ErrInvalidPIN", err)
	}
	if token != "" {
		t.Fatalf("token = %q, want empty", token)
	}
}

func TestManagerLogoutClearsSession(t *testing.T) {
	manager, err := NewManager("123456")
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	token, _, err := manager.Login("123456")
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}

	manager.Logout(token)

	if manager.Authenticated(token) {
		t.Fatal("token should not be authenticated after logout")
	}
}

func TestNewManagerRequiresPIN(t *testing.T) {
	_, err := NewManager("")
	if err == nil {
		t.Fatal("NewManager returned nil error for missing PIN")
	}
}

func TestSessionExpiresServerSide(t *testing.T) {
	manager, err := NewManager("123456")
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }

	token, _, err := manager.Login("123456")
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}

	now = now.Add(SessionTTL + time.Second)
	if manager.Authenticated(token) {
		t.Fatal("expired token should not be authenticated")
	}
}

func TestInvalidAttemptsAreRateLimited(t *testing.T) {
	manager, err := NewManager("123456")
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	for range MaxFailedAttempts {
		_, _, err := manager.LoginForKey("client-1", "bad-pin")
		if !errors.Is(err, ErrInvalidPIN) {
			t.Fatalf("LoginForKey error = %v, want ErrInvalidPIN", err)
		}
	}

	_, _, err = manager.LoginForKey("client-1", "123456")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("LoginForKey error = %v, want ErrRateLimited", err)
	}
}
