package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/auth"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/sessions"
	"selfstudio/agent/internal/upload"
)

func TestSessionUploadsEndpointsRequireAuthAndTrustedOrigin(t *testing.T) {
	manager, _ := auth.NewManager("123456")
	handler := NewSessionUploadsHandler(sessions.NewStore(), upload.NewStore(), upload.NewJobsStore(), &upload.Worker{}, activity.NewStore(10), events.NewBroker())
	mux := NewMuxWithUploads(NewAuthHandlerWithActivity(manager, activity.NewStore(10)), NewEventsHandler(events.NewBroker()), NewActivityHandler(activity.NewStore(10)), StationsHandler{}, ReadinessHandler{}, EventReadinessHandler{}, StationConfigHandler{}, WatchValidationHandler{}, SessionsHandler{}, IngestionHandler{}, QuarantineHandler{}, ProcessingQueueHandler{}, PhotoRetryHandler{}, CloudSettingsHandler{}, CloudTargetHandler{}, handler)

	get := httptest.NewRequest(http.MethodGet, "/api/sessions/s1/uploads", nil)
	getRecorder := httptest.NewRecorder()
	mux.ServeHTTP(getRecorder, get)
	if getRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("GET without auth = %d", getRecorder.Code)
	}

	post := httptest.NewRequest(http.MethodPost, "/api/sessions/s1/uploads/start", nil)
	postRecorder := httptest.NewRecorder()
	mux.ServeHTTP(postRecorder, post)
	if postRecorder.Code != http.StatusForbidden && postRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("POST without trusted auth = %d", postRecorder.Code)
	}
}

func TestSessionUploadsStartTargetNotReadyAndLocalPending(t *testing.T) {
	manager, _ := auth.NewManager("123456")
	sessionStore := sessions.NewStore()
	_ = sessionStore.ReplaceAll([]sessions.Session{apiTargetSession("s1", sessions.StatusLocked), apiTargetSession("s2", sessions.StatusActive)})
	photoStore := photos.NewStore()
	jobs := upload.NewJobsStore()
	targets := upload.NewStore()
	worker := upload.Worker{Sessions: sessionStore, Photos: photoStore, Targets: targets, Jobs: jobs, Persistence: upload.NewJobsPersistence(t.TempDir()), Uploader: fakeSessionUploadUploader{}}
	handler := NewSessionUploadsHandler(sessionStore, targets, jobs, &worker, activity.NewStore(10), events.NewBroker())
	mux := NewMuxWithUploads(NewAuthHandlerWithActivity(manager, activity.NewStore(10)), NewEventsHandler(events.NewBroker()), NewActivityHandler(activity.NewStore(10)), StationsHandler{}, ReadinessHandler{}, EventReadinessHandler{}, StationConfigHandler{}, WatchValidationHandler{}, SessionsHandler{}, IngestionHandler{}, QuarantineHandler{}, ProcessingQueueHandler{}, PhotoRetryHandler{}, CloudSettingsHandler{}, CloudTargetHandler{}, handler)

	rec := authorizedAPIRequest(t, mux, manager, http.MethodPost, "/api/sessions/s1/uploads/start", nil)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), upload.ErrorCloudTargetNotReady) || strings.Contains(rec.Body.String(), "private_key") {
		t.Fatalf("expected target-not-ready safe error, code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = authorizedAPIRequest(t, mux, manager, http.MethodPost, "/api/sessions/s2/uploads/start", nil)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), upload.ErrorUploadPendingLocalCompletion) {
		t.Fatalf("expected local completion error, code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSessionUploadsStartSuccessAndGetStatusSafe(t *testing.T) {
	manager, _ := auth.NewManager("123456")
	sessionStore := sessions.NewStore()
	_ = sessionStore.ReplaceAll([]sessions.Session{apiTargetSession("s1", sessions.StatusLocked)})
	photoStore := photos.NewStore()
	original := writeUploadTestFile(t, "original.jpg")
	_ = photoStore.ReplaceAll([]photos.Photo{{PhotoID: "p1", StationID: "station_1", SessionID: "s1", SourcePath: original, SourceSizeBytes: 10, DetectedAt: time.Now(), StableAt: time.Now(), RoutedAt: time.Now(), Status: photos.StatusRouted, LocalOriginalPath: original, OriginalSaveStatus: photos.OriginalStatusSaved, GradedProcessingStatus: photos.GradedStatusFailed}})
	jobs := upload.NewJobsStore()
	targets := upload.NewStore()
	_ = targets.Upsert(readyUploadAPITarget("s1"))
	worker := upload.Worker{Sessions: sessionStore, Photos: photoStore, Targets: targets, Jobs: jobs, Persistence: upload.NewJobsPersistence(t.TempDir()), Uploader: fakeSessionUploadUploader{}, Events: make(chan upload.FileUploadJob, 10)}
	handler := NewSessionUploadsHandler(sessionStore, targets, jobs, &worker, activity.NewStore(20), events.NewBroker())
	mux := NewMuxWithUploads(NewAuthHandlerWithActivity(manager, activity.NewStore(10)), NewEventsHandler(events.NewBroker()), NewActivityHandler(activity.NewStore(10)), StationsHandler{}, ReadinessHandler{}, EventReadinessHandler{}, StationConfigHandler{}, WatchValidationHandler{}, SessionsHandler{}, IngestionHandler{}, QuarantineHandler{}, ProcessingQueueHandler{}, PhotoRetryHandler{}, CloudSettingsHandler{}, CloudTargetHandler{}, handler)

	rec := authorizedAPIRequest(t, mux, manager, http.MethodPost, "/api/sessions/s1/uploads/start", nil)
	if rec.Code != http.StatusAccepted || strings.Contains(rec.Body.String(), "private_key") || strings.Contains(rec.Body.String(), "service_account") {
		t.Fatalf("expected safe start success, code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = authorizedAPIRequest(t, mux, manager, http.MethodGet, "/api/sessions/s1/uploads", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "original") || strings.Contains(rec.Body.String(), "private_key") {
		t.Fatalf("expected safe get status, code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSessionUploadsStartStateSaveFailure(t *testing.T) {
	manager, _ := auth.NewManager("123456")
	sessionStore := sessions.NewStore()
	_ = sessionStore.ReplaceAll([]sessions.Session{apiTargetSession("s1", sessions.StatusLocked)})
	photoStore := photos.NewStore()
	jobs := upload.NewJobsStore()
	targets := upload.NewStore()
	_ = targets.Upsert(readyUploadAPITarget("s1"))
	badRoot := writeUploadTestFile(t, "not-a-dir")
	worker := upload.Worker{Sessions: sessionStore, Photos: photoStore, Targets: targets, Jobs: jobs, Persistence: upload.NewJobsPersistence(badRoot), Uploader: fakeSessionUploadUploader{}}
	handler := NewSessionUploadsHandler(sessionStore, targets, jobs, &worker, activity.NewStore(10), events.NewBroker())
	mux := NewMuxWithUploads(NewAuthHandlerWithActivity(manager, activity.NewStore(10)), NewEventsHandler(events.NewBroker()), NewActivityHandler(activity.NewStore(10)), StationsHandler{}, ReadinessHandler{}, EventReadinessHandler{}, StationConfigHandler{}, WatchValidationHandler{}, SessionsHandler{}, IngestionHandler{}, QuarantineHandler{}, ProcessingQueueHandler{}, PhotoRetryHandler{}, CloudSettingsHandler{}, CloudTargetHandler{}, handler)

	rec := authorizedAPIRequest(t, mux, manager, http.MethodPost, "/api/sessions/s1/uploads/start", nil)
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), upload.ErrorUploadStateSaveFailed) {
		t.Fatalf("expected state save failure, code=%d body=%s", rec.Code, rec.Body.String())
	}
}

type fakeSessionUploadUploader struct{}

func (fakeSessionUploadUploader) Upload(_ context.Context, bucketName, objectKey, _ string) (upload.UploadResult, error) {
	return upload.UploadResult{RemoteIdentity: "gs://" + bucketName + "/" + objectKey}, nil
}

func writeUploadTestFile(t *testing.T, name string) string {
	t.Helper()
	path := t.TempDir() + string(os.PathSeparator) + name
	if err := os.WriteFile(path, []byte("jpg"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readyUploadAPITarget(sessionID string) upload.SessionCloudTarget {
	now := time.Now().UTC()
	objectPrefix := "root/2026/05/19/customer/order/station_1/" + sessionID
	return upload.SessionCloudTarget{SessionID: sessionID, StationID: "station_1", BucketName: "selfstudio-bucket", ObjectPrefix: objectPrefix, RemoteIdentity: "gs://selfstudio-bucket/" + objectPrefix, Status: upload.StatusReady, CreatedAt: now, UpdatedAt: now}
}
