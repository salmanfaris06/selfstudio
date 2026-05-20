package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/auth"
	"selfstudio/agent/internal/cloud"
	"selfstudio/agent/internal/events"
)

type failingCloudStore struct {
	settings cloud.Settings
	loadErr  error
	saveErr  error
}

func (s *failingCloudStore) LoadOrDefault() (cloud.Settings, error) {
	if s.loadErr != nil {
		return cloud.Settings{}, s.loadErr
	}
	if s.settings.Provider == "" {
		s.settings = cloud.DefaultSettings()
		s.settings.BucketName = "my-bucket"
		s.settings.ServiceAccountJSON = "SECRET-CREDENTIAL"
	}
	return s.settings, nil
}

func (s *failingCloudStore) Save(settings cloud.Settings) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.settings = settings
	return nil
}

func muxWithCloudStore(t *testing.T, store cloudSettingsStore, checker cloud.Checker, activityStore *activity.Store, broker *events.Broker) (http.Handler, string) {
	t.Helper()
	m, _ := auth.NewManager("123456")
	token, _, _ := m.Login("123456")
	h := NewCloudSettingsHandler(store, checker, activityStore, broker)
	return NewMuxWithCloudSettings(NewAuthHandler(m), NewEventsHandler(broker), NewActivityHandler(activityStore), StationsHandler{}, ReadinessHandler{}, EventReadinessHandler{}, StationConfigHandler{}, WatchValidationHandler{}, SessionsHandler{}, IngestionHandler{}, QuarantineHandler{}, ProcessingQueueHandler{}, PhotoRetryHandler{}, h), token
}

func TestCloudCheckPersistFailureDoesNotPublishSuccessOrActivity(t *testing.T) {
	store := &failingCloudStore{saveErr: errors.New("disk full")}
	broker := events.NewBroker()
	activityStore := activity.NewStore(20)
	mux, token := muxWithCloudStore(t, store, cloud.FakeChecker{}, activityStore, broker)
	sub, done := broker.Subscribe()
	defer done()

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, authedReq("POST", "/api/cloud/settings/check", "", token))

	if rec.Code != 500 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "CLOUD_CONFIG_SAVE_FAILED") {
		t.Fatalf("expected save failure code, got %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "SECRET-CREDENTIAL") {
		t.Fatal("secret leaked in save failure response")
	}
	select {
	case ev := <-sub:
		t.Fatalf("unexpected event published after failed persistence: %s", ev.EventType)
	default:
	}
	if got := activityStore.Recent(10, "cloud.connection_checked"); len(got) != 0 {
		t.Fatalf("unexpected activity after failed persistence: %d", len(got))
	}
}

func TestCloudPutLoadFailureReturnsSafeReadError(t *testing.T) {
	store := &failingCloudStore{loadErr: errors.New("corrupt json containing SECRET")}
	broker := events.NewBroker()
	activityStore := activity.NewStore(20)
	mux, token := muxWithCloudStore(t, store, cloud.FakeChecker{}, activityStore, broker)
	sub, done := broker.Subscribe()
	defer done()

	body := `{"provider":"google_drive","drive_root_folder_id":"drive-root-123","service_account_json":"NEW-SECRET"}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, authedReq("PUT", "/api/cloud/settings", body, token))

	if rec.Code != 500 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "CLOUD_CONFIG_READ_FAILED") {
		t.Fatalf("expected read failure code, got %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "SECRET") || strings.Contains(rec.Body.String(), "corrupt json") {
		t.Fatalf("unsafe detail leaked in read failure response: %s", rec.Body.String())
	}
	select {
	case ev := <-sub:
		t.Fatalf("unexpected event published after failed load: %s", ev.EventType)
	default:
	}
	if got := activityStore.Recent(10, "cloud.settings_updated"); len(got) != 0 {
		t.Fatalf("unexpected activity after failed load: %d", len(got))
	}
}
