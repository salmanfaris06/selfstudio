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

func TestWatchValidationAPIRequiresAuth(t *testing.T) {
	mux, _ := watchValidationTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stations/station_1/validation/watch-test", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWatchValidationAPIRejectsUntrustedOrigin(t *testing.T) {
	mux, token := watchValidationTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stations/station_1/validation/watch-test", nil)
	req.Header.Set("Origin", "http://evil.example")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWatchValidationAPISuccessActivityAndSSE(t *testing.T) {
	manager, token, store, activityStore, broker := watchValidationDeps(t)
	input := configureWatchStation(t, store)
	if err := os.WriteFile(filepath.Join(input, "test.jpg"), []byte("jpg"), 0o644); err != nil {
		t.Fatal(err)
	}
	ch, unsub := broker.Subscribe()
	defer unsub()
	mux := watchValidationMux(manager, store, activityStore, broker)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stations/station_1/validation/watch-test", strings.NewReader(`{"timeout_ms":1000,"stability_ms":200}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(activityStore.Recent(10, "station.validation_succeeded")) != 1 {
		t.Fatal("missing success activity")
	}
	select {
	case event := <-ch:
		if event.EventType != "station.validation_completed" {
			t.Fatalf("event=%s", event.EventType)
		}
	case <-time.After(time.Second):
		t.Fatal("missing validation event")
	}
}

func TestWatchValidationAPINoFileRecordsFailure(t *testing.T) {
	manager, token, store, activityStore, broker := watchValidationDeps(t)
	configureWatchStation(t, store)
	mux := watchValidationMux(manager, store, activityStore, broker)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stations/station_1/validation/watch-test", strings.NewReader(`{"timeout_ms":1000,"stability_ms":200}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(activityStore.Recent(10, "station.validation_failed")) != 1 {
		t.Fatal("missing failure activity")
	}
}

func watchValidationTestMux(t *testing.T) (http.Handler, string) {
	t.Helper()
	manager, token, store, activityStore, broker := watchValidationDeps(t)
	return watchValidationMux(manager, store, activityStore, broker), token
}

func watchValidationDeps(t *testing.T) (*auth.Manager, string, *stations.Store, *activity.Store, *events.Broker) {
	t.Helper()
	manager, err := auth.NewManager("123456")
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := manager.Login("123456")
	if err != nil {
		t.Fatal(err)
	}
	return manager, token, stations.NewStore(), activity.NewStore(20), events.NewBroker()
}

func watchValidationMux(manager *auth.Manager, store *stations.Store, activityStore *activity.Store, broker *events.Broker) http.Handler {
	persistence := stations.NewPersistence("")
	return NewMuxWithWatchValidation(NewAuthHandlerWithActivity(manager, activityStore), NewEventsHandler(broker), NewActivityHandler(activityStore), NewPersistentStationsHandler(store, persistence, activityStore, broker), NewReadinessHandler(store, activityStore, broker, ""), NewEventReadinessHandler(store, activityStore, broker, ""), NewStationConfigHandler(store, persistence, activityStore, broker), NewWatchValidationHandler(store, activityStore, broker))
}

func configureWatchStation(t *testing.T, store *stations.Store) string {
	t.Helper()
	input := t.TempDir()
	if _, err := store.Update(stations.Station1ID, stations.UpdateStation{Name: "Station 1", DeviceIdentifier: "Camera", InputFolder: input, BackgroundName: "White", DefaultLUTPath: "D:/lut.cube", OutputRule: "{station_id}"}); err != nil {
		t.Fatal(err)
	}
	return input
}
