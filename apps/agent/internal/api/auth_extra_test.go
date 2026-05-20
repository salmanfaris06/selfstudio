package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/auth"
)

func TestLoginRejectsEmptyPINAsInvalidRequest(t *testing.T) {
	manager, _ := auth.NewManager("123456")
	handler := NewAuthHandlerWithActivity(manager, activity.NewStore(10))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"pin":""}`))
	req.Header.Set("Content-Type", "application/json")
	handler.Login(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestClientKeyIgnoresSpoofedForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")

	if got := clientKey(req); got != "127.0.0.1" {
		t.Fatalf("clientKey = %q, want remote addr", got)
	}
}

func TestLogoutRecordsFailureWithoutSession(t *testing.T) {
	manager, _ := auth.NewManager("123456")
	store := activity.NewStore(10)
	handler := NewAuthHandlerWithActivity(manager, store)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	handler.Logout(rec, req)

	entries := store.Recent(10, "logout.success")
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Result != activity.ResultFailure {
		t.Fatalf("result = %q, want failure", entries[0].Result)
	}
}
