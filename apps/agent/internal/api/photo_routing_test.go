package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/auth"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/ingestion"
	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/quarantine"
	"selfstudio/agent/internal/sessions"
	"selfstudio/agent/internal/stations"
)

func TestIngestionScanRoutesPhotoAndSessionDetailShowsPhotos(t *testing.T) {
	manager, token, stationStore, activityStore, broker, sessionStore, persistence := sessionDeps(t)
	photoStore := photos.NewStore()
	input := stationStore.List()[0].InputFolder
	mux := photoRoutingMux(manager, stationStore, activityStore, broker, sessionStore, persistence, photoStore)
	start := httptest.NewRecorder()
	mux.ServeHTTP(start, authorizedSessionRequest(token, "/api/stations/station_1/sessions", validStartSessionBody()))
	if start.Code != http.StatusCreated {
		t.Fatalf("start status=%d body=%s", start.Code, start.Body.String())
	}
	sessionID := sessionStore.List()[0].SessionID
	if err := os.WriteFile(input+"/A.JPG", []byte("jpg"), 0o644); err != nil {
		t.Fatal(err)
	}
	ch, unsubscribe := broker.Subscribe()
	defer unsubscribe()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/ingestion/scan", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"routed_photos"`) || !strings.Contains(body, `"status":"routed"`) || !strings.Contains(body, sessionID) {
		t.Fatalf("scan status=%d body=%s", rec.Code, body)
	}
	foundEvent := false
	for i := 0; i < 2; i++ {
		event := <-ch
		if event.EventType == "photo.routed" && event.EntityType == "photo" && strings.HasPrefix(event.EntityID, "photo_") {
			foundEvent = true
		}
	}
	if !foundEvent {
		t.Fatalf("photo.routed event not published")
	}
	entries := activityStore.Recent(10, "photo.routed")
	if len(entries) != 1 || entries[0].StationID == nil || *entries[0].StationID != stations.Station1ID || entries[0].SessionID == nil || *entries[0].SessionID != sessionID {
		t.Fatalf("activity entries=%+v", entries)
	}
	detail := httptest.NewRecorder()
	detailReq := httptest.NewRequest(http.MethodGet, "/api/sessions/"+sessionID, nil)
	detailReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(detail, detailReq)
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"photo_count":1`) || !strings.Contains(detail.Body.String(), `"photos"`) {
		t.Fatalf("detail status=%d body=%s", detail.Code, detail.Body.String())
	}
}

func photoRoutingMux(manager *auth.Manager, stationStore *stations.Store, activityStore *activity.Store, broker *events.Broker, sessionStore *sessions.Store, persistence sessions.Persistence, photoStore *photos.Store) http.Handler {
	stationPersistence := stations.NewPersistence("")
	scanner := ingestion.NewScanner(stationStore)
	quarantineStore := quarantine.NewStore()
	router := ingestion.NewRouterWithQuarantine(sessionStore, photoStore, quarantineStore)
	return NewMuxWithIngestion(NewAuthHandlerWithActivity(manager, activityStore), NewEventsHandler(broker), NewActivityHandler(activityStore), NewPersistentStationsHandler(stationStore, stationPersistence, activityStore, broker), NewReadinessHandler(stationStore, activityStore, broker, persistence.OutputRoot()), NewEventReadinessHandler(stationStore, activityStore, broker, persistence.OutputRoot()), NewStationConfigHandler(stationStore, stationPersistence, activityStore, broker), NewWatchValidationHandler(stationStore, activityStore, broker), NewSessionsHandlerWithPhotosAndQuarantine(stationStore, sessionStore, persistence, activityStore, broker, persistence.OutputRoot(), photoStore, quarantineStore), NewIngestionHandlerWithRouter(scanner, router, activityStore, broker))
}
