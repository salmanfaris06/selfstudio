package upload

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"
)

type countingUploader struct {
	mu    sync.Mutex
	calls int
	block chan struct{}
}

func (u *countingUploader) Upload(ctx context.Context, bucketName, objectKey, localPath string) (UploadResult, error) {
	u.mu.Lock()
	u.calls++
	u.mu.Unlock()
	if u.block != nil {
		<-u.block
	}
	return UploadResult{RemoteIdentity: "gs://" + bucketName + "/" + objectKey, RemoteGeneration: 7, RemoteETag: "etag"}, nil
}
func (u *countingUploader) Calls() int { u.mu.Lock(); defer u.mu.Unlock(); return u.calls }

func TestRetryRunnerDuplicateGuardPreventsParallelUpload(t *testing.T) {
	w, sessionID, _, _ := testWorker(t, nil, true)
	u := &countingUploader{block: make(chan struct{})}
	w.Uploader = u
	now := time.Now().UTC()
	job := FileUploadJob{JobID: JobID(sessionID, "p1", AssetKindOriginal), SessionID: sessionID, StationID: "station_1", PhotoID: "p1", AssetKind: AssetKindOriginal, LocalPath: mustTempFile(t), BucketName: "bucket", ObjectKey: "root/file.jpg", DedupeKey: JobID(sessionID, "p1", AssetKindOriginal), Status: JobStatusFailed, AttemptCount: 1, LastErrorCode: ErrorUploadFailed, LastErrorAction: ActionRetryCloudUpload, CreatedAt: now, UpdatedAt: now}
	if err := w.Jobs.Upsert(job); err != nil {
		t.Fatal(err)
	}
	res, err := (&w).RetryJob(context.Background(), job.JobID, true)
	if err != nil || !res.Accepted {
		t.Fatalf("first retry should be accepted: %+v %v", res, err)
	}
	_, _ = (&w).RetryJob(context.Background(), job.JobID, true)
	time.Sleep(50 * time.Millisecond)
	if u.Calls() != 1 {
		t.Fatalf("expected one upload call, got %d", u.Calls())
	}
	close(u.block)
}

func TestAutoRetryAndManualRetryShareGuard(t *testing.T) {
	w, sessionID, _, _ := testWorker(t, nil, true)
	u := &countingUploader{block: make(chan struct{})}
	w.Uploader = u
	now := time.Now().UTC()
	nextRetry := now.Add(-time.Second)
	job := FileUploadJob{JobID: JobID(sessionID, "p-auto", AssetKindOriginal), SessionID: sessionID, StationID: "station_1", PhotoID: "p-auto", AssetKind: AssetKindOriginal, LocalPath: mustTempFile(t), BucketName: "bucket", ObjectKey: "root/auto.jpg", DedupeKey: JobID(sessionID, "p-auto", AssetKindOriginal), Status: JobStatusRetryScheduled, AttemptCount: 1, MaxAttempts: MaxAutoUploadAttempts, LastErrorCode: ErrorUploadFailed, LastErrorAction: ActionRetryCloudUpload, NextRetryAt: &nextRetry, CreatedAt: now, UpdatedAt: now}
	if err := w.Jobs.Upsert(job); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		w.AutoRetryDue(context.Background(), now)
	}()
	go func() {
		defer wg.Done()
		_, _ = w.RetryJob(context.Background(), job.JobID, true)
	}()
	wg.Wait()
	time.Sleep(50 * time.Millisecond)
	if u.Calls() != 1 {
		t.Fatalf("expected scheduler and manual retry to share guard and call uploader once, got %d", u.Calls())
	}
	close(u.block)
}

func TestUploadedRetryIsNoopAndFailedRetryReusesIdentity(t *testing.T) {
	w, sessionID, _, _ := testWorker(t, nil, true)
	now := time.Now().UTC()
	job := FileUploadJob{JobID: JobID(sessionID, "p2", AssetKindGraded), SessionID: sessionID, StationID: "station_1", PhotoID: "p2", AssetKind: AssetKindGraded, LocalPath: mustTempFile(t), BucketName: "bucket", ObjectKey: "root/graded/file.jpg", DedupeKey: JobID(sessionID, "p2", AssetKindGraded), Status: JobStatusUploaded, AttemptCount: 1, CreatedAt: now, UpdatedAt: now}
	if err := w.Jobs.Upsert(job); err != nil {
		t.Fatal(err)
	}
	res, err := (&w).RetryJob(context.Background(), job.JobID, true)
	if err != nil || res.NoopReason != ErrorUploadAlreadyUploaded || res.Accepted {
		t.Fatalf("uploaded retry should be successful truthful no-op, res=%+v err=%v", res, err)
	}
	job.Status = JobStatusFailed
	job.LastErrorCode = ErrorUploadFailed
	job.LastErrorAction = ActionRetryCloudUpload
	if err := w.Jobs.Upsert(job); err != nil {
		t.Fatal(err)
	}
	_, err = (&w).RetryJob(context.Background(), job.JobID, true)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		got, _ := w.Jobs.Get(job.JobID)
		if got.Status == JobStatusUploaded {
			if got.JobID != job.JobID || got.ObjectKey != job.ObjectKey || got.LocalPath != job.LocalPath {
				t.Fatalf("identity changed: %+v", got)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("retry did not upload")
}

func mustTempFile(t *testing.T) string {
	t.Helper()
	p := t.TempDir() + "/file.jpg"
	if err := os.WriteFile(p, []byte("jpg"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRetryRunnerNoopReasonsDoNotIncrementAttempts(t *testing.T) {
	w, sessionID, _, _ := testWorker(t, nil, true)
	now := time.Now().UTC()
	for _, tc := range []struct {
		name   string
		status string
		reason string
	}{
		{name: "uploading", status: JobStatusUploading, reason: ErrorUploadAlreadyRunning},
		{name: "retrying", status: JobStatusRetrying, reason: ErrorUploadAlreadyRunning},
		{name: "uploaded", status: JobStatusUploaded, reason: ErrorUploadAlreadyUploaded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			job := FileUploadJob{JobID: JobID(sessionID, tc.name, AssetKindOriginal), SessionID: sessionID, StationID: "station_1", PhotoID: tc.name, AssetKind: AssetKindOriginal, LocalPath: mustTempFile(t), BucketName: "bucket", ObjectKey: "root/" + tc.name + ".jpg", DedupeKey: JobID(sessionID, tc.name, AssetKindOriginal), Status: tc.status, AttemptCount: 2, LastErrorCode: ErrorDriveUploadFailed, LastErrorAction: ActionRetryDriveUpload, CreatedAt: now, UpdatedAt: now}
			if tc.status == JobStatusUploaded {
				job.DriveFileID = "drive-file"
				job.RemoteIdentity = "drive-file"
			}
			if err := w.Jobs.Upsert(job); err != nil {
				t.Fatal(err)
			}
			res, err := w.RetryJob(context.Background(), job.JobID, true)
			if err != nil || res.Accepted || res.NoopReason != tc.reason {
				t.Fatalf("expected safe no-op %s, res=%+v err=%v", tc.reason, res, err)
			}
			got, _ := w.Jobs.Get(job.JobID)
			if got.AttemptCount != 2 {
				t.Fatalf("attempt count changed on no-op: %d", got.AttemptCount)
			}
		})
	}
}
