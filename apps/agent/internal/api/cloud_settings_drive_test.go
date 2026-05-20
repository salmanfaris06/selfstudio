package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"selfstudio/agent/internal/cloud"
)

func performAuthedCloudReq(mux http.Handler, token, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, authedReq(method, path, body, token))
	return rec
}

func TestCloudDriveSaveAndGetSafeMetadata(t *testing.T) {
	mux, token, _, _ := cloudMux(t, cloud.FakeChecker{})
	body := `{"provider":"google_drive","drive_root_folder_id":"drive-root-123","drive_root_folder_name":"Selfstudio Delivery","service_account_json":"{\"private_key\":\"SECRET\"}"}`
	rec := performAuthedCloudReq(mux, token, "PUT", "/api/cloud/settings", body)
	if rec.Code != 200 {
		t.Fatalf("put %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "SECRET") || strings.Contains(rec.Body.String(), "private_key") {
		t.Fatal("secret leaked in put response")
	}
	var envelope struct {
		Data cloud.PublicSettings `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Provider != cloud.ProviderGoogleDrive || envelope.Data.DriveRootFolderID != "drive-root-123" || envelope.Data.FolderNamingTemplate == "" {
		t.Fatalf("unexpected public settings: %+v", envelope.Data)
	}

	rec = performAuthedCloudReq(mux, token, "GET", "/api/cloud/settings", "")
	if rec.Code != 200 {
		t.Fatalf("get %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "SECRET") || strings.Contains(rec.Body.String(), "private_key") {
		t.Fatal("secret leaked in get response")
	}
}

func TestCloudDrivePreviewFolderPath(t *testing.T) {
	mux, token, _, _ := cloudMux(t, cloud.FakeChecker{})
	body := `{"customer_name":"Customer Demo","order_number":"SO 001","station_id":"station-1","session_id":"sess-demo","asset_kind":"original","file_name":"IMG 001.JPG"}`
	rec := performAuthedCloudReq(mux, token, "POST", "/api/cloud/settings/folder-preview", body)
	if rec.Code != 200 {
		t.Fatalf("preview %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "customer-demo/so-001/station-1/sess-demo") || strings.Contains(rec.Body.String(), "object_key") {
		t.Fatalf("unexpected preview body: %s", rec.Body.String())
	}
}
