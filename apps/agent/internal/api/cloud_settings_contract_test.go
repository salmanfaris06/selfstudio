package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"selfstudio/agent/internal/cloud"
)

func TestCloudDrivePublicSettingsContractAllowlist(t *testing.T) {
	mux, token, _, _ := cloudMux(t, cloud.FakeChecker{Result: cloud.CheckResult{Status: cloud.StatusFailed, ErrorCode: "DRIVE_CHECK_FAILED", ErrorMessage: "internal detail must stay private", ErrorAction: cloud.ActionRetryCheck}})
	put := performAuthedCloudReq(mux, token, "PUT", "/api/cloud/settings", `{"provider":"google_drive","drive_root_folder_id":"drive-root-123","drive_root_folder_name":"Delivery","service_account_json":"{\"private_key\":\"SECRET\"}"}`)
	if put.Code != http.StatusOK {
		t.Fatalf("put %d %s", put.Code, put.Body.String())
	}
	check := performAuthedCloudReq(mux, token, "POST", "/api/cloud/settings/check", "")
	if check.Code != http.StatusBadRequest {
		t.Fatalf("check %d %s", check.Code, check.Body.String())
	}
	get := performAuthedCloudReq(mux, token, "GET", "/api/cloud/settings", "")
	if get.Code != http.StatusOK {
		t.Fatalf("get %d %s", get.Code, get.Body.String())
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.NewDecoder(get.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"provider":               true,
		"drive_root_folder_id":   true,
		"drive_root_folder_name": true,
		"folder_naming_template": true,
		"credentials_configured": true,
		"connection_status":      true,
		"last_checked_at":        true,
		"last_error_code":        true,
		"last_error_action":      true,
	}
	for key := range envelope.Data {
		if !allowed[key] {
			t.Fatalf("unexpected public settings key %q in %+v", key, envelope.Data)
		}
	}
	for _, forbidden := range []string{"bucket_name", "target_root_prefix", "object_naming_template", "last_error", "service_account_json", "credential_file_path"} {
		if _, ok := envelope.Data[forbidden]; ok {
			t.Fatalf("forbidden key %q leaked in public settings: %+v", forbidden, envelope.Data)
		}
	}
}

func TestDriveFolderRuleInvalidActionsAreDriveSpecific(t *testing.T) {
	mux, token, _, _ := cloudMux(t, cloud.FakeChecker{})
	preview := performAuthedCloudReq(mux, token, "POST", "/api/cloud/settings/folder-preview", `{"customer_name":"../secret","order_number":"SO 001","station_id":"station-1","session_id":"sess-1"}`)
	assertErrorAction(t, preview, cloud.ActionFixRules)

	put := performAuthedCloudReq(mux, token, "PUT", "/api/cloud/settings", `{"provider":"google_drive","drive_root_folder_id":"drive-root-123","drive_root_folder_name":"../secret","service_account_json":"{}"}`)
	assertErrorAction(t, put, cloud.ActionFixRules)
}

func assertErrorAction(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Error struct {
			Action string `json:"action"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error.Action != want {
		t.Fatalf("action = %q, want %q; body %s", envelope.Error.Action, want, rec.Body.String())
	}
}
