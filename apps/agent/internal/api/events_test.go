package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"selfstudio/agent/internal/auth"
	"selfstudio/agent/internal/events"
)

func TestEventsRequiresAuth(t *testing.T) {
	manager, err := auth.NewManager("123456")
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	mux := NewMux(NewAuthHandler(manager), NewEventsHandler(events.NewBroker()), NewActivityHandler(nil))

	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if !strings.Contains(rec.Body.String(), "UNAUTHORIZED") {
		t.Fatalf("response body = %s, want UNAUTHORIZED", rec.Body.String())
	}
}

func TestEventsStreamHeadersWhenAuthenticated(t *testing.T) {
	manager, err := auth.NewManager("123456")
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	token, _, err := manager.Login("123456")
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}

	server := httptest.NewServer(NewMux(NewAuthHandler(manager), NewEventsHandler(events.NewBroker()), NewActivityHandler(nil)))
	defer server.Close()

	client := server.Client()
	client.Timeout = 2 * time.Second
	req, err := http.NewRequest(http.MethodGet, server.URL+"/events", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", got)
	}
}

func TestWriteSSEEventUsesDotNotationEventAndWrapperData(t *testing.T) {
	event := events.Event{
		EventID:    "evt_1",
		EventType:  "health.updated",
		EntityType: "service",
		EntityID:   "selfstudio-agent",
		OccurredAt: time.Date(2026, 5, 18, 10, 30, 0, 0, time.UTC),
		Data:       map[string]any{"status": "ok"},
	}

	rec := httptest.NewRecorder()
	if err := writeSSEEvent(rec, event); err != nil {
		t.Fatalf("writeSSEEvent returned error: %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "id: evt_1") {
		t.Fatalf("SSE body missing id line: %s", body)
	}
	if !strings.Contains(body, "event: health.updated") {
		t.Fatalf("SSE body missing event line: %s", body)
	}
	for _, part := range []string{"event_id", "event_type", "entity_type", "entity_id", "occurred_at", "data"} {
		if !strings.Contains(body, part) {
			t.Fatalf("SSE body missing %s: %s", part, body)
		}
	}
}

func TestWriteSSEEventRejectsInvalidEventType(t *testing.T) {
	event := events.Event{
		EventID:    "evt_1",
		EventType:  "health.updated\ndata: injected",
		EntityType: "service",
		EntityID:   "selfstudio-agent",
		OccurredAt: time.Date(2026, 5, 18, 10, 30, 0, 0, time.UTC),
		Data:       map[string]any{"status": "ok"},
	}

	rec := httptest.NewRecorder()
	if err := writeSSEEvent(rec, event); err == nil {
		t.Fatal("writeSSEEvent returned nil error for invalid event type")
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("response body = %q, want empty body", rec.Body.String())
	}
}
