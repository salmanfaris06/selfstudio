package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/auth"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/stations"
)

func TestStationBackupRequiresAuth(t *testing.T) {
	mux, _ := stationConfigTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stations/backup", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestStationBackupRejectsUntrustedOrigin(t *testing.T) {
	mux, token := stationConfigTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stations/backup", nil)
	req.Header.Set("Origin", "http://evil.example")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestStationBackupAndRestoreAPI(t *testing.T) {
	manager, token, store, persistence, activityStore, broker := stationConfigDeps(t)
	mux := stationConfigMux(manager, store, persistence, activityStore, broker)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stations/backup", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("backup status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(activityStore.Recent(10, "station.config_backup_created")) != 1 {
		t.Fatal("missing backup activity")
	}

	filename := extractFilename(t, rec.Body.String())
	if _, err := store.Update(stations.Station1ID, stations.UpdateStation{Name: "Changed", DeviceIdentifier: "Sony", InputFolder: "D:/changed", BackgroundName: "White", DefaultLUTPath: "D:/lut.cube", OutputRule: "{station_id}"}); err != nil {
		t.Fatal(err)
	}
	ch, unsub := broker.Subscribe()
	defer unsub()
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/stations/restore", strings.NewReader(`{"filename":"`+filename+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("restore status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(activityStore.Recent(10, "station.config_restored")) != 1 {
		t.Fatal("missing restore activity")
	}
	select {
	case event := <-ch:
		if event.EventType != "stations.restored" {
			t.Fatalf("event=%s", event.EventType)
		}
	case <-time.After(time.Second):
		t.Fatal("missing restored event")
	}
}

func TestStationRestoreRejectsInvalidBackup(t *testing.T) {
	mux, token := stationConfigTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stations/restore", strings.NewReader(`{"filename":"../bad.json"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func stationConfigTestMux(t *testing.T) (http.Handler, string) {
	t.Helper()
	manager, token, store, persistence, activityStore, broker := stationConfigDeps(t)
	return stationConfigMux(manager, store, persistence, activityStore, broker), token
}

func stationConfigDeps(t *testing.T) (*auth.Manager, string, *stations.Store, stations.Persistence, *activity.Store, *events.Broker) {
	t.Helper()
	manager, err := auth.NewManager("123456")
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := manager.Login("123456")
	if err != nil {
		t.Fatal(err)
	}
	return manager, token, stations.NewStore(), stations.NewPersistence(t.TempDir()), activity.NewStore(20), events.NewBroker()
}

func stationConfigMux(manager *auth.Manager, store *stations.Store, persistence stations.Persistence, activityStore *activity.Store, broker *events.Broker) http.Handler {
	return NewMuxWithStationConfig(NewAuthHandlerWithActivity(manager, activityStore), NewEventsHandler(broker), NewActivityHandler(activityStore), NewPersistentStationsHandler(store, persistence, activityStore, broker), NewReadinessHandler(store, activityStore, broker, ""), NewEventReadinessHandler(store, activityStore, broker, ""), NewStationConfigHandler(store, persistence, activityStore, broker))
}

func extractFilename(t *testing.T, body string) string {
	t.Helper()
	marker := `"filename":"`
	idx := strings.Index(body, marker)
	if idx < 0 {
		t.Fatalf("filename not found in %s", body)
	}
	start := idx + len(marker)
	end := strings.Index(body[start:], `"`)
	if end < 0 {
		t.Fatalf("filename end not found in %s", body)
	}
	return body[start : start+end]
}
