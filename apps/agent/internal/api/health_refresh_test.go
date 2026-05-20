package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/auth"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/stations"
)

func TestHealthRefreshRequiresAuth(t *testing.T) {
	mux, _ := healthRefreshTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stations/station_1/health/refresh", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHealthRefreshRejectsUntrustedOrigin(t *testing.T) {
	mux, token := healthRefreshTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stations/station_1/health/refresh", nil)
	req.Header.Set("Origin", "http://evil.example")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHealthRefreshUnknownStation(t *testing.T) {
	mux, token := healthRefreshTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stations/unknown/health/refresh", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHealthRefreshActivityAndSSE(t *testing.T) {
	manager, token, store, activityStore, broker := healthRefreshDeps(t)
	mux := healthRefreshMux(manager, store, activityStore, broker)
	ch, unsub := broker.Subscribe()
	defer unsub()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stations/station_1/health/refresh", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(activityStore.Recent(10, "station.health_refreshed"))+len(activityStore.Recent(10, "station.health_refresh_failed")) != 1 {
		t.Fatal("missing health refresh activity")
	}
	select {
	case event := <-ch:
		if event.EventType != "station.health_refreshed" {
			t.Fatalf("event=%s", event.EventType)
		}
	case <-time.After(time.Second):
		t.Fatal("missing health refresh event")
	}
}

func healthRefreshTestMux(t *testing.T) (http.Handler, string) {
	t.Helper()
	manager, token, store, activityStore, broker := healthRefreshDeps(t)
	return healthRefreshMux(manager, store, activityStore, broker), token
}

func healthRefreshDeps(t *testing.T) (*auth.Manager, string, *stations.Store, *activity.Store, *events.Broker) {
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

func healthRefreshMux(manager *auth.Manager, store *stations.Store, activityStore *activity.Store, broker *events.Broker) http.Handler {
	persistence := stations.NewPersistence("")
	return NewMuxWithWatchValidation(NewAuthHandlerWithActivity(manager, activityStore), NewEventsHandler(broker), NewActivityHandler(activityStore), NewPersistentStationsHandler(store, persistence, activityStore, broker), NewReadinessHandler(store, activityStore, broker, ""), NewEventReadinessHandler(store, activityStore, broker, ""), NewStationConfigHandler(store, persistence, activityStore, broker), NewWatchValidationHandler(store, activityStore, broker))
}
