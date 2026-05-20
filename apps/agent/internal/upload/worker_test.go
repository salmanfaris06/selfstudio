package upload

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/sessions"
)

type fakeUploader struct{ err error }

func (f fakeUploader) Upload(ctx context.Context, bucketName, objectKey, localPath string) (UploadResult, error) {
	if f.err != nil {
		return UploadResult{}, f.err
	}
	return UploadResult{RemoteIdentity: "gs://" + bucketName + "/" + objectKey}, nil
}

func TestWorkerUploadsOriginalAndGradedWithFakeUploader(t *testing.T) {
	w, sessionID, original, graded := testWorker(t, nil, true)
	res, err := w.StartSession(context.Background(), sessionID)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		jobs := w.Jobs.ListBySession(sessionID)
		if len(jobs) == 2 {
			done := 0
			for _, j := range jobs {
				if j.Status == JobStatusUploaded {
					done++
				}
			}
			if done == 2 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(original); err != nil {
		t.Fatal("original deleted or missing")
	}
	if _, err := os.Stat(graded); err != nil {
		t.Fatal("graded deleted or missing")
	}
	if len(res.Jobs) != 2 {
		t.Fatalf("expected two jobs, got %d", len(res.Jobs))
	}
}

func TestWorkerKeepsOriginalEligibleWhenGradedFailed(t *testing.T) {
	w, sessionID, _, _ := testWorker(t, nil, false)
	_, err := w.StartSession(context.Background(), sessionID)
	if err != nil {
		t.Fatal(err)
	}
	jobs := w.Jobs.ListBySession(sessionID)
	if len(jobs) != 2 {
		t.Fatalf("expected original + not eligible graded")
	}
	var notEligible bool
	for _, j := range jobs {
		if j.AssetKind == AssetKindGraded && j.Status == JobStatusNotEligible {
			notEligible = true
		}
	}
	if !notEligible {
		t.Fatal("graded failed should be not eligible")
	}
}

func TestWorkerPersistsSafeFailureOnUploaderError(t *testing.T) {
	w, sessionID, original, _ := testWorker(t, errors.New("raw token private_key secret"), true)
	_, err := w.StartSession(context.Background(), sessionID)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if _, err := os.Stat(original); err != nil {
		t.Fatal("local file should remain")
	}
	for _, j := range w.Jobs.ListBySession(sessionID) {
		if (j.Status == JobStatusFailed || j.Status == JobStatusRetryScheduled) && j.LastErrorCode != "" && j.LastErrorAction != "" && j.LastErrorCode != "raw token private_key secret" {
			return
		}
	}
	t.Fatal("expected safe failed job")
}

func testWorker(t *testing.T, uploadErr error, gradedProcessed bool) (Worker, string, string, string) {
	t.Helper()
	dir := t.TempDir()
	original := filepath.Join(dir, "orig.JPG")
	graded := filepath.Join(dir, "graded.JPG")
	os.WriteFile(original, []byte("jpg"), 0o600)
	os.WriteFile(graded, []byte("jpg"), 0o600)
	sessionID := "sess_abcd1234"
	ss := sessions.NewStore()
	now := time.Now().UTC()
	ended := now
	reason := sessions.EndReasonManual
	if err := ss.ReplaceAll([]sessions.Session{{SessionID: sessionID, StationID: "station_1", Status: sessions.StatusLocked, CustomerName: "Cust", OrderNumber: "Ord", TimerSeconds: 60, StartedAt: now, EndsAt: now.Add(time.Minute), StationSnapshot: sessions.StationSnapshot{OutputFolder: dir}, EndedAt: &ended, EndReason: &reason}}); err != nil {
		t.Fatal(err)
	}
	ps := photos.NewStore()
	p := photos.Photo{PhotoID: "photo_abcd1234", StationID: "station_1", SessionID: sessionID, SourcePath: filepath.Join(dir, "src.JPG"), SourceSizeBytes: 1, DetectedAt: now, StableAt: now, RoutedAt: now, Status: photos.StatusRouted, LocalOriginalPath: original, OriginalSaveStatus: photos.OriginalStatusSaved, GradedProcessingStatus: photos.GradedStatusFailed}
	if gradedProcessed {
		p.LocalGradedPath = graded
		p.GradedProcessingStatus = photos.GradedStatusProcessed
	}
	ps.ReplaceAll([]photos.Photo{p})
	targets := NewStore()
	targets.Upsert(SessionCloudTarget{SessionID: sessionID, StationID: "station_1", BucketName: "bucket", ObjectPrefix: "root/2026/05/19/cust/ord/station_1/sess_abcd1234", RemoteIdentity: "gcs://bucket/root/2026/05/19/cust/ord/station_1/sess_abcd1234", Status: StatusReady, CreatedAt: now, UpdatedAt: now})
	return Worker{Sessions: ss, Photos: ps, Targets: targets, Jobs: NewJobsStore(), Persistence: NewJobsPersistence(dir), Uploader: fakeUploader{err: uploadErr}}, sessionID, original, graded
}
