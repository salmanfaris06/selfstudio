package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"selfstudio/agent/internal/auth"
	"selfstudio/agent/internal/stations"
)

func TestEventReadinessRequiresAuth(t *testing.T) {
	mux, _ := eventReadinessTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/readiness", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestEventReadinessCheckRejectsUntrustedOrigin(t *testing.T) {
	mux, token := eventReadinessTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/readiness/check", nil)
	req.Header.Set("Origin", "http://evil.example")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestEventReadinessUnavailableReturnsError(t *testing.T) {
	manager, token, _, activityStore, broker, outputRoot := readinessDeps(t)
	mux := NewMuxWithEventReadiness(
		NewAuthHandlerWithActivity(manager, activityStore),
		NewEventsHandler(broker),
		NewActivityHandler(activityStore),
		StationsHandler{},
		ReadinessHandler{},
		NewEventReadinessHandler(nil, activityStore, broker, outputRoot),
	)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/readiness", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestEventReadinessCheckRecordsFailureActivityAndPublishesEvent(t *testing.T) {
	manager, token, store, activityStore, broker, outputRoot := readinessDeps(t)
	ch, unsubscribe := broker.Subscribe()
	defer unsubscribe()
	mux := NewMuxWithEventReadiness(
		NewAuthHandlerWithActivity(manager, activityStore),
		NewEventsHandler(broker),
		NewActivityHandler(activityStore),
		NewStationsHandler(store, activityStore, broker),
		NewReadinessHandler(store, activityStore, broker, outputRoot),
		NewEventReadinessHandler(store, activityStore, broker, outputRoot),
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/readiness/check", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if entries := activityStore.Recent(10, "readiness.check_failed"); len(entries) != 1 {
		t.Fatalf("failure activity entries = %d, want 1", len(entries))
	}
	select {
	case event := <-ch:
		if event.EventType != "readiness.checked" {
			t.Fatalf("event type = %q", event.EventType)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting readiness event")
	}
}

func TestEventReadinessCheckWarningRecordsSuccessActivity(t *testing.T) {
	manager, token, store, activityStore, broker, outputRoot := readinessDeps(t)
	configureValidChecklistStations(t, store, outputRoot)
	mux := NewMuxWithEventReadiness(
		NewAuthHandlerWithActivity(manager, activityStore),
		NewEventsHandler(broker),
		NewActivityHandler(activityStore),
		NewStationsHandler(store, activityStore, broker),
		NewReadinessHandler(store, activityStore, broker, outputRoot),
		NewEventReadinessHandler(store, activityStore, broker, outputRoot),
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/readiness/check", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if entries := activityStore.Recent(10, "readiness.checked"); len(entries) != 1 {
		t.Fatalf("success activity entries = %d, want 1", len(entries))
	}
}

func eventReadinessTestMux(t *testing.T) (http.Handler, string) {
	t.Helper()
	manager, token, store, activityStore, broker, outputRoot := readinessDeps(t)
	return NewMuxWithEventReadiness(
		NewAuthHandlerWithActivity(manager, activityStore),
		NewEventsHandler(broker),
		NewActivityHandler(activityStore),
		NewStationsHandler(store, activityStore, broker),
		NewReadinessHandler(store, activityStore, broker, outputRoot),
		NewEventReadinessHandler(store, activityStore, broker, outputRoot),
	), token
}

func configureValidChecklistStations(t *testing.T, store *stations.Store, outputRoot string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(outputRoot) error = %v", err)
	}
	for _, station := range store.List() {
		input := filepath.Join(root, station.StationID, "input")
		output := filepath.Join(outputRoot, station.StationID)
		lut := filepath.Join(root, station.StationID, "default.cube")
		if err := os.MkdirAll(input, 0o755); err != nil {
			t.Fatalf("MkdirAll(input) error = %v", err)
		}
		if err := os.MkdirAll(output, 0o755); err != nil {
			t.Fatalf("MkdirAll(output) error = %v", err)
		}
		if err := os.WriteFile(lut, []byte("TITLE test"), 0o644); err != nil {
			t.Fatalf("WriteFile(lut) error = %v", err)
		}
		_, err := store.Update(station.StationID, stations.UpdateStation{Name: station.Name, DeviceIdentifier: station.DeviceIdentifier, InputFolder: input, BackgroundName: station.BackgroundName, DefaultLUTPath: lut, OutputRule: "{station_id}"})
		if err != nil {
			t.Fatalf("Update(%s) error = %v", station.StationID, err)
		}
	}
}
