package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/auth"
	"selfstudio/agent/internal/cameras"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/stations"
)

type apiFakeTetherStarter struct{ starts int }
type apiFakeProc struct{ wait chan error }

func (p apiFakeProc) Wait() error { return <-p.wait }
func (p apiFakeProc) Kill() error {
	select {
	case p.wait <- nil:
	default:
	}
	return nil
}
func (s *apiFakeTetherStarter) Start(ctx context.Context, spec cameras.TetherCommandSpec, output func(string)) (cameras.TetherProcess, error) {
	s.starts++
	output(`Saving file as D:\secret\station_1_001.JPG`)
	return apiFakeProc{wait: make(chan error, 1)}, nil
}

func TestTetherAPIAuthTrustedOriginAndSuccessWrapper(t *testing.T) {
	mux, token, starter := tetherTestMux(t, true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stations/station_1/tether-listener", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("get unauth=%d", rec.Code)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/stations/station_1/tether-listener/start", nil)
	req.Header.Set("Origin", "http://evil.example")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("origin=%d", rec.Code)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/stations/station_1/tether-listener/start", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("start=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload DataResponse[TetherListenerData]
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Listener.Status != cameras.TetherStatusRunning || starter.starts != 1 {
		t.Fatalf("payload=%+v starts=%d", payload.Data, starter.starts)
	}
	if payload.Data.Settings.DesiredState != cameras.TetherDesiredRunning || payload.Data.Settings.AutoRestartEnabled {
		t.Fatalf("settings should persist running desired and safe auto-restart default: %+v", payload.Data.Settings)
	}
	if strings.Contains(rec.Body.String(), "D:\\") {
		t.Fatalf("raw path leaked: %s", rec.Body.String())
	}
}

func TestTetherAPIDuplicateStartNoop(t *testing.T) {
	mux, token, starter := tetherTestMux(t, true)
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/stations/station_1/tether-listener/start", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	}
	if starter.starts != 1 {
		t.Fatalf("starts=%d", starter.starts)
	}
}

func TestTetherAPIStopNoopAndMissingAssignment(t *testing.T) {
	mux, token, _ := tetherTestMux(t, false)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stations/station_1/tether-listener/start", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing assignment=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "CAMERA_ASSIGNMENT_REQUIRED") {
		t.Fatalf("body=%s", rec.Body.String())
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/stations/station_1/tether-listener/stop", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "stopped") {
		t.Fatalf("stop=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestTetherAPISettingsAndRetryRequireTrustedOrigin(t *testing.T) {
	mux, token, starter := tetherTestMux(t, true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/stations/station_1/tether-listener/settings", bytes.NewBufferString(`{"auto_restart_enabled":true}`))
	req.Header.Set("Origin", "http://evil.example")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("settings origin=%d", rec.Code)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/stations/station_1/tether-listener/settings", bytes.NewBufferString(`{"auto_restart_enabled":true}`))
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"auto_restart_enabled":true`) {
		t.Fatalf("settings=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/stations/station_1/tether-listener/retry", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || starter.starts != 1 {
		t.Fatalf("retry=%d starts=%d body=%s", rec.Code, starter.starts, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "D:\\") || strings.Contains(strings.ToLower(rec.Body.String()), "stdout") {
		t.Fatalf("unsafe retry body: %s", rec.Body.String())
	}
}

func TestTetherAPIStatusAndStopValidateStationScope(t *testing.T) {
	mux, token, _ := tetherTestMux(t, true)
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/stations/station_99/tether-listener"},
		{http.MethodPost, "/api/stations/station_99/tether-listener/stop"},
		{http.MethodGet, "/api/stations/not-a-station/tether-listener"},
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.Header.Set("Origin", "http://localhost:3000")
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s status=%d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), `"data"`) || strings.Contains(rec.Body.String(), "already stopped") {
			t.Fatalf("synthetic success leaked for %s %s: %s", tc.method, tc.path, rec.Body.String())
		}
	}
}

func tetherTestMux(t *testing.T, assign bool) (http.Handler, string, *apiFakeTetherStarter) {
	t.Helper()
	t.Setenv("TMPDIR", t.TempDir())
	manager, _ := auth.NewManager("123456")
	token, _, _ := manager.Login("123456")
	store := stations.NewStore()
	list := store.List()
	for i := range list {
		list[i].InputFolder = t.TempDir()
	}
	if err := store.ReplaceAll(list); err != nil {
		t.Fatal(err)
	}
	if assign {
		list := store.List()
		for i := range list {
			if list[i].StationID == "station_1" {
				list[i].CameraAssignment = &stations.CameraAssignment{IdentityKey: "native_windows|usb:001,004|sony", CameraName: "Sony", Port: "usb:001,004", Runtime: "native_windows", Connected: true}
			}
		}
		if err := store.ReplaceAll(list); err != nil {
			t.Fatal(err)
		}
	}
	activityStore := activity.NewStore(20)
	broker := events.NewBroker()
	camerasHandler := NewCamerasHandler(store, nil, activityStore, broker, cameras.NewDiscoveryService(cameraAPIRunner{}))
	starter := &apiFakeTetherStarter{}
	supervisor := cameras.NewTetherSupervisor(starter)
	stateStore := cameras.NewTetherStateStoreAt(t.TempDir() + "/tether.json")
	recovery := cameras.NewTetherRecoveryCoordinator(supervisor, stateStore, func(stationID string) (cameras.TetherStationConfig, bool) {
		station, err := store.Get(stationID)
		if err != nil {
			return cameras.TetherStationConfig{}, false
		}
		var assignment *cameras.TetherAssignment
		if station.CameraAssignment != nil {
			assignment = &cameras.TetherAssignment{IdentityKey: station.CameraAssignment.IdentityKey, CameraName: station.CameraAssignment.CameraName, Port: station.CameraAssignment.Port, Runtime: cameras.Runtime(station.CameraAssignment.Runtime)}
		}
		return cameras.TetherStationConfig{StationID: station.StationID, InputFolder: station.InputFolder, Assignment: assignment}, true
	}, TetherRecoveryNotifier{Activity: activityStore, Broker: broker})
	recovery.AfterFunc = nil
	handler := NewCameraTetherHandlerWithRecovery(store, supervisor, stateStore, recovery, &camerasHandler)
	mux := http.NewServeMux()
	mux.Handle("GET /api/stations/{station_id}/tether-listener", RequireAuth(manager, http.HandlerFunc(handler.Get)))
	mux.Handle("POST /api/stations/{station_id}/tether-listener/start", RequireTrustedOrigin(RequireAuth(manager, http.HandlerFunc(handler.Start))))
	mux.Handle("POST /api/stations/{station_id}/tether-listener/stop", RequireTrustedOrigin(RequireAuth(manager, http.HandlerFunc(handler.Stop))))
	mux.Handle("POST /api/stations/{station_id}/tether-listener/retry", RequireTrustedOrigin(RequireAuth(manager, http.HandlerFunc(handler.Retry))))
	mux.Handle("PUT /api/stations/{station_id}/tether-listener/settings", RequireTrustedOrigin(RequireAuth(manager, http.HandlerFunc(handler.PutSettings))))
	return mux, token, starter
}
