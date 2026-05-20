package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/auth"
	"selfstudio/agent/internal/events"
)

func TestActivityListRequiresAuth(t *testing.T) {
	manager, _ := auth.NewManager("123456")
	store := activity.NewStore(10)
	mux := NewMux(NewAuthHandlerWithActivity(manager, store), NewEventsHandler(events.NewBroker()), NewActivityHandler(store))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/activity", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestActivityListReturnsRecentEntries(t *testing.T) {
	manager, _ := auth.NewManager("123456")
	token, _, _ := manager.Login("123456")
	store := activity.NewStore(10)
	store.Record("login.success", activity.ResultSuccess, "Operator login berhasil.")
	store.Record("health.recheck", activity.ResultSuccess, "Health dashboard diperiksa.")
	mux := NewMux(NewAuthHandlerWithActivity(manager, store), NewEventsHandler(events.NewBroker()), NewActivityHandler(store))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/activity?action_type=health.recheck", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var response DataResponse[ActivityListData]
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data.Entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(response.Data.Entries))
	}
	if response.Data.Entries[0].Message == "" || strings.Contains(response.Data.Entries[0].Message, "123456") {
		t.Fatalf("unsafe or empty message: %q", response.Data.Entries[0].Message)
	}
}

func TestLoginRecordsSafeActivity(t *testing.T) {
	manager, _ := auth.NewManager("123456")
	store := activity.NewStore(10)
	handler := NewAuthHandlerWithActivity(manager, store)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"pin":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	handler.Login(rec, req)

	entries := store.Recent(10, "login.failure")
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if strings.Contains(entries[0].Message, "bad") || strings.Contains(entries[0].Message, "123456") {
		t.Fatalf("activity message leaked secret: %q", entries[0].Message)
	}
}

func TestConfigPlaceholderRouteRequiresAuth(t *testing.T) {
	manager, _ := auth.NewManager("123456")
	store := activity.NewStore(10)
	mux := NewMux(NewAuthHandlerWithActivity(manager, store), NewEventsHandler(events.NewBroker()), NewActivityHandler(store))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/config/placeholder-action", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestConfigPlaceholderRouteRejectsUntrustedOrigin(t *testing.T) {
	manager, _ := auth.NewManager("123456")
	token, _, _ := manager.Login("123456")
	store := activity.NewStore(10)
	mux := NewMux(NewAuthHandlerWithActivity(manager, store), NewEventsHandler(events.NewBroker()), NewActivityHandler(store))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/config/placeholder-action", nil)
	req.Header.Set("Origin", "http://evil.example")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestConfigPlaceholderRecordsActivity(t *testing.T) {
	store := activity.NewStore(10)
	handler := NewActivityHandler(store)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/config/placeholder-action", nil)
	handler.ConfigPlaceholderAction(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	entries := store.Recent(10, "config.placeholder")
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
}
