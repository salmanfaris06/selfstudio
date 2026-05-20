package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/auth"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/stations"
)

func TestReadinessRequiresAuth(t *testing.T) {
	mux, _ := readinessTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stations/station_1/readiness", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestReadinessCheckRejectsUntrustedOrigin(t *testing.T) {
	mux, token := readinessTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stations/station_1/readiness/check", strings.NewReader(""))
	req.Header.Set("Origin", "http://evil.example")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestReadinessUnknownStation(t *testing.T) {
	mux, token := readinessTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stations/unknown/readiness/check", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestReadinessCheckRecordsActivityAndPublishesEvent(t *testing.T) {
	manager, token, store, activityStore, broker, outputRoot := readinessDeps(t)
	station, _ := stations.NewStore().Get(stations.Station1ID)
	station.InputFolder = outputRoot
	station.DefaultLUTPath = filepath.Join(outputRoot, "default.cube")
	station.OutputRule = "{station_id}"
	if err := os.MkdirAll(filepath.Join(outputRoot, stations.Station1ID), 0o755); err != nil {
		t.Fatalf("MkdirAll(output rule) error = %v", err)
	}
	_, err := store.Update(stations.Station1ID, stations.UpdateStation{
		Name:             station.Name,
		DeviceIdentifier: station.DeviceIdentifier,
		InputFolder:      station.InputFolder,
		BackgroundName:   station.BackgroundName,
		DefaultLUTPath:   station.DefaultLUTPath,
		OutputRule:       station.OutputRule,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	ch, unsubscribe := broker.Subscribe()
	defer unsubscribe()
	mux := NewMuxWithReadiness(NewAuthHandlerWithActivity(manager, activityStore), NewEventsHandler(broker), NewActivityHandler(activityStore), NewStationsHandler(store, activityStore, broker), NewReadinessHandler(store, activityStore, broker, outputRoot))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stations/station_1/readiness/check", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if entries := activityStore.Recent(10, "station.readiness_checked"); len(entries) != 1 {
		t.Fatalf("activity entries = %d, want 1", len(entries))
	}
	select {
	case event := <-ch:
		if event.EventType != "station.readiness_checked" {
			t.Fatalf("event type = %q", event.EventType)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting readiness event")
	}
}

func readinessTestMux(t *testing.T) (http.Handler, string) {
	t.Helper()
	manager, token, store, activityStore, broker, outputRoot := readinessDeps(t)
	return NewMuxWithReadiness(NewAuthHandlerWithActivity(manager, activityStore), NewEventsHandler(broker), NewActivityHandler(activityStore), NewStationsHandler(store, activityStore, broker), NewReadinessHandler(store, activityStore, broker, outputRoot)), token
}

func readinessDeps(t *testing.T) (*auth.Manager, string, *stations.Store, *activity.Store, *events.Broker, string) {
	t.Helper()
	manager, err := auth.NewManager("123456")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	token, _, err := manager.Login("123456")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "default.cube"), []byte("TITLE test"), 0o644); err != nil {
		t.Fatalf("write lut = %v", err)
	}
	return manager, token, stations.NewStore(), activity.NewStore(20), events.NewBroker(), root
}
