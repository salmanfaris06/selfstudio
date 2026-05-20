package api

import (
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

type cameraAPIRunner struct {
	responses map[string]cameras.CommandResult
	errors    map[string]error
}

func (r cameraAPIRunner) Run(ctx context.Context, spec cameras.CommandSpec) (cameras.CommandResult, error) {
	key := spec.Name
	if spec.Name == "wsl.exe" {
		key = "wsl"
	}
	if err := r.errors[key]; err != nil {
		return cameras.CommandResult{}, err
	}
	return r.responses[key], nil
}

func TestCameraDiscoveryRequiresAuth(t *testing.T) {
	mux, _ := cameraTestMux(t, cameraAPIRunner{responses: map[string]cameras.CommandResult{"gphoto2": {ExitCode: 0}}})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cameras/gphoto2/discover", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCameraDiscoveryRejectsUntrustedOrigin(t *testing.T) {
	mux, token := cameraTestMux(t, cameraAPIRunner{responses: map[string]cameras.CommandResult{"gphoto2": {ExitCode: 0}}})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cameras/gphoto2/discover", nil)
	req.Header.Set("Origin", "http://evil.example")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestCameraDiscoverySafeErrorShapeAndNoRawCommandLeak(t *testing.T) {
	runner := cameraAPIRunner{errors: map[string]error{"gphoto2": cameras.ErrCommandUnavailable, "wsl": cameras.ErrCommandUnavailable}}
	mux, token := cameraTestMux(t, runner)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cameras/gphoto2/discover", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
	var payload ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if payload.Error.Code != "GPHOTO2_UNAVAILABLE" || payload.Error.Action != string(cameras.ActionInstallGPhoto2) {
		t.Fatalf("error = %+v", payload.Error)
	}
	body := strings.ToLower(rec.Body.String())
	for _, raw := range []string{"gphoto2 --auto-detect", "wsl.exe --", "usbipd bind", "usbipd attach", "powershell", "cmd.exe"} {
		if strings.Contains(body, raw) {
			t.Fatalf("response leaked raw command %q: %s", raw, rec.Body.String())
		}
	}
}

func TestCameraDiscoveryUSBIPDCheckResponseIsSafe(t *testing.T) {
	runner := cameraAPIRunner{
		errors: map[string]error{"gphoto2": cameras.ErrCommandUnavailable},
		responses: map[string]cameras.CommandResult{
			"wsl":    {ExitCode: 0, Stdout: "Model Port\n----------------------------------------------------------\n"},
			"usbipd": {ExitCode: 0, Stdout: "Connected:\n1-4 Sony Camera Not attached\n"},
		},
	}
	mux, token := cameraTestMux(t, runner)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cameras/gphoto2/discover", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var payload DataResponse[GPhoto2DiscoveryData]
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if payload.Data.Status != cameras.DiscoveryStatusUSBIPDCheckNeeded || payload.Data.Action != cameras.ActionCheckUSBIPD {
		t.Fatalf("data = %+v", payload.Data)
	}
}

func TestCameraAssignmentRequiresAuthAndTrustedOrigin(t *testing.T) {
	mux, token := cameraTestMux(t, cameraAPIRunner{})
	body := `{"identity_key":"wsl|usb:001,004|sony","camera_name":"Sony","port":"usb:001,004","runtime":"wsl"}`

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/stations/station_1/camera-assignment", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:3000")
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status = %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/stations/station_1/camera-assignment", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://evil.example")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("untrusted status = %d", rec.Code)
	}
}

func TestCameraAssignmentDuplicateReturnsSafeError(t *testing.T) {
	mux, token := cameraTestMux(t, cameraAPIRunner{})
	body := `{"identity_key":"wsl|usb:001,004|sony","camera_name":"Sony","port":"usb:001,004","runtime":"wsl"}`

	for _, stationID := range []string{"station_1", "station_2"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/stations/"+stationID+"/camera-assignment", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://localhost:3000")
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
		mux.ServeHTTP(rec, req)
		if stationID == "station_1" && rec.Code != http.StatusOK {
			t.Fatalf("first status = %d body=%s", rec.Code, rec.Body.String())
		}
		if stationID == "station_2" {
			if rec.Code != http.StatusConflict {
				t.Fatalf("duplicate status = %d body=%s", rec.Code, rec.Body.String())
			}
			var payload ErrorResponse
			if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload.Error.Code != "DUPLICATE_CAMERA_ASSIGNMENT" || payload.Error.Action != string(cameras.ActionChooseDifferent) {
				t.Fatalf("error = %+v", payload.Error)
			}
		}
	}
}

func cameraTestMux(t *testing.T, runner cameraAPIRunner) (http.Handler, string) {
	t.Helper()
	manager, err := auth.NewManager("123456")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	token, _, err := manager.Login("123456")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	store := stations.NewStore()
	activityStore := activity.NewStore(20)
	broker := events.NewBroker()
	if runner.responses == nil {
		runner.responses = map[string]cameras.CommandResult{}
	}
	if runner.errors == nil {
		runner.errors = map[string]error{}
	}
	handler := NewCamerasHandler(store, nil, activityStore, broker, cameras.NewDiscoveryService(runner))
	mux := http.NewServeMux()
	mux.Handle("POST /api/cameras/gphoto2/discover", RequireTrustedOrigin(RequireAuth(manager, http.HandlerFunc(handler.Discover))))
	mux.Handle("PUT /api/stations/{station_id}/camera-assignment", RequireTrustedOrigin(RequireAuth(manager, http.HandlerFunc(handler.Assign))))
	return mux, token
}
