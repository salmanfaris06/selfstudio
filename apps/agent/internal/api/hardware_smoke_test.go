package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/auth"
	"selfstudio/agent/internal/cameras"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/stations"
)

type apiSmokeVerifier struct{}

func (apiSmokeVerifier) Discover(context.Context, cameras.HardwareSmokeStation) (bool, cameras.SafeAction, string) {
	return true, "", ""
}
func (apiSmokeVerifier) EnsureListener(context.Context, cameras.HardwareSmokeStation) (bool, cameras.SafeAction, string) {
	return true, "", ""
}
func (apiSmokeVerifier) PrepareSession(context.Context, cameras.HardwareSmokeStation, bool) (string, bool, cameras.SafeAction, string) {
	return "session_1", true, "", ""
}
func (apiSmokeVerifier) WaitForNewJPG(context.Context, string, time.Time, time.Duration) (string, error) {
	return "new.jpg", nil
}
func (apiSmokeVerifier) VerifyIngestion(context.Context, string, string, time.Duration) (string, int, error) {
	return "session_1", 1, nil
}
func (apiSmokeVerifier) VerifyLocalOriginal(context.Context, string, string, time.Duration) (string, error) {
	return "original.jpg", nil
}
func (apiSmokeVerifier) VerifyGraded(context.Context, string, string, time.Duration) (string, error) {
	return "graded.jpg", nil
}
func (apiSmokeVerifier) VerifyDrive(context.Context, string, string, time.Duration) (string, bool, error) {
	return "not_configured", false, nil
}

func TestHardwareSmokeAPIRequiresAuthTrustedOriginAndReturnsSafeWrapper(t *testing.T) {
	stationStore := stations.NewStore()
	_, _ = stationStore.UpdateCameraAssignment("station_1", stations.UpdateCameraAssignment{IdentityKey: "id", CameraName: "Sony A6000", Port: "usb:001,004", Runtime: "wsl", Connected: true})
	authManager, err := auth.NewManager("123456")
	if err != nil {
		t.Fatal(err)
	}
	authHandler := NewAuthHandler(authManager)
	handler := NewHardwareSmokeHandler(stationStore, activity.NewStore(20), events.NewBroker(), &cameras.HardwareSmokeRunner{Verifier: apiSmokeVerifier{}, Now: func() time.Time { return time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC) }})

	unauth := httptest.NewRequest(http.MethodPost, "/api/stations/station_1/hardware-smoke-tests", bytes.NewBufferString(`{}`))
	unauth.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	RequireTrustedOrigin(RequireAuth(authManager, http.HandlerFunc(handler.Run))).ServeHTTP(rr, unauth)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauth code=%d", rr.Code)
	}

	login := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"pin":"123456"}`))
	req.Header.Set("Content-Type", "application/json")
	authHandler.Login(login, req)
	cookie := login.Result().Cookies()[0]
	authed := httptest.NewRequest(http.MethodPost, "/api/stations/station_1/hardware-smoke-tests", bytes.NewBufferString(`{"mode":"local_only"}`))
	authed.SetPathValue("station_id", "station_1")
	authed.Header.Set("Content-Type", "application/json")
	authed.Header.Set("Origin", "http://localhost:3000")
	authed.AddCookie(cookie)
	rr = httptest.NewRecorder()
	RequireTrustedOrigin(RequireAuth(authManager, http.HandlerFunc(handler.Run))).ServeHTTP(rr, authed)
	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["data"] == nil {
		t.Fatalf("missing data wrapper: %s", rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "usb:001") {
		t.Fatalf("raw port leaked: %s", rr.Body.String())
	}
}
