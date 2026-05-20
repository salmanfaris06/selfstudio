package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/auth"
	"selfstudio/agent/internal/cloud"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/sessions"
	"selfstudio/agent/internal/upload"
)

func TestCloudTargetEndpointsRequireAuthAndTrustedOrigin(t *testing.T) {
	manager, _ := auth.NewManager("123456")
	sessionStore := sessions.NewStore()
	targetStore := upload.NewStore()
	handler := NewCloudTargetHandler(sessionStore, upload.Resolver{CloudStore: fakeAPITargetCloudStore{settings: cloud.DefaultSettings()}, Targets: targetStore}, targetStore, activity.NewStore(10), events.NewBroker())
	mux := NewMuxWithCloudTargets(NewAuthHandlerWithActivity(manager, activity.NewStore(10)), NewEventsHandler(events.NewBroker()), NewActivityHandler(activity.NewStore(10)), StationsHandler{}, ReadinessHandler{}, EventReadinessHandler{}, StationConfigHandler{}, WatchValidationHandler{}, SessionsHandler{}, IngestionHandler{}, QuarantineHandler{}, ProcessingQueueHandler{}, PhotoRetryHandler{}, CloudSettingsHandler{}, handler)

	get := httptest.NewRequest(http.MethodGet, "/api/sessions/s1/cloud-target", nil)
	getRecorder := httptest.NewRecorder()
	mux.ServeHTTP(getRecorder, get)
	if getRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("GET without auth = %d", getRecorder.Code)
	}

	post := httptest.NewRequest(http.MethodPost, "/api/sessions/s1/cloud-target/resolve", nil)
	postRecorder := httptest.NewRecorder()
	mux.ServeHTTP(postRecorder, post)
	if postRecorder.Code != http.StatusForbidden && postRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("POST without trusted auth = %d", postRecorder.Code)
	}
}

func TestCloudTargetResolveActiveSessionWaitsForLocalCompletion(t *testing.T) {
	manager, _ := auth.NewManager("123456")
	sessionStore := sessions.NewStore()
	_ = sessionStore.ReplaceAll([]sessions.Session{apiTargetSession("s1", sessions.StatusActive)})
	targetStore := upload.NewStore()
	activityStore := activity.NewStore(10)
	broker := events.NewBroker()
	handler := NewCloudTargetHandler(sessionStore, upload.Resolver{CloudStore: fakeAPITargetCloudStore{settings: authorizedAPICloudSettings()}, Targets: targetStore}, targetStore, activityStore, broker)
	mux := NewMuxWithCloudTargets(NewAuthHandlerWithActivity(manager, activityStore), NewEventsHandler(broker), NewActivityHandler(activityStore), StationsHandler{}, ReadinessHandler{}, EventReadinessHandler{}, StationConfigHandler{}, WatchValidationHandler{}, SessionsHandler{}, IngestionHandler{}, QuarantineHandler{}, ProcessingQueueHandler{}, PhotoRetryHandler{}, CloudSettingsHandler{}, handler)
	rec := authorizedAPIRequest(t, mux, manager, http.MethodPost, "/api/sessions/s1/cloud-target/resolve", nil)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), upload.ActionWaitLocalCompletion) {
		t.Fatalf("expected local completion guard, code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCloudTargetResolveSuccessIdempotentAndSafe(t *testing.T) {
	manager, _ := auth.NewManager("123456")
	sessionStore := sessions.NewStore()
	_ = sessionStore.ReplaceAll([]sessions.Session{apiTargetSession("s1", sessions.StatusLocked)})
	targetStore := upload.NewStore()
	activityStore := activity.NewStore(10)
	broker := events.NewBroker()
	resolver := upload.Resolver{CloudStore: fakeAPITargetCloudStore{settings: authorizedAPICloudSettings()}, Targets: targetStore, DriveFolders: newAPIFakeDriveFolders()}
	handler := NewCloudTargetHandler(sessionStore, resolver, targetStore, activityStore, broker)
	mux := NewMuxWithCloudTargets(NewAuthHandlerWithActivity(manager, activityStore), NewEventsHandler(broker), NewActivityHandler(activityStore), StationsHandler{}, ReadinessHandler{}, EventReadinessHandler{}, StationConfigHandler{}, WatchValidationHandler{}, SessionsHandler{}, IngestionHandler{}, QuarantineHandler{}, ProcessingQueueHandler{}, PhotoRetryHandler{}, CloudSettingsHandler{}, handler)
	rec := authorizedAPIRequest(t, mux, manager, http.MethodPost, "/api/sessions/s1/cloud-target/resolve", nil)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "service_account") || strings.Contains(rec.Body.String(), "private_key") {
		t.Fatalf("resolve unsafe/fail code=%d body=%s", rec.Code, rec.Body.String())
	}
	first, ok := targetStore.Get("s1")
	if !ok || first.Status != upload.StatusReady {
		t.Fatalf("missing ready target: %#v", first)
	}
	rec = authorizedAPIRequest(t, mux, manager, http.MethodPost, "/api/sessions/s1/cloud-target/resolve", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("second resolve code=%d body=%s", rec.Code, rec.Body.String())
	}
	second, _ := targetStore.Get("s1")
	if first.RemoteIdentity != second.RemoteIdentity || second.AttemptCount != 1 {
		t.Fatalf("not idempotent: %#v %#v", first, second)
	}
}

type apiFakeDriveFolders struct{ folders map[string]upload.DriveFolder }

func newAPIFakeDriveFolders() *apiFakeDriveFolders {
	return &apiFakeDriveFolders{folders: map[string]upload.DriveFolder{}}
}
func (f *apiFakeDriveFolders) FindFolder(_ context.Context, parentID, name string) ([]upload.DriveFolder, error) {
	if d, ok := f.folders[parentID+"/"+name]; ok {
		return []upload.DriveFolder{d}, nil
	}
	return nil, nil
}
func (f *apiFakeDriveFolders) CreateFolder(_ context.Context, parentID, name string) (upload.DriveFolder, error) {
	d := upload.DriveFolder{ID: "folder-" + name, Name: name, ParentID: parentID}
	f.folders[parentID+"/"+name] = d
	return d, nil
}
func (f *apiFakeDriveFolders) GetFolder(_ context.Context, folderID string) (upload.DriveFolder, error) {
	return upload.DriveFolder{ID: folderID}, nil
}

type fakeAPITargetCloudStore struct{ settings cloud.Settings }

func (f fakeAPITargetCloudStore) LoadOrDefault() (cloud.Settings, error) { return f.settings, nil }

func authorizedAPICloudSettings() cloud.Settings {
	return cloud.Settings{Provider: cloud.ProviderGoogleDrive, DriveRootFolderID: "drive-root-123", ServiceAccountJSON: "{}", ConnectionStatus: cloud.StatusAuthorized}
}

func apiTargetSession(id, status string) sessions.Session {
	ended := time.Date(2026, 5, 19, 10, 5, 0, 0, time.UTC)
	reason := sessions.EndReasonManual
	s := sessions.Session{SessionID: id, StationID: "station_1", Status: status, CustomerName: "Customer", OrderNumber: "Order", TimerSeconds: 60, StartedAt: time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC), EndsAt: ended, StationSnapshot: sessions.StationSnapshot{OutputFolder: "out"}}
	if status == sessions.StatusLocked {
		s.EndedAt = &ended
		s.EndReason = &reason
	}
	return s
}

func authorizedAPIRequest(t *testing.T, mux http.Handler, manager *auth.Manager, method, path string, body *strings.Reader) *httptest.ResponseRecorder {
	t.Helper()
	if body == nil {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Origin", "http://localhost:3000")
	token, _, err := manager.Login("123456")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "selfstudio_session", Value: token, Path: "/"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}
