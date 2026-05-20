package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"selfstudio/agent/internal/auth"
	"selfstudio/agent/internal/cameras"
	"selfstudio/agent/internal/stations"
)

type apiFakeCaptureRunner struct{}

func (apiFakeCaptureRunner) Capture(ctx context.Context, spec cameras.TestCaptureCommandSpec) error {
	return os.WriteFile(spec.Filename, []byte{0xFF, 0xD8, 0xFF, 0xE0, 1}, 0o644)
}

func TestCameraTestCaptureRequiresAuth(t *testing.T) {
	mux, _ := cameraTestCaptureMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stations/station_1/camera-test-capture", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCameraTestCaptureRejectsUntrustedOrigin(t *testing.T) {
	mux, token := cameraTestCaptureMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stations/station_1/camera-test-capture", nil)
	req.Header.Set("Origin", "http://evil.example")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestCameraTestCaptureSuccessWrapperAndStationScope(t *testing.T) {
	mux, token := cameraTestCaptureMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stations/station_1/camera-test-capture", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload DataResponse[CameraTestCaptureData]
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if payload.Data.TestCapture.StationID != "station_1" || payload.Data.TestCapture.Status != cameras.TestCaptureSuccess {
		t.Fatalf("payload = %+v", payload.Data.TestCapture)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/stations/station_missing/camera-test-capture", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown station status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCameraTestCaptureErrorDetailsAreSafeAllowlist(t *testing.T) {
	mux, token := cameraTestCaptureMuxWithoutAssignment(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stations/station_1/camera-test-capture", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if _, exists := payload.Error.Details["test_capture"]; exists {
		t.Fatalf("details must not expose full test_capture: %+v", payload.Error.Details)
	}
	for _, key := range []string{"status", "action"} {
		if _, exists := payload.Error.Details[key]; !exists {
			t.Fatalf("missing safe detail %q: %+v", key, payload.Error.Details)
		}
	}
	body := strings.ToLower(rec.Body.String())
	for _, raw := range []string{"input_folder", "identity_key", "usb:001,002", "gphoto2 --", "stderr"} {
		if strings.Contains(body, raw) {
			t.Fatalf("response leaked raw internals %q: %s", raw, rec.Body.String())
		}
	}
}

func cameraTestCaptureMux(t *testing.T) (http.Handler, string) {
	t.Helper()
	store := stations.NewStore()
	station, err := store.Get("station_1")
	if err != nil {
		t.Fatal(err)
	}
	station.InputFolder = t.TempDir()
	station.CameraAssignment = &stations.CameraAssignment{IdentityKey: "native_windows|safe-camera", CameraName: "Sony", Port: "usb:001,002", Runtime: "native_windows"}
	if _, err := store.Update(station.StationID, stations.UpdateStation{Name: station.Name, DeviceIdentifier: station.DeviceIdentifier, InputFolder: station.InputFolder, BackgroundName: station.BackgroundName, DefaultLUTPath: station.DefaultLUTPath, OutputRule: station.OutputRule, CameraAssignment: station.CameraAssignment}); err != nil {
		t.Fatal(err)
	}
	return cameraTestCaptureMuxWithStore(t, store)
}

func cameraTestCaptureMuxWithoutAssignment(t *testing.T) (http.Handler, string) {
	t.Helper()
	store := stations.NewStore()
	station, err := store.Get("station_1")
	if err != nil {
		t.Fatal(err)
	}
	station.InputFolder = t.TempDir()
	station.CameraAssignment = nil
	if _, err := store.Update(station.StationID, stations.UpdateStation{Name: station.Name, DeviceIdentifier: station.DeviceIdentifier, InputFolder: station.InputFolder, BackgroundName: station.BackgroundName, DefaultLUTPath: station.DefaultLUTPath, OutputRule: station.OutputRule, CameraAssignment: station.CameraAssignment}); err != nil {
		t.Fatal(err)
	}
	return cameraTestCaptureMuxWithStore(t, store)
}

func cameraTestCaptureMuxWithStore(t *testing.T, store *stations.Store) (http.Handler, string) {
	t.Helper()
	manager, err := auth.NewManager("123456")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	token, _, err := manager.Login("123456")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	handler := NewCameraTestCaptureHandler(store, nil, nil, cameras.NewTetherSupervisor(nil), cameras.NewTestCaptureService(apiFakeCaptureRunner{}))
	mux := http.NewServeMux()
	mux.Handle("POST /api/stations/{station_id}/camera-test-capture", RequireTrustedOrigin(RequireAuth(manager, http.HandlerFunc(handler.Run))))
	return mux, token
}
