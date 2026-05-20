package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/auth"
	"selfstudio/agent/internal/cloud"
	"selfstudio/agent/internal/events"
)

func cloudMux(t *testing.T, checker cloud.Checker) (http.Handler, string, *events.Broker, *activity.Store) {
	t.Helper()
	m, _ := auth.NewManager("123456")
	token, _, _ := m.Login("123456")
	a := activity.NewStore(20)
	b := events.NewBroker()
	h := NewCloudSettingsHandler(cloud.NewPersistence(t.TempDir()), checker, a, b)
	return NewMuxWithCloudSettings(NewAuthHandler(m), NewEventsHandler(b), NewActivityHandler(a), StationsHandler{}, ReadinessHandler{}, EventReadinessHandler{}, StationConfigHandler{}, WatchValidationHandler{}, SessionsHandler{}, IngestionHandler{}, QuarantineHandler{}, ProcessingQueueHandler{}, PhotoRetryHandler{}, h), token, b, a
}
func authedReq(method, path, body, token string) *http.Request {
	r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	r.Header.Set("Origin", "http://localhost:3000")
	return r
}
func TestCloudSettingsRequiresAuth(t *testing.T) {
	mux, _, _, _ := cloudMux(t, cloud.FakeChecker{})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/cloud/settings", nil))
	if rec.Code != 401 {
		t.Fatalf("status %d", rec.Code)
	}
}
func TestCloudPutRejectsUntrustedOrigin(t *testing.T) {
	mux, token, _, _ := cloudMux(t, cloud.FakeChecker{})
	req := httptest.NewRequest("PUT", "/api/cloud/settings", strings.NewReader(`{}`))
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatalf("status %d", rec.Code)
	}
}
func TestCloudSaveAndGetNoSecret(t *testing.T) {
	mux, token, _, _ := cloudMux(t, cloud.FakeChecker{})
	body := `{"provider":"google_drive","drive_root_folder_id":"drive-root-123","drive_root_folder_name":"Selfstudio Delivery","service_account_json":"{\"private_key\":\"SECRET\"}"}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, authedReq("PUT", "/api/cloud/settings", body, token))
	if rec.Code != 200 {
		t.Fatalf("put %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "SECRET") || strings.Contains(rec.Body.String(), "private_key") {
		t.Fatal("secret leaked")
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, authedReq("GET", "/api/cloud/settings", "", token))
	if strings.Contains(rec.Body.String(), "SECRET") || strings.Contains(rec.Body.String(), "private_key") {
		t.Fatal("secret leaked on get")
	}
}
func TestCloudCheckSuccessPublishesActivity(t *testing.T) {
	mux, token, broker, store := cloudMux(t, cloud.FakeChecker{})
	sub, done := broker.Subscribe()
	defer done()
	body := `{"provider":"google_drive","drive_root_folder_id":"drive-root-123","service_account_json":"{}"}`
	mux.ServeHTTP(httptest.NewRecorder(), authedReq("PUT", "/api/cloud/settings", body, token))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, authedReq("POST", "/api/cloud/settings/check", "", token))
	if rec.Code != 200 {
		t.Fatalf("check %d %s", rec.Code, rec.Body.String())
	}
	ev := <-sub
	if ev.EventType != "cloud.status_updated" {
		t.Fatal(ev.EventType)
	}
	if len(store.Recent(10, "cloud.connection_checked")) == 0 {
		t.Fatal("activity missing")
	}
}
func TestCloudCheckFailureActionableSafe(t *testing.T) {
	mux, token, _, _ := cloudMux(t, cloud.FakeChecker{Result: cloud.CheckResult{Status: cloud.StatusFailed, ErrorCode: "CLOUD_CREDENTIALS_INVALID", ErrorMessage: "Credential cloud tidak valid atau tidak dapat dipakai.", ErrorAction: cloud.ActionFixCredentials}})
	mux.ServeHTTP(httptest.NewRecorder(), authedReq("PUT", "/api/cloud/settings", `{"provider":"google_drive","drive_root_folder_id":"drive-root-123","service_account_json":"SECRET"}`, token))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, authedReq("POST", "/api/cloud/settings/check", "", token))
	if rec.Code != 400 {
		t.Fatalf("status %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "SECRET") {
		t.Fatal("secret leaked")
	}
	var er ErrorResponse
	_ = json.NewDecoder(rec.Body).Decode(&er)
	if er.Error.Action != cloud.ActionFixCredentials {
		t.Fatal(er.Error.Action)
	}
}
