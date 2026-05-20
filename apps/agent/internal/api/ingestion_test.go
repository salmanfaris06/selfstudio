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
	"selfstudio/agent/internal/stations"
)

func TestIngestionScanRequiresAuth(t *testing.T) {
	manager, _ := auth.NewManager("123456")
	mux := NewMuxWithIngestion(NewAuthHandlerWithActivity(manager, activity.NewStore(20)), NewEventsHandler(events.NewBroker()), NewActivityHandler(activity.NewStore(20)), StationsHandler{}, ReadinessHandler{}, EventReadinessHandler{}, StationConfigHandler{}, WatchValidationHandler{}, SessionsHandler{}, NewIngestionHandler(ingestion.NewScanner(stations.NewStore()), activity.NewStore(20), events.NewBroker()))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/ingestion/scan", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestIngestionScanDetectsStableJPGOnce(t *testing.T) {
	manager, err := auth.NewManager("123456")
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := manager.Login("123456")
	if err != nil {
		t.Fatal(err)
	}
	stationStore := stations.NewStore()
	input := t.TempDir()
	lut := t.TempDir() + "/x.cube"
	if err := os.WriteFile(lut, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := stationStore.Update(stations.Station1ID, stations.UpdateStation{Name: "Station 1", DeviceIdentifier: "cam", InputFolder: input, BackgroundName: "bg", DefaultLUTPath: lut, OutputRule: "{station_id}"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input+"/A.JPG", []byte("jpg"), 0o644); err != nil {
		t.Fatal(err)
	}
	activityStore := activity.NewStore(20)
	broker := events.NewBroker()
	scanner := ingestion.NewScanner(stationStore)
	mux := NewMuxWithIngestion(NewAuthHandlerWithActivity(manager, activityStore), NewEventsHandler(broker), NewActivityHandler(activityStore), StationsHandler{}, ReadinessHandler{}, EventReadinessHandler{}, StationConfigHandler{}, WatchValidationHandler{}, SessionsHandler{}, NewIngestionHandler(scanner, activityStore, broker))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/ingestion/scan", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "A.JPG") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/ingestion/scan", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"photos":[]`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
