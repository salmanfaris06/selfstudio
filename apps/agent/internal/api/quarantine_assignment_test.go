package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"selfstudio/agent/internal/processing"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/auth"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/ingestion"
	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/quarantine"
	"selfstudio/agent/internal/sessions"
	"selfstudio/agent/internal/stations"
)

func TestQuarantineAssignmentSameStationPublishesSafeEventAndIsIdempotent(t *testing.T) {
	manager, token, stationStore, activityStore, broker, sessionStore, persistence := sessionDeps(t)
	photoStore := photos.NewStore()
	quarantineStore := quarantine.NewStore()
	mux := quarantineMux(manager, stationStore, activityStore, broker, sessionStore, persistence, photoStore, quarantineStore)
	mux.ServeHTTP(httptest.NewRecorder(), authorizedSessionRequest(token, "/api/stations/station_1/sessions", validStartSessionBody()))
	sessionID := sessionStore.List()[0].SessionID
	record := quarantineStore.Quarantine(stations.Station1ID, "", "C:/camera/A.JPG", 10, time.Now(), time.Now(), time.Now(), quarantine.ReasonNoActiveSession)
	ch, unsubscribe := broker.Subscribe()
	defer unsubscribe()

	rec := httptest.NewRecorder()
	req := assignRequest(token, record.QuarantineID, sessionID)
	mux.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"status":"assigned"`) || !strings.Contains(body, `"assigned_session_id":"`+sessionID+`"`) || !strings.Contains(body, `"photo_id":"photo_`) {
		t.Fatalf("assign status=%d body=%s", rec.Code, body)
	}
	foundAssigned := false
	for i := 0; i < 2; i++ {
		event := <-ch
		if event.EventType == "quarantine.assigned" && event.EntityType == "quarantine" && event.EntityID == record.QuarantineID && !strings.Contains(event.EntityID, "C:/") {
			foundAssigned = true
		}
	}
	if !foundAssigned {
		t.Fatalf("quarantine.assigned event not published")
	}
	entries := activityStore.Recent(10, "quarantine.assigned")
	if len(entries) != 1 || strings.Contains(entries[0].Message, "C:/camera") {
		t.Fatalf("expected one safe activity entry, got %+v", entries)
	}

	again := httptest.NewRecorder()
	mux.ServeHTTP(again, assignRequest(token, record.QuarantineID, sessionID))
	if again.Code != http.StatusOK || photoStore.CountBySession(sessionID) != 1 || len(activityStore.Recent(10, "quarantine.assigned")) != 1 {
		t.Fatalf("idempotent assign status=%d photos=%d activity=%d body=%s", again.Code, photoStore.CountBySession(sessionID), len(activityStore.Recent(10, "quarantine.assigned")), again.Body.String())
	}
}

func TestQuarantineAssignmentPublishesGradedProcessingEvents(t *testing.T) {
	manager, token, stationStore, activityStore, broker, sessionStore, persistence := sessionDeps(t)
	photoStore := photos.NewStore()
	quarantineStore := quarantine.NewStore()
	mux := quarantineMuxWithGradedProcessor(t, manager, stationStore, activityStore, broker, sessionStore, persistence, photoStore, quarantineStore)
	mux.ServeHTTP(httptest.NewRecorder(), authorizedSessionRequest(token, "/api/stations/station_1/sessions", validStartSessionBody()))
	sessionID := sessionStore.List()[0].SessionID
	source := filepath.Join(t.TempDir(), "A.JPG")
	if err := os.WriteFile(source, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	record := quarantineStore.Quarantine(stations.Station1ID, "", source, int64(len("original")), time.Now(), time.Now(), time.Now(), quarantine.ReasonNoActiveSession)
	ch, unsubscribe := broker.Subscribe()
	defer unsubscribe()

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, assignRequest(token, record.QuarantineID, sessionID))
	if rec.Code != http.StatusOK {
		t.Fatalf("assign status=%d body=%s", rec.Code, rec.Body.String())
	}
	seen := map[string]bool{}
	deadline := time.After(2 * time.Second)
	for !(seen["photo.processing_started"] && seen["photo.processed"] && seen["queue.updated"]) {
		select {
		case event := <-ch:
			seen[event.EventType] = true
			if strings.HasPrefix(event.EventType, "photo.processing") && strings.Contains(event.EntityID, source) {
				t.Fatalf("processing event leaked raw source path as entity id: %+v", event)
			}
		case <-deadline:
			t.Fatalf("timed out waiting graded events, saw %+v", seen)
		}
	}
}

func TestQuarantineAssignmentRejectsCrossStationAndConflictingPhoto(t *testing.T) {
	manager, token, stationStore, activityStore, broker, sessionStore, persistence := sessionDeps(t)
	photoStore := photos.NewStore()
	quarantineStore := quarantine.NewStore()
	mux := quarantineMux(manager, stationStore, activityStore, broker, sessionStore, persistence, photoStore, quarantineStore)
	mux.ServeHTTP(httptest.NewRecorder(), authorizedSessionRequest(token, "/api/stations/station_1/sessions", validStartSessionBody()))
	station1Session := sessionStore.List()[0].SessionID
	station2, err := sessionStore.Start(stationStore.List()[1], sessions.StartSessionRequest{CustomerName: "Customer C", OrderNumber: "ORD-003", TimerSeconds: 900}, "local-data/output/station_2/ORD-003", time.Now())
	if err != nil {
		t.Fatalf("start station 2 session: %v", err)
	}
	station2Session := station2.SessionID
	record := quarantineStore.Quarantine(stations.Station1ID, "", "C:/camera/B.JPG", 10, time.Now(), time.Now(), time.Now(), quarantine.ReasonNoActiveSession)

	cross := httptest.NewRecorder()
	mux.ServeHTTP(cross, assignRequest(token, record.QuarantineID, station2Session))
	if cross.Code != http.StatusConflict || !strings.Contains(cross.Body.String(), "QUARANTINE_SESSION_INELIGIBLE") || quarantineStore.CountByStation(stations.Station1ID) != 1 {
		t.Fatalf("cross-station status=%d body=%s", cross.Code, cross.Body.String())
	}
	photoStore.Route(stations.Station1ID, station1Session, record.SourcePath, record.SourceSizeBytes, record.DetectedAt, record.StableAt, time.Now())
	second := quarantineStore.Quarantine(stations.Station1ID, "", "C:/camera/C.JPG", 11, time.Now(), time.Now(), time.Now(), quarantine.ReasonNoActiveSession)
	photoStore.Route(stations.Station1ID, station1Session, second.SourcePath, second.SourceSizeBytes, second.DetectedAt, second.StableAt, time.Now())
	mux.ServeHTTP(httptest.NewRecorder(), authorizedSessionRequest(token, "/api/sessions/"+station1Session+"/end", `{"reason":"manual"}`))
	mux.ServeHTTP(httptest.NewRecorder(), authorizedSessionRequest(token, "/api/stations/station_1/sessions", `{"customer_name":"Customer B","order_number":"ORD-002","timer_seconds":900}`))
	newStation1Session := sessionStore.List()[2].SessionID
	conflict := httptest.NewRecorder()
	mux.ServeHTTP(conflict, assignRequest(token, second.QuarantineID, newStation1Session))
	if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), "QUARANTINE_PHOTO_CONFLICT") {
		t.Fatalf("conflict status=%d body=%s", conflict.Code, conflict.Body.String())
	}
}

func TestNoActiveSessionRejectsLockedSameStationTarget(t *testing.T) {
	manager, token, stationStore, activityStore, broker, sessionStore, persistence := sessionDeps(t)
	photoStore := photos.NewStore()
	quarantineStore := quarantine.NewStore()
	mux := quarantineMux(manager, stationStore, activityStore, broker, sessionStore, persistence, photoStore, quarantineStore)
	mux.ServeHTTP(httptest.NewRecorder(), authorizedSessionRequest(token, "/api/stations/station_1/sessions", validStartSessionBody()))
	sessionID := sessionStore.List()[0].SessionID
	mux.ServeHTTP(httptest.NewRecorder(), authorizedSessionRequest(token, "/api/sessions/"+sessionID+"/end", `{"reason":"manual"}`))
	record := quarantineStore.Quarantine(stations.Station1ID, "", "C:/camera/NO_ACTIVE.JPG", 10, time.Now(), time.Now(), time.Now(), quarantine.ReasonNoActiveSession)

	eligible := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/quarantine/"+record.QuarantineID+"/eligible-sessions", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(eligible, req)
	if eligible.Code != http.StatusOK || strings.Contains(eligible.Body.String(), sessionID) || strings.Contains(eligible.Body.String(), "related_locked_late_photo_recovery") {
		t.Fatalf("locked no-active target should not be eligible status=%d body=%s", eligible.Code, eligible.Body.String())
	}

	assign := httptest.NewRecorder()
	mux.ServeHTTP(assign, assignRequest(token, record.QuarantineID, sessionID))
	if assign.Code != http.StatusConflict || !strings.Contains(assign.Body.String(), "QUARANTINE_SESSION_INELIGIBLE") || quarantineStore.CountByStation(stations.Station1ID) != 1 || photoStore.CountBySession(sessionID) != 0 {
		t.Fatalf("locked no-active assign status=%d body=%s", assign.Code, assign.Body.String())
	}
}

func TestPersistentQuarantineAssignmentReturnsActionableErrorAndRollsBack(t *testing.T) {
	manager, token, stationStore, activityStore, broker, sessionStore, persistence := sessionDeps(t)
	photoStore := photos.NewStore()
	quarantineStore := quarantine.NewStore()
	mux := quarantineMuxWithPersistence(manager, stationStore, activityStore, broker, sessionStore, persistence, photoStore, quarantineStore, func() error { return quarantine.ErrNotAssignable }, func() error {
		if err := photoStore.ReplaceAll([]photos.Photo{}); err != nil {
			return err
		}
		return quarantineStore.ReplaceAll([]quarantine.Record{})
	})
	mux.ServeHTTP(httptest.NewRecorder(), authorizedSessionRequest(token, "/api/stations/station_1/sessions", validStartSessionBody()))
	sessionID := sessionStore.List()[0].SessionID
	record := quarantineStore.Quarantine(stations.Station1ID, "", "C:/camera/PERSIST.JPG", 10, time.Now(), time.Now(), time.Now(), quarantine.ReasonNoActiveSession)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, assignRequest(token, record.QuarantineID, sessionID))
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "QUARANTINE_PERSIST_FAILED") || photoStore.CountBySession(sessionID) != 0 || quarantineStore.CountByStation(stations.Station1ID) != 0 {
		t.Fatalf("persist failure status=%d photos=%d open=%d body=%s", rec.Code, photoStore.CountBySession(sessionID), quarantineStore.CountByStation(stations.Station1ID), rec.Body.String())
	}
}

func TestLatePhotoEligibleSessionsIncludesAndAssignsRelatedLockedSession(t *testing.T) {
	manager, token, stationStore, activityStore, broker, sessionStore, persistence := sessionDeps(t)
	photoStore := photos.NewStore()
	quarantineStore := quarantine.NewStore()
	mux := quarantineMux(manager, stationStore, activityStore, broker, sessionStore, persistence, photoStore, quarantineStore)
	mux.ServeHTTP(httptest.NewRecorder(), authorizedSessionRequest(token, "/api/stations/station_1/sessions", validStartSessionBody()))
	sessionID := sessionStore.List()[0].SessionID
	mux.ServeHTTP(httptest.NewRecorder(), authorizedSessionRequest(token, "/api/sessions/"+sessionID+"/end", `{"reason":"manual"}`))
	record := quarantineStore.Quarantine(stations.Station1ID, sessionID, "C:/camera/LATE.JPG", 10, time.Now(), time.Now(), time.Now(), quarantine.ReasonLatePhoto)
	eligible := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/quarantine/"+record.QuarantineID+"/eligible-sessions", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(eligible, req)
	if eligible.Code != http.StatusOK || !strings.Contains(eligible.Body.String(), sessionID) || !strings.Contains(eligible.Body.String(), "related_locked_late_photo_recovery") {
		t.Fatalf("eligible status=%d body=%s", eligible.Code, eligible.Body.String())
	}

	assign := httptest.NewRecorder()
	mux.ServeHTTP(assign, assignRequest(token, record.QuarantineID, sessionID))
	if assign.Code != http.StatusOK || !strings.Contains(assign.Body.String(), `"status":"assigned"`) || !strings.Contains(assign.Body.String(), `"assigned_session_id":"`+sessionID+`"`) {
		t.Fatalf("related locked late-photo assign status=%d body=%s", assign.Code, assign.Body.String())
	}
}

type fakeAssignmentLUT struct{}

func (fakeAssignmentLUT) Apply(ctx context.Context, inputPath, lutPath, outputPath string) error {
	return os.WriteFile(outputPath, []byte("graded"), 0o644)
}

func quarantineMuxWithGradedProcessor(t *testing.T, manager *auth.Manager, stationStore *stations.Store, activityStore *activity.Store, broker *events.Broker, sessionStore *sessions.Store, persistence sessions.Persistence, photoStore *photos.Store, quarantineStore *quarantine.Store) http.Handler {
	t.Helper()
	stationPersistence := stations.NewPersistence("")
	scanner := ingestion.NewScanner(stationStore)
	router := ingestion.NewRouterWithQuarantine(sessionStore, photoStore, quarantineStore)
	originalSaver := &processing.OriginalSaver{Photos: photoStore, Sessions: sessionStore}
	gradedProcessor := &processing.GradedProcessor{Photos: photoStore, Sessions: sessionStore, Processor: fakeAssignmentLUT{}}
	return NewMuxWithQuarantine(NewAuthHandlerWithActivity(manager, activityStore), NewEventsHandler(broker), NewActivityHandler(activityStore), NewPersistentStationsHandler(stationStore, stationPersistence, activityStore, broker), NewReadinessHandler(stationStore, activityStore, broker, persistence.OutputRoot()), NewEventReadinessHandler(stationStore, activityStore, broker, persistence.OutputRoot()), NewStationConfigHandler(stationStore, stationPersistence, activityStore, broker), NewWatchValidationHandler(stationStore, activityStore, broker), NewSessionsHandlerWithPhotosAndQuarantine(stationStore, sessionStore, persistence, activityStore, broker, persistence.OutputRoot(), photoStore, quarantineStore), NewPersistentIngestionHandler(scanner, router, activityStore, broker, nil, nil), NewPersistentQuarantineHandlerWithProcessors(quarantineStore, sessionStore, photoStore, activityStore, broker, nil, nil, originalSaver, gradedProcessor))
}

func quarantineMux(manager *auth.Manager, stationStore *stations.Store, activityStore *activity.Store, broker *events.Broker, sessionStore *sessions.Store, persistence sessions.Persistence, photoStore *photos.Store, quarantineStore *quarantine.Store) http.Handler {
	return quarantineMuxWithPersistence(manager, stationStore, activityStore, broker, sessionStore, persistence, photoStore, quarantineStore, nil, nil)
}

func quarantineMuxWithPersistence(manager *auth.Manager, stationStore *stations.Store, activityStore *activity.Store, broker *events.Broker, sessionStore *sessions.Store, persistence sessions.Persistence, photoStore *photos.Store, quarantineStore *quarantine.Store, save func() error, rollback func() error) http.Handler {
	stationPersistence := stations.NewPersistence("")
	scanner := ingestion.NewScanner(stationStore)
	router := ingestion.NewRouterWithQuarantine(sessionStore, photoStore, quarantineStore)
	return NewMuxWithQuarantine(NewAuthHandlerWithActivity(manager, activityStore), NewEventsHandler(broker), NewActivityHandler(activityStore), NewPersistentStationsHandler(stationStore, stationPersistence, activityStore, broker), NewReadinessHandler(stationStore, activityStore, broker, persistence.OutputRoot()), NewEventReadinessHandler(stationStore, activityStore, broker, persistence.OutputRoot()), NewStationConfigHandler(stationStore, stationPersistence, activityStore, broker), NewWatchValidationHandler(stationStore, activityStore, broker), NewSessionsHandlerWithPhotosAndQuarantine(stationStore, sessionStore, persistence, activityStore, broker, persistence.OutputRoot(), photoStore, quarantineStore), NewPersistentIngestionHandler(scanner, router, activityStore, broker, save, rollback), NewPersistentQuarantineHandler(quarantineStore, sessionStore, photoStore, activityStore, broker, save, rollback))
}

func assignRequest(token string, quarantineID string, sessionID string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/quarantine/"+quarantineID+"/assign", strings.NewReader(`{"session_id":"`+sessionID+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	return req
}
