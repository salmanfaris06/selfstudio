package api

import (
	"encoding/json"
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

func TestStationsListRequiresAuth(t *testing.T) {
	mux := stationTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stations", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestStationsListReturnsThreeStations(t *testing.T) {
	mux, token := stationTestMuxWithToken(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stations", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload DataResponse[StationListData]
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(payload.Data.Stations) != 3 {
		t.Fatalf("len(stations) = %d, want 3", len(payload.Data.Stations))
	}
}

func TestStationUpdateRequiresAuth(t *testing.T) {
	mux := stationTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/stations/station_1", strings.NewReader(validStationUpdateJSON("D:/Selfstudio/input/main")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:3000")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestStationUpdateRejectsUntrustedOrigin(t *testing.T) {
	mux, token := stationTestMuxWithToken(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/stations/station_1", strings.NewReader(validStationUpdateJSON("D:/Selfstudio/input/main")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://evil.example")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestStationUpdateRejectsUnknownStationAndRecordsFailureWithoutStationRef(t *testing.T) {
	manager, token, store, activityStore, broker := stationTestDeps(t)
	mux := NewMuxWithStations(NewAuthHandlerWithActivity(manager, activityStore), NewEventsHandler(broker), NewActivityHandler(activityStore), NewStationsHandler(store, activityStore, broker))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/stations/unknown", strings.NewReader(validStationUpdateJSON("D:/Selfstudio/input/main")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	entries := activityStore.Recent(10, "station.config_update_failed")
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].StationID != nil {
		t.Fatalf("StationID = %+v, want nil", entries[0].StationID)
	}
}

func TestStationUpdateRejectsMalformedAndMultipleJSON(t *testing.T) {
	mux, token := stationTestMuxWithToken(t)
	cases := []string{`{"name":`, `{}` + `{}`}
	for _, body := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/stations/station_1", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://localhost:3000")
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d for body %q", rec.Code, http.StatusBadRequest, body)
		}
	}
}

func TestStationUpdateRejectsNonJSONContentType(t *testing.T) {
	mux, token := stationTestMuxWithToken(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/stations/station_1", strings.NewReader(validStationUpdateJSON("D:/Selfstudio/input/main")))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
	}
}

func TestStationUpdateValidatesRequiredFieldsAndRecordsFailure(t *testing.T) {
	manager, token, store, activityStore, broker := stationTestDeps(t)
	mux := NewMuxWithStations(NewAuthHandlerWithActivity(manager, activityStore), NewEventsHandler(broker), NewActivityHandler(activityStore), NewStationsHandler(store, activityStore, broker))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/stations/station_1", strings.NewReader(`{"name":"","device_identifier":"Sony","input_folder":"D:/input/main","background_name":"White","default_lut_path":"D:/lut.cube","output_rule":"{station_id}"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	entries := activityStore.Recent(10, "station.config_update_failed")
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
}

func TestStationUpdateBlocksDuplicateFolder(t *testing.T) {
	manager, token, store, activityStore, broker := stationTestDeps(t)
	station2 := store.List()[1]
	mux := NewMuxWithStations(NewAuthHandlerWithActivity(manager, activityStore), NewEventsHandler(broker), NewActivityHandler(activityStore), NewStationsHandler(store, activityStore, broker))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/stations/station_1", strings.NewReader(validStationUpdateJSON(station2.InputFolder)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var payload ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if payload.Error.Code != "DUPLICATE_INPUT_FOLDER" {
		t.Fatalf("code = %q", payload.Error.Code)
	}
}

func TestStationUpdateRecordsActivityAndPublishesEvent(t *testing.T) {
	manager, token, store, activityStore, broker := stationTestDeps(t)
	ch, unsubscribe := broker.Subscribe()
	defer unsubscribe()
	mux := NewMuxWithStations(NewAuthHandlerWithActivity(manager, activityStore), NewEventsHandler(broker), NewActivityHandler(activityStore), NewStationsHandler(store, activityStore, broker))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/stations/station_1", strings.NewReader(validStationUpdateJSON("D:/Selfstudio/input/main")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	entries := activityStore.Recent(10, "station.config_updated")
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].StationID == nil || *entries[0].StationID != stations.Station1ID {
		t.Fatalf("StationID = %+v", entries[0].StationID)
	}

	select {
	case event := <-ch:
		if event.EventType != "station.updated" || event.EntityID != stations.Station1ID {
			t.Fatalf("event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for station.updated")
	}
}

func stationTestMux(t *testing.T) http.Handler {
	t.Helper()
	manager, _, store, activityStore, broker := stationTestDeps(t)
	return NewMuxWithStations(NewAuthHandlerWithActivity(manager, activityStore), NewEventsHandler(broker), NewActivityHandler(activityStore), NewStationsHandler(store, activityStore, broker))
}

func stationTestMuxWithToken(t *testing.T) (http.Handler, string) {
	t.Helper()
	manager, token, store, activityStore, broker := stationTestDeps(t)
	return NewMuxWithStations(NewAuthHandlerWithActivity(manager, activityStore), NewEventsHandler(broker), NewActivityHandler(activityStore), NewStationsHandler(store, activityStore, broker)), token
}

func stationTestDeps(t *testing.T) (*auth.Manager, string, *stations.Store, *activity.Store, *events.Broker) {
	t.Helper()
	manager, err := auth.NewManager("123456")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	token, _, err := manager.Login("123456")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	return manager, token, stations.NewStore(), activity.NewStore(20), events.NewBroker()
}

func validStationUpdateJSON(inputFolder string) string {
	payload, _ := json.Marshal(stations.UpdateStation{
		Name:             "Main Camera",
		DeviceIdentifier: "Sony A6000 Main",
		InputFolder:      inputFolder,
		BackgroundName:   "White",
		DefaultLUTPath:   "D:/Selfstudio/luts/default.cube",
		OutputRule:       "{customer_name}/{order_number}/{station_id}",
	})
	return string(payload)
}
