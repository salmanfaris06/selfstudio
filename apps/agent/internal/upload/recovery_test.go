package upload

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/events"
)

type recoveryCountingUploader struct {
	mu    sync.Mutex
	calls map[string]int
	block chan struct{}
}

func (u *recoveryCountingUploader) Upload(ctx context.Context, bucketName, objectKey, localPath string) (UploadResult, error) {
	u.mu.Lock()
	if u.calls == nil {
		u.calls = map[string]int{}
	}
	u.calls[objectKey]++
	u.mu.Unlock()
	if u.block != nil {
		select {
		case <-u.block:
		case <-ctx.Done():
			return UploadResult{}, ctx.Err()
		}
	}
	return UploadResult{RemoteIdentity: "gs://" + bucketName + "/" + objectKey, RemoteGeneration: 7, RemoteETag: "etag"}, nil
}

func (u *recoveryCountingUploader) count(objectKey string) int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.calls[objectKey]
}

type recoveryDriveUploader struct {
	mu     sync.Mutex
	calls  map[string]int
	fileID string
	block  chan struct{}
}

func (u *recoveryDriveUploader) UploadFile(ctx context.Context, folderID string, fileName string, localPath string) (DriveUploadResult, error) {
	u.mu.Lock()
	if u.calls == nil {
		u.calls = map[string]int{}
	}
	u.calls[folderID]++
	u.mu.Unlock()
	if u.block != nil {
		select {
		case <-u.block:
		case <-ctx.Done():
			return DriveUploadResult{}, ctx.Err()
		}
	}
	id := u.fileID
	if id == "" {
		id = "drive-file-" + folderID
	}
	return DriveUploadResult{DriveFileID: id, DriveFolderID: folderID, RemoteETag: "etag"}, nil
}

func (u *recoveryDriveUploader) count(folderID string) int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.calls[folderID]
}

type failingJobsPersistence struct{}

func TestStartupRecoveryNormalizesInterruptedDriveJobAndResumesOnce(t *testing.T) {
	tmp := t.TempDir()
	local := tmp + "/photo.jpg"
	writeTestFile(t, local)
	now := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	store := NewJobsStore()
	job := recoveryDriveTestJob("s1", "p1", AssetKindOriginal, local, JobStatusUploading)
	job.AttemptCount = 1
	if err := store.Upsert(job); err != nil {
		t.Fatal(err)
	}
	drive := &recoveryDriveUploader{fileID: "drive-existing"}
	worker := &Worker{Jobs: store, Persistence: NewJobsPersistence(tmp), DriveUploader: drive, Events: make(chan FileUploadJob, 16)}
	res, err := (StartupRecovery{Worker: worker, Now: func() time.Time { return now }}).Recover(context.Background())
	if err != nil {
		t.Fatalf("recover failed: %v", err)
	}
	if res.Summary.RecoveredPending != 1 || res.Summary.Resumed != 1 {
		t.Fatalf("unexpected summary: %+v", res.Summary)
	}
	waitFor(t, func() bool { return drive.count(job.DriveFolderID) == 1 })
	got, _ := store.Get(job.JobID)
	if got.Status != JobStatusUploaded || got.JobID != job.JobID || got.DedupeKey != job.DedupeKey || got.DriveFolderID != job.DriveFolderID || got.LocalPath != job.LocalPath || got.DriveFileID != "drive-existing" || got.RemoteIdentity != "drive-existing" {
		t.Fatalf("job not uploaded with same Drive identity: %+v", got)
	}
}

func TestStartupRecoveryMissingLocalMarksFailed(t *testing.T) {
	tmp := t.TempDir()
	store := NewJobsStore()
	job := recoveryDriveTestJob("s1", "p1", AssetKindOriginal, tmp+"/missing.jpg", JobStatusPending)
	if err := store.Upsert(job); err != nil {
		t.Fatal(err)
	}
	worker := &Worker{Jobs: store, Persistence: NewJobsPersistence(tmp), DriveUploader: &recoveryDriveUploader{}}
	res, err := (StartupRecovery{Worker: worker}).Recover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.FailedMissingLocal != 1 || res.Summary.Resumed != 0 {
		t.Fatalf("unexpected summary: %+v", res.Summary)
	}
	got, _ := store.Get(job.JobID)
	if got.Status != JobStatusFailed || got.LastErrorCode != ErrorUploadLocalFileMissing || got.LastErrorAction != ActionCheckLocalOutput || got.NextRetryAt != nil {
		t.Fatalf("unexpected missing local mapping: %+v", got)
	}
	if AggregateUploadStatus(SessionCloudTarget{SessionID: "s1", Status: StatusReady}, store.ListBySession("s1")) != SessionUploadFailed {
		t.Fatalf("aggregate status not failed")
	}
}

type recoveryVerifier struct {
	info RemoteObjectInfo
	err  error
}

func (v recoveryVerifier) Stat(ctx context.Context, bucketName, objectKey string) (RemoteObjectInfo, error) {
	return v.info, v.err
}

func TestStartupRecoveryPreservesUploadedWithMetadataAndSkipsNotEligible(t *testing.T) {
	tmp := t.TempDir()
	store := NewJobsStore()
	uploaded := recoveryTestJob("s1", "p1", AssetKindOriginal, tmp+"/missing-ok.jpg", "obj/original/photo.jpg", JobStatusUploaded)
	uploaded.RemoteIdentity = "gs://bucket/obj/original/photo.jpg"
	uploaded.RemoteGeneration = 5
	ne := recoveryTestJob("s1", "p2", AssetKindGraded, "", "", JobStatusNotEligible)
	if err := store.Upsert(uploaded); err != nil {
		t.Fatal(err)
	}
	if err := store.Upsert(ne); err != nil {
		t.Fatal(err)
	}
	worker := &Worker{Jobs: store, Persistence: NewJobsPersistence(tmp), Uploader: &recoveryCountingUploader{}}
	res, err := (StartupRecovery{Worker: worker, Verifier: recoveryVerifier{info: RemoteObjectInfo{RemoteIdentity: uploaded.RemoteIdentity, RemoteGeneration: uploaded.RemoteGeneration}}}).Recover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.VerifiedUploaded != 1 || len(res.EnqueueIDs) != 0 {
		t.Fatalf("unexpected summary: %+v", res.Summary)
	}
	got, _ := store.Get(uploaded.JobID)
	if got.Status != JobStatusUploaded {
		t.Fatalf("uploaded job changed: %+v", got)
	}
}

func TestStartupRecoveryUploadedWithDriveFileIDNoVerifierStaysUploadedAndRepairsRemoteIdentity(t *testing.T) {
	tmp := t.TempDir()
	store := NewJobsStore()
	job := recoveryDriveTestJob("s1", "p1", AssetKindOriginal, tmp+"/photo.jpg", JobStatusUploaded)
	job.DriveFileID = "drive-file-1"
	if err := store.Upsert(job); err != nil {
		t.Fatal(err)
	}
	worker := &Worker{Jobs: store, Persistence: NewJobsPersistence(tmp), DriveUploader: &recoveryDriveUploader{}}
	res, err := (StartupRecovery{Worker: worker}).Recover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.UnverifiedUploaded != 1 || res.Summary.VerifiedUploaded != 0 {
		t.Fatalf("unexpected summary: %+v", res.Summary)
	}
	got, _ := store.Get(job.JobID)
	if got.Status != JobStatusUploaded || got.DriveFileID != "drive-file-1" || got.RemoteIdentity != "drive-file-1" {
		t.Fatalf("uploaded Drive job changed incorrectly: %+v", got)
	}
}

func TestStartupRecoveryUploadedWithoutMetadataRequiresCloudCheck(t *testing.T) {
	tmp := t.TempDir()
	store := NewJobsStore()
	job := recoveryTestJob("s1", "p1", AssetKindOriginal, tmp+"/photo.jpg", "obj/original/photo.jpg", JobStatusUploaded)
	if err := store.Upsert(job); err != nil {
		t.Fatal(err)
	}
	worker := &Worker{Jobs: store, Persistence: NewJobsPersistence(tmp), Uploader: &recoveryCountingUploader{}}
	res, err := (StartupRecovery{Worker: worker}).Recover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.RequiresCloudCheck != 1 {
		t.Fatalf("unexpected summary: %+v", res.Summary)
	}
	got, _ := store.Get(job.JobID)
	if got.Status != JobStatusFailed || got.LastErrorCode != ErrorUploadRemoteCheckNeeded || got.LastErrorAction != ActionCheckDriveFile {
		t.Fatalf("unexpected Drive check state: %+v", got)
	}
}

func TestStartupRecoverySaveFailurePublishesNoSuccessAndDoesNotResume(t *testing.T) {
	tmp := t.TempDir()
	local := tmp + "/photo.jpg"
	writeTestFile(t, local)
	store := NewJobsStore()
	job := recoveryDriveTestJob("s1", "p1", AssetKindOriginal, local, JobStatusUploading)
	if err := store.Upsert(job); err != nil {
		t.Fatal(err)
	}
	broker := events.NewBroker()
	act := activity.NewStore(10)
	drive := &recoveryDriveUploader{}
	if err := os.WriteFile(tmp+"/not-a-dir", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	worker := &Worker{Jobs: store, Persistence: JobsPersistence{path: tmp + "/not-a-dir/upload_jobs.json"}, DriveUploader: drive}
	_, err := (StartupRecovery{Worker: worker, Activity: act, Broker: broker}).Recover(context.Background())
	if err == nil {
		t.Fatal("expected save failure")
	}
	if len(act.Recent(10, "cloud.upload_recovery_completed")) != 0 {
		t.Fatal("success activity published on save failure")
	}
	if drive.count(job.DriveFolderID) != 0 {
		t.Fatal("upload resumed after save failure")
	}
}

func TestStartupRecoveryFutureRetryIsNotResumedAndDueRetryIsResumedOnce(t *testing.T) {
	tmp := t.TempDir()
	localFuture := tmp + "/future.jpg"
	localDue := tmp + "/due.jpg"
	writeTestFile(t, localFuture)
	writeTestFile(t, localDue)
	now := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	due := now.Add(-time.Minute)
	store := NewJobsStore()
	futureJob := recoveryDriveTestJob("s1", "future", AssetKindOriginal, localFuture, JobStatusRetryScheduled)
	futureJob.LastErrorCode = ErrorDriveUploadFailed
	futureJob.LastErrorAction = ActionRetryDriveUpload
	futureJob.AttemptCount = 1
	futureJob.NextRetryAt = &future
	dueJob := recoveryDriveTestJob("s1", "due", AssetKindOriginal, localDue, JobStatusRetryScheduled)
	dueJob.LastErrorCode = ErrorDriveUploadFailed
	dueJob.LastErrorAction = ActionRetryDriveUpload
	dueJob.AttemptCount = 1
	dueJob.NextRetryAt = &due
	if err := store.Upsert(futureJob); err != nil {
		t.Fatal(err)
	}
	if err := store.Upsert(dueJob); err != nil {
		t.Fatal(err)
	}
	drive := &recoveryDriveUploader{}
	worker := &Worker{Jobs: store, Persistence: NewJobsPersistence(tmp), DriveUploader: drive, Events: make(chan FileUploadJob, 16)}
	res, err := (StartupRecovery{Worker: worker, Now: func() time.Time { return now }}).Recover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.Resumed != 1 || len(res.EnqueueIDs) != 1 || res.EnqueueIDs[0] != dueJob.JobID {
		t.Fatalf("unexpected resume set: %+v ids=%v", res.Summary, res.EnqueueIDs)
	}
	waitFor(t, func() bool { return drive.count(dueJob.DriveFolderID) == 1 })
	futureGot, _ := store.Get(futureJob.JobID)
	if futureGot.Status != JobStatusRetryScheduled || futureGot.NextRetryAt == nil || !futureGot.NextRetryAt.Equal(future) {
		t.Fatalf("future retry changed unexpectedly: %+v", futureGot)
	}
}

func TestStartupRecoveryPartialAndNotEligibleAggregatesRemainHonest(t *testing.T) {
	store := NewJobsStore()
	uploaded := recoveryDriveTestJob("s1", "p1", AssetKindOriginal, "unused", JobStatusUploaded)
	uploaded.DriveFileID = "file-1"
	failed := recoveryDriveTestJob("s1", "p2", AssetKindOriginal, "unused", JobStatusFailed)
	ne := recoveryDriveTestJob("s2", "p3", AssetKindGraded, "", JobStatusNotEligible)
	uploaded2 := recoveryDriveTestJob("s2", "p4", AssetKindOriginal, "unused", JobStatusUploaded)
	uploaded2.DriveFileID = "file-2"
	for _, j := range []FileUploadJob{uploaded, failed, ne, uploaded2} {
		if err := store.Upsert(j); err != nil {
			t.Fatal(err)
		}
	}
	if got := AggregateUploadStatus(SessionCloudTarget{SessionID: "s1", Status: StatusReady}, store.ListBySession("s1")); got != SessionUploadPartialFailed {
		t.Fatalf("got %s", got)
	}
	if got := AggregateUploadStatus(SessionCloudTarget{SessionID: "s2", Status: StatusReady}, store.ListBySession("s2")); got != SessionUploadUploaded {
		t.Fatalf("got %s", got)
	}
}

func TestStartupRecoveryAndAutoRetryDueDoNotDoubleResume(t *testing.T) {
	tmp := t.TempDir()
	local := tmp + "/photo.jpg"
	writeTestFile(t, local)
	now := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	store := NewJobsStore()
	job := recoveryDriveTestJob("s1", "p-auto", AssetKindOriginal, local, JobStatusRetryScheduled)
	job.LastErrorCode = ErrorDriveUploadFailed
	job.LastErrorAction = ActionRetryDriveUpload
	job.AttemptCount = 1
	job.NextRetryAt = &now
	if err := store.Upsert(job); err != nil {
		t.Fatal(err)
	}
	block := make(chan struct{})
	drive := &recoveryDriveUploader{block: block}
	worker := &Worker{Jobs: store, Persistence: NewJobsPersistence(tmp), DriveUploader: drive, Events: make(chan FileUploadJob, 16)}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = (StartupRecovery{Worker: worker, Now: func() time.Time { return now }}).Recover(context.Background())
	}()
	go func() {
		defer wg.Done()
		worker.AutoRetryDue(context.Background(), now.Add(5*time.Second))
	}()
	waitFor(t, func() bool { return drive.count(job.DriveFolderID) == 1 })
	close(block)
	wg.Wait()
	if got := drive.count(job.DriveFolderID); got != 1 {
		t.Fatalf("expected recovery and auto retry ticker to resume once, got %d", got)
	}
}

func TestStartupRecoveryDriveVerifierMismatchDoesNotDowngradeUploadedWithDriveFileID(t *testing.T) {
	tmp := t.TempDir()
	store := NewJobsStore()
	job := recoveryDriveTestJob("s1", "p1", AssetKindOriginal, tmp+"/photo.jpg", JobStatusUploaded)
	job.DriveFileID = "drive-file-1"
	job.RemoteIdentity = "drive-file-1"
	if err := store.Upsert(job); err != nil {
		t.Fatal(err)
	}
	worker := &Worker{Jobs: store, Persistence: NewJobsPersistence(tmp), DriveUploader: &recoveryDriveUploader{}}
	res, err := (StartupRecovery{Worker: worker, UploadedVerifier: recoveryUploadedVerifier{info: RemoteObjectInfo{RemoteIdentity: "different-drive-file"}}}).Recover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.UnverifiedUploaded != 1 || res.Summary.RequiresCloudCheck != 0 {
		t.Fatalf("unexpected summary: %+v", res.Summary)
	}
	got, _ := store.Get(job.JobID)
	if got.Status != JobStatusUploaded || got.DriveFileID != "drive-file-1" || got.RemoteIdentity != "drive-file-1" {
		t.Fatalf("valid uploaded Drive job was downgraded: %+v", got)
	}
}

type recoveryUploadedVerifier struct {
	info RemoteObjectInfo
	err  error
}

func (v recoveryUploadedVerifier) VerifyUploaded(ctx context.Context, job FileUploadJob) (RemoteObjectInfo, error) {
	return v.info, v.err
}

func TestStartupRecoveryAndManualRetryUseOneGuard(t *testing.T) {
	tmp := t.TempDir()
	local := tmp + "/photo.jpg"
	writeTestFile(t, local)
	store := NewJobsStore()
	job := recoveryDriveTestJob("s1", "p1", AssetKindOriginal, local, JobStatusPending)
	job.LastErrorCode = ErrorDriveUploadFailed
	job.LastErrorAction = ActionRetryDriveUpload
	if err := store.Upsert(job); err != nil {
		t.Fatal(err)
	}
	block := make(chan struct{})
	drive := &recoveryDriveUploader{block: block}
	worker := &Worker{Jobs: store, Persistence: NewJobsPersistence(tmp), DriveUploader: drive, Events: make(chan FileUploadJob, 16)}
	_, err := (StartupRecovery{Worker: worker}).Recover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, _ = worker.RetryJob(context.Background(), job.JobID, true)
	close(block)
	waitFor(t, func() bool { return drive.count(job.DriveFolderID) == 1 })
	if got := drive.count(job.DriveFolderID); got != 1 {
		t.Fatalf("expected one upload call, got %d", got)
	}
}

func recoveryTestJob(sessionID, photoID, kind, localPath, objectKey, status string) FileUploadJob {
	now := time.Date(2026, 5, 19, 9, 0, 0, 0, time.UTC)
	return FileUploadJob{JobID: JobID(sessionID, photoID, kind), SessionID: sessionID, StationID: "station-1", PhotoID: photoID, AssetKind: kind, LocalPath: localPath, BucketName: "bucket", ObjectKey: objectKey, DedupeKey: JobID(sessionID, photoID, kind), Status: status, MaxAttempts: MaxAutoUploadAttempts, CreatedAt: now, UpdatedAt: now}
}

func recoveryDriveTestJob(sessionID, photoID, kind, localPath, status string) FileUploadJob {
	now := time.Date(2026, 5, 19, 9, 0, 0, 0, time.UTC)
	id := JobID(sessionID, photoID, kind)
	return FileUploadJob{JobID: id, SessionID: sessionID, StationID: "station-1", PhotoID: photoID, AssetKind: kind, LocalPath: localPath, DriveFolderID: "drive-folder-" + kind, DedupeKey: id, Status: status, MaxAttempts: MaxAutoUploadAttempts, CreatedAt: now, UpdatedAt: now}
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("jpg"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ok() {
		t.Fatal("condition not met before timeout")
	}
}
