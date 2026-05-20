package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/auth"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/sessions"
	"selfstudio/agent/internal/stations"
)

func TestListSessionsRequiresAuth(t *testing.T) {
	mux, _ := sessionTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestListSessionsReturnsSessions(t *testing.T) {
	mux, token := sessionTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, authorizedSessionRequest(token, "/api/stations/station_1/sessions", validStartSessionBody()))
	if rec.Code != http.StatusCreated {
		t.Fatalf("start status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "sessions") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetSessionDetail(t *testing.T) {
	mux, token := sessionTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, authorizedSessionRequest(token, "/api/stations/station_1/sessions", validStartSessionBody()))
	if rec.Code != http.StatusCreated {
		t.Fatalf("start status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	start := strings.Index(body, "sess_")
	if start < 0 {
		t.Fatalf("session id missing: %s", body)
	}
	end := start
	for end < len(body) && body[end] != '"' {
		end++
	}
	sessionID := body[start:end]
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/sessions/"+sessionID, nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "local_output_folder") || !strings.Contains(rec.Body.String(), "drive_target_status") || !strings.Contains(rec.Body.String(), "drive_session_folder_id") || !strings.Contains(rec.Body.String(), "drive_folder_path") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetUnknownSessionDetail(t *testing.T) {
	mux, token := sessionTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/sessions/missing", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "SESSION_NOT_FOUND") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStartSessionRequiresAuth(t *testing.T) {
	mux, _ := sessionTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stations/station_1/sessions", strings.NewReader(validStartSessionBody()))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestStartSessionRejectsUntrustedOrigin(t *testing.T) {
	mux, token := sessionTestMux(t)
	rec := httptest.NewRecorder()
	req := authorizedSessionRequest(token, "/api/stations/station_1/sessions", validStartSessionBody())
	req.Header.Set("Origin", "http://evil.example")
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestStartSessionSuccessActivityAndSSE(t *testing.T) {
	manager, token, stationStore, activityStore, broker, sessionStore, persistence := sessionDeps(t)
	mux := sessionMux(manager, stationStore, activityStore, broker, sessionStore, persistence)
	ch, unsub := broker.Subscribe()
	defer unsub()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, authorizedSessionRequest(token, "/api/stations/station_1/sessions", validStartSessionBody()))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(activityStore.Recent(10, "session.started")) != 1 {
		t.Fatal("missing session.started activity")
	}
	select {
	case event := <-ch:
		if event.EventType != "session.started" {
			t.Fatalf("event=%s", event.EventType)
		}
	case <-time.After(time.Second):
		t.Fatal("missing session.started SSE")
	}
}

func TestStartSessionDuplicateBlocked(t *testing.T) {
	mux, token := sessionTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, authorizedSessionRequest(token, "/api/stations/station_1/sessions", validStartSessionBody()))
	if rec.Code != http.StatusCreated {
		t.Fatalf("first status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, authorizedSessionRequest(token, "/api/stations/station_1/sessions", validStartSessionBody()))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "SESSION_ALREADY_ACTIVE") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStartSessionUnknownStation(t *testing.T) {
	mux, token := sessionTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, authorizedSessionRequest(token, "/api/stations/unknown/sessions", validStartSessionBody()))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStartSessionReadinessBlocked(t *testing.T) {
	mux, token := sessionTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, authorizedSessionRequest(token, "/api/stations/station_2/sessions", validStartSessionBody()))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "SESSION_READINESS_BLOCKED") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestEndSessionManual(t *testing.T) {
	mux, token := sessionTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, authorizedSessionRequest(token, "/api/stations/station_1/sessions", validStartSessionBody()))
	if rec.Code != http.StatusCreated {
		t.Fatalf("start status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	start := strings.Index(body, "sess_")
	if start < 0 {
		t.Fatalf("session id missing: %s", body)
	}
	end := start
	for end < len(body) && body[end] != '"' {
		end++
	}
	sessionID := body[start:end]
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, authorizedSessionRequest(token, "/api/sessions/"+sessionID+"/end", `{"reason":"manual"}`))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"locked"`) || !strings.Contains(rec.Body.String(), `"end_reason":"manual"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestListSessionsLocksExpiredPersistedSession(t *testing.T) {
	manager, token, stationStore, activityStore, broker, sessionStore, persistence := sessionDeps(t)
	station := stationStore.List()[0]
	startedAt := time.Now().UTC().Add(-2 * time.Hour)
	if _, err := sessionStore.Start(station, sessions.StartSessionRequest{CustomerName: "Customer", OrderNumber: "ORD-1", TimerSeconds: 60}, t.TempDir(), startedAt); err != nil {
		t.Fatal(err)
	}
	if err := persistence.Save(sessionStore); err != nil {
		t.Fatal(err)
	}
	loaded, err := persistence.LoadOrDefault()
	if err != nil {
		t.Fatal(err)
	}
	mux := sessionMux(manager, stationStore, activityStore, broker, loaded, persistence)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"locked"`) || !strings.Contains(rec.Body.String(), `"end_reason":"timer"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSessionPersistenceRejectsDuplicateActiveSessions(t *testing.T) {
	persistence := sessions.NewPersistence(t.TempDir())
	station := stations.NewStore().List()[0]
	store := sessions.NewStore()
	if _, err := store.Start(station, sessions.StartSessionRequest{CustomerName: "A", OrderNumber: "1", TimerSeconds: 900}, t.TempDir(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Start(station, sessions.StartSessionRequest{CustomerName: "B", OrderNumber: "2", TimerSeconds: 900}, t.TempDir(), time.Now().UTC()); !errors.Is(err, sessions.ErrSessionAlreadyActive) {
		t.Fatalf("err=%v", err)
	}
	if err := persistence.Save(store); err != nil {
		t.Fatal(err)
	}
}

func TestSessionPersistenceRoundtrip(t *testing.T) {
	persistence := sessions.NewPersistence(t.TempDir())
	store := sessions.NewStore()
	station := stations.NewStore().List()[0]
	if _, err := store.Start(station, sessions.StartSessionRequest{CustomerName: "Customer", OrderNumber: "ORD-1", TimerSeconds: 900}, t.TempDir(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := persistence.Save(store); err != nil {
		t.Fatal(err)
	}
	loaded, err := persistence.LoadOrDefault()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.List()) != 1 {
		t.Fatalf("sessions=%d", len(loaded.List()))
	}
}

func sessionTestMux(t *testing.T) (http.Handler, string) {
	t.Helper()
	manager, token, stationStore, activityStore, broker, sessionStore, persistence := sessionDeps(t)
	return sessionMux(manager, stationStore, activityStore, broker, sessionStore, persistence), token
}

func sessionDeps(t *testing.T) (*auth.Manager, string, *stations.Store, *activity.Store, *events.Broker, *sessions.Store, sessions.Persistence) {
	t.Helper()
	manager, err := auth.NewManager("123456")
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := manager.Login("123456")
	if err != nil {
		t.Fatal(err)
	}
	stationStore := stations.NewStore()
	inputDir := t.TempDir()
	localDataDir := t.TempDir()
	if err := os.MkdirAll(localDataDir+"/Customer/ORD-001/station_1", 0o755); err != nil {
		t.Fatal(err)
	}
	lutPath := t.TempDir() + "/default.cube"
	if err := os.WriteFile(lutPath, []byte("TITLE test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := stationStore.Update(stations.Station1ID, stations.UpdateStation{Name: "Station 1", DeviceIdentifier: "Camera 1", InputFolder: inputDir, BackgroundName: "White", DefaultLUTPath: lutPath, OutputRule: "Customer/ORD-001/{station_id}"}); err != nil {
		t.Fatal(err)
	}
	return manager, token, stationStore, activity.NewStore(20), events.NewBroker(), sessions.NewStore(), sessions.NewPersistence(localDataDir)
}

func sessionMux(manager *auth.Manager, stationStore *stations.Store, activityStore *activity.Store, broker *events.Broker, sessionStore *sessions.Store, persistence sessions.Persistence) http.Handler {
	stationPersistence := stations.NewPersistence("")
	return NewMuxWithSessions(NewAuthHandlerWithActivity(manager, activityStore), NewEventsHandler(broker), NewActivityHandler(activityStore), NewPersistentStationsHandler(stationStore, stationPersistence, activityStore, broker), NewReadinessHandler(stationStore, activityStore, broker, persistence.OutputRoot()), NewEventReadinessHandler(stationStore, activityStore, broker, persistence.OutputRoot()), NewStationConfigHandler(stationStore, stationPersistence, activityStore, broker), NewWatchValidationHandler(stationStore, activityStore, broker), NewSessionsHandler(stationStore, sessionStore, persistence, activityStore, broker, persistence.OutputRoot()))
}

func authorizedSessionRequest(token string, path string, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	return req
}

func validStartSessionBody() string {
	return `{"customer_name":"Customer A","order_number":"ORD-001","timer_seconds":900}`
}
