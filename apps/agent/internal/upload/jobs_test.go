package upload

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildFileObjectKeyDeterministicForOriginalAndGraded(t *testing.T) {
	target := SessionCloudTarget{SessionID: "s1", StationID: "station-a", Status: StatusReady, BucketName: "bucket", ObjectPrefix: "root/2026/05/19/customer/order/station-a/s1"}
	orig, err := BuildFileObjectKey(target, AssetKindOriginal, filepath.Join("tmp", "My Original.JPG"))
	if err != nil {
		t.Fatal(err)
	}
	graded, err := BuildFileObjectKey(target, AssetKindGraded, filepath.Join("tmp", "Color Grade.jpeg"))
	if err != nil {
		t.Fatal(err)
	}
	if orig != "root/2026/05/19/customer/order/station-a/s1/original/my-original.jpg" {
		t.Fatalf("unexpected original key: %s", orig)
	}
	if graded != "root/2026/05/19/customer/order/station-a/s1/graded/color-grade.jpeg" {
		t.Fatalf("unexpected graded key: %s", graded)
	}
	again, _ := BuildFileObjectKey(target, AssetKindOriginal, filepath.Join("else", "My Original.JPG"))
	if again != orig {
		t.Fatalf("key not deterministic")
	}
}

func TestJobsStoreRejectsDuplicatePhotoAssetAndCorruptState(t *testing.T) {
	now := time.Now().UTC()
	j := FileUploadJob{JobID: JobID("s1", "p1", AssetKindOriginal), SessionID: "s1", StationID: "st1", PhotoID: "p1", AssetKind: AssetKindOriginal, LocalPath: "a.jpg", BucketName: "bucket", ObjectKey: "root/a/original/a.jpg", Status: JobStatusPending, CreatedAt: now, UpdatedAt: now}
	if _, err := NewJobsStoreFromRecords([]FileUploadJob{j, j}); err == nil {
		t.Fatal("expected duplicate validation error")
	}
	dir := t.TempDir()
	p := NewJobsPersistence(dir)
	if err := os.MkdirAll(filepath.Dir(p.Path()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.Path(), []byte(`{"version":1,"jobs":[`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := p.LoadOrDefault(); err == nil {
		t.Fatal("expected corrupt state error")
	}
}

func TestAggregateUploadStatusTruthfulForRetryAndNotEligible(t *testing.T) {
	target := SessionCloudTarget{SessionID: "s1", StationID: "st1", Status: StatusReady}
	base := FileUploadJob{SessionID: "s1", StationID: "st1", PhotoID: "p", AssetKind: AssetKindOriginal, LocalPath: "file.jpg", DriveFolderID: "folder", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	job := func(photo, kind, status string) FileUploadJob {
		j := base
		j.PhotoID = photo
		j.AssetKind = kind
		j.JobID = JobID("s1", photo, kind)
		j.DedupeKey = j.JobID
		j.Status = status
		return j
	}
	if got := AggregateUploadStatus(target, []FileUploadJob{job("p1", AssetKindOriginal, JobStatusUploaded), job("p2", AssetKindGraded, JobStatusFailed)}); got != SessionUploadPartialFailed {
		t.Fatalf("uploaded+failed got %s", got)
	}
	if got := AggregateUploadStatus(target, []FileUploadJob{job("p1", AssetKindOriginal, JobStatusFailed), job("p2", AssetKindGraded, JobStatusFailed)}); got != SessionUploadFailed {
		t.Fatalf("all failed got %s", got)
	}
	if got := AggregateUploadStatus(target, []FileUploadJob{job("p1", AssetKindOriginal, JobStatusRetrying)}); got != SessionUploadUploading {
		t.Fatalf("retrying got %s", got)
	}
	if got := AggregateUploadStatus(target, []FileUploadJob{job("p1", AssetKindOriginal, JobStatusRetryScheduled)}); got != SessionUploadPending {
		t.Fatalf("retry scheduled got %s", got)
	}
	notEligible := job("p1", AssetKindGraded, JobStatusNotEligible)
	notEligible.LocalPath = ""
	notEligible.DriveFolderID = ""
	if got := AggregateUploadStatus(target, []FileUploadJob{job("p1", AssetKindOriginal, JobStatusUploaded), notEligible}); got != SessionUploadUploaded {
		t.Fatalf("uploaded+not_eligible got %s", got)
	}
}
