package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/auth"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/sessions"
	"selfstudio/agent/internal/upload"
)

func TestSessionUploadRetryEndpointsAuthTrustedAndSuccess(t *testing.T) {
	manager, _ := auth.NewManager("123456")
	sessionStore := sessions.NewStore()
	_ = sessionStore.ReplaceAll([]sessions.Session{apiTargetSession("s1", sessions.StatusLocked)})
	jobs := upload.NewJobsStore()
	targets := upload.NewStore()
	_ = targets.Upsert(readyUploadAPITarget("s1"))
	local := writeUploadTestFile(t, "retry.jpg")
	job := upload.FileUploadJob{JobID: upload.JobID("s1", "p1", upload.AssetKindOriginal), SessionID: "s1", StationID: "station_1", PhotoID: "p1", AssetKind: upload.AssetKindOriginal, LocalPath: local, BucketName: "selfstudio-bucket", ObjectKey: "root/retry.jpg", DedupeKey: upload.JobID("s1", "p1", upload.AssetKindOriginal), Status: upload.JobStatusFailed, AttemptCount: 2, LastErrorCode: upload.ErrorUploadFailed, LastErrorAction: upload.ActionRetryCloudUpload, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := jobs.Upsert(job); err != nil {
		t.Fatal(err)
	}
	worker := upload.Worker{Sessions: sessionStore, Targets: targets, Jobs: jobs, Persistence: upload.NewJobsPersistence(t.TempDir()), Uploader: fakeSessionUploadUploader{}, Events: make(chan upload.FileUploadJob, 10)}
	handler := NewSessionUploadsHandler(sessionStore, targets, jobs, &worker, activity.NewStore(20), events.NewBroker())
	mux := NewMuxWithUploads(NewAuthHandlerWithActivity(manager, activity.NewStore(10)), NewEventsHandler(events.NewBroker()), NewActivityHandler(activity.NewStore(10)), StationsHandler{}, ReadinessHandler{}, EventReadinessHandler{}, StationConfigHandler{}, WatchValidationHandler{}, SessionsHandler{}, IngestionHandler{}, QuarantineHandler{}, ProcessingQueueHandler{}, PhotoRetryHandler{}, CloudSettingsHandler{}, CloudTargetHandler{}, handler)

	unauthReq := httptest.NewRequest(http.MethodPost, "/api/uploads/"+job.JobID+"/retry", nil)
	unauth := httptest.NewRecorder()
	mux.ServeHTTP(unauth, unauthReq)
	if unauth.Code != http.StatusForbidden && unauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected auth/trusted rejection, got %d", unauth.Code)
	}
	rec := authorizedAPIRequest(t, mux, manager, http.MethodPost, "/api/uploads/"+job.JobID+"/retry", nil)
	if rec.Code != http.StatusAccepted || strings.Contains(rec.Body.String(), "private_key") || strings.Contains(rec.Body.String(), local) || strings.Contains(rec.Body.String(), "local_path") {
		t.Fatalf("expected retry accepted with safe payload, code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSessionUploadRetryEndpointNotFoundAndAlreadyUploaded(t *testing.T) {
	manager, _ := auth.NewManager("123456")
	jobs := upload.NewJobsStore()
	worker := upload.Worker{Jobs: jobs, Persistence: upload.NewJobsPersistence(t.TempDir()), Uploader: fakeSessionUploadUploader{}, Events: make(chan upload.FileUploadJob, 10)}
	handler := NewSessionUploadsHandler(sessions.NewStore(), upload.NewStore(), jobs, &worker, activity.NewStore(20), events.NewBroker())
	mux := NewMuxWithUploads(NewAuthHandlerWithActivity(manager, activity.NewStore(10)), NewEventsHandler(events.NewBroker()), NewActivityHandler(activity.NewStore(10)), StationsHandler{}, ReadinessHandler{}, EventReadinessHandler{}, StationConfigHandler{}, WatchValidationHandler{}, SessionsHandler{}, IngestionHandler{}, QuarantineHandler{}, ProcessingQueueHandler{}, PhotoRetryHandler{}, CloudSettingsHandler{}, CloudTargetHandler{}, handler)
	rec := authorizedAPIRequest(t, mux, manager, http.MethodPost, "/api/uploads/missing/retry", nil)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), upload.ErrorUploadJobNotFound) {
		t.Fatalf("expected not found, code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSessionUploadRetryConcurrentRequestsShareGuard(t *testing.T) {
	manager, _ := auth.NewManager("123456")
	jobs := upload.NewJobsStore()
	local := writeUploadTestFile(t, "retry-concurrent.jpg")
	job := upload.FileUploadJob{JobID: upload.JobID("s1", "p1", upload.AssetKindOriginal), SessionID: "s1", StationID: "station_1", PhotoID: "p1", AssetKind: upload.AssetKindOriginal, LocalPath: local, BucketName: "selfstudio-bucket", ObjectKey: "root/retry-concurrent.jpg", DedupeKey: upload.JobID("s1", "p1", upload.AssetKindOriginal), Status: upload.JobStatusFailed, AttemptCount: 1, LastErrorCode: upload.ErrorUploadFailed, LastErrorAction: upload.ActionRetryCloudUpload, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := jobs.Upsert(job); err != nil {
		t.Fatal(err)
	}
	uploader := &blockingRetryUploader{release: make(chan struct{})}
	worker := upload.Worker{Jobs: jobs, Persistence: upload.NewJobsPersistence(t.TempDir()), Uploader: uploader, Events: make(chan upload.FileUploadJob, 20)}
	handler := NewSessionUploadsHandler(sessions.NewStore(), upload.NewStore(), jobs, &worker, activity.NewStore(20), events.NewBroker())
	mux := NewMuxWithUploads(NewAuthHandlerWithActivity(manager, activity.NewStore(10)), NewEventsHandler(events.NewBroker()), NewActivityHandler(activity.NewStore(10)), StationsHandler{}, ReadinessHandler{}, EventReadinessHandler{}, StationConfigHandler{}, WatchValidationHandler{}, SessionsHandler{}, IngestionHandler{}, QuarantineHandler{}, ProcessingQueueHandler{}, PhotoRetryHandler{}, CloudSettingsHandler{}, CloudTargetHandler{}, handler)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = authorizedAPIRequest(t, mux, manager, http.MethodPost, "/api/uploads/"+job.JobID+"/retry", nil)
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(uploader.release)
	wg.Wait()
	if uploader.calls != 1 {
		t.Fatalf("expected exactly one upload call, got %d", uploader.calls)
	}
}

type blockingRetryUploader struct {
	mu      sync.Mutex
	calls   int
	release chan struct{}
}

func (u *blockingRetryUploader) Upload(_ context.Context, bucketName, objectKey, _ string) (upload.UploadResult, error) {
	u.mu.Lock()
	u.calls++
	u.mu.Unlock()
	<-u.release
	return upload.UploadResult{RemoteIdentity: "gs://" + bucketName + "/" + objectKey}, nil
}
