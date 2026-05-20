package processing

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/sessions"
)

func TestStartupRecoveryVerifiesOriginalAndFailsMissingProcessedOutput(t *testing.T) {
	root := t.TempDir()
	orig := filepath.Join(root, "orig.jpg")
	if err := os.WriteFile(orig, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := photos.NewStore()
	p1 := store.Route("station_1", "session_1", filepath.Join(root, "src1.jpg"), int64(len("original")), time.Now(), time.Now(), time.Now())
	store.MarkOriginalSaved(p1.PhotoID, orig, time.Now())
	p2 := store.Route("station_1", "session_1", filepath.Join(root, "src2.jpg"), 1, time.Now(), time.Now(), time.Now())
	orig2 := filepath.Join(root, "orig2.jpg")
	if err := os.WriteFile(orig2, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	store.MarkOriginalSaved(p2.PhotoID, orig2, time.Now())
	store.MarkGradedProcessed(p2.PhotoID, filepath.Join(root, "missing-graded.jpg"), filepath.Join(root, "look.cube"), time.Now())

	res := StartupRecovery{OriginalSaver: &OriginalSaver{Photos: store}, Processor: &GradedProcessor{Photos: store}}.Recover()
	if res.Summary.Failed == 0 || res.Summary.Resumed == 0 {
		t.Fatalf("unexpected summary: %+v", res.Summary)
	}
	failed, _ := store.Get(p2.PhotoID)
	if failed.GradedProcessingStatus != photos.GradedStatusFailed || !strings.Contains(failed.GradedLastError, "file is missing") || !strings.Contains(failed.GradedLastError, "manual retry") {
		t.Fatalf("missing graded was not failed actionably: %+v", failed)
	}
}

func TestStartupRecoveryReportsSpecificOriginalFailureReason(t *testing.T) {
	root := t.TempDir()
	store := photos.NewStore()
	p := store.Route("station_1", "session_1", filepath.Join(root, "src.jpg"), 4, time.Now(), time.Now(), time.Now())
	dir := filepath.Join(root, "original-dir")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	store.MarkOriginalSaved(p.PhotoID, dir, time.Now())
	res := StartupRecovery{OriginalSaver: &OriginalSaver{Photos: store}, Processor: &GradedProcessor{Photos: store}}.Recover()
	if res.Summary.Failed != 1 || res.Summary.Errors != 1 {
		t.Fatalf("unexpected summary: %+v", res.Summary)
	}
	failed, _ := store.Get(p.PhotoID)
	if !strings.Contains(failed.LastError, "directory") || !strings.Contains(failed.LastError, "restore") {
		t.Fatalf("original failure reason not specific/actionable: %q", failed.LastError)
	}
}

func TestStartupRecoveryRetriesOriginalFromSourceOrFailsActionably(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "camera.jpg")
	if err := os.WriteFile(source, []byte("jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := photos.NewStore()
	ok := store.Route("station_1", "session_1", source, 4, time.Now(), time.Now(), time.Now())
	missing := store.Route("station_1", "session_1", filepath.Join(root, "missing.jpg"), 4, time.Now(), time.Now(), time.Now())
	res := StartupRecovery{OriginalSaver: &OriginalSaver{Photos: store, Sessions: testSessionStore(root), Persist: func() error { return nil }}, Processor: &GradedProcessor{Photos: store}}.Recover()
	if res.Summary.Failed == 0 || res.Summary.Verified == 0 {
		t.Fatalf("unexpected summary: %+v", res.Summary)
	}
	okPhoto, _ := store.Get(ok.PhotoID)
	if okPhoto.OriginalSaveStatus != photos.OriginalStatusSaved {
		t.Fatalf("source-backed original did not recover: %+v", okPhoto)
	}
	missingPhoto, _ := store.Get(missing.PhotoID)
	if missingPhoto.OriginalSaveStatus != photos.OriginalStatusFailed || missingPhoto.LastError == "" {
		t.Fatalf("missing source did not fail actionably: %+v", missingPhoto)
	}
}

func TestStartupRecoveryEnqueuesPendingAndFailedUnderLimitButSkipsLimit(t *testing.T) {
	root := t.TempDir()
	orig := filepath.Join(root, "orig.jpg")
	_ = os.WriteFile(orig, []byte("original"), 0o644)
	store := photos.NewStore()
	pending := store.Route("station_1", "session_1", filepath.Join(root, "pending.jpg"), int64(len("original")), time.Now(), time.Now(), time.Now())
	store.MarkOriginalSaved(pending.PhotoID, orig, time.Now())
	failed := store.Route("station_1", "session_1", filepath.Join(root, "failed.jpg"), int64(len("original")), time.Now(), time.Now(), time.Now())
	store.MarkOriginalSaved(failed.PhotoID, orig, time.Now())
	store.MarkGradedProcessing(failed.PhotoID, filepath.Join(root, "f.jpg"), "lut", time.Now())
	store.MarkGradedFailed(failed.PhotoID, "failed", time.Now())
	limit := store.Route("station_1", "session_1", filepath.Join(root, "limit.jpg"), int64(len("original")), time.Now(), time.Now(), time.Now())
	store.MarkOriginalSaved(limit.PhotoID, orig, time.Now())
	for i := 0; i < MaxGradedAttempts; i++ {
		store.MarkGradedProcessing(limit.PhotoID, filepath.Join(root, "l.jpg"), "lut", time.Now())
		store.MarkGradedFailed(limit.PhotoID, "failed", time.Now())
	}
	res := StartupRecovery{Processor: &GradedProcessor{Photos: store}}.Recover()
	if len(res.EnqueueIDs) != 2 || res.Summary.Resumed != 2 || res.Summary.SkippedRetryLimit != 1 {
		t.Fatalf("unexpected recovery result: %+v", res)
	}
}

func TestStartupRecoveryCountsFailedAttemptTwoAsResumedForOneFinalAutomaticAttempt(t *testing.T) {
	root := t.TempDir()
	orig := filepath.Join(root, "orig.jpg")
	_ = os.WriteFile(orig, []byte("original"), 0o644)
	store := photos.NewStore()
	failed := store.Route("station_1", "session_1", filepath.Join(root, "failed.jpg"), int64(len("original")), time.Now(), time.Now(), time.Now())
	store.MarkOriginalSaved(failed.PhotoID, orig, time.Now())
	for i := 0; i < MaxGradedAttempts-1; i++ {
		store.MarkGradedProcessing(failed.PhotoID, filepath.Join(root, "f.jpg"), "lut", time.Now())
		store.MarkGradedFailed(failed.PhotoID, "failed", time.Now())
	}
	res := StartupRecovery{Processor: &GradedProcessor{Photos: store}}.Recover()
	if len(res.EnqueueIDs) != 1 || res.EnqueueIDs[0] != failed.PhotoID || res.Summary.Resumed != 1 || res.Summary.SkippedRetryLimit != 0 {
		t.Fatalf("attempt two should resume for exactly one automatic attempt: %+v", res)
	}
}

func TestStartupRecoveryPersistenceErrorIncrementsErrorsAndDoesNotPublishSuccess(t *testing.T) {
	root := t.TempDir()
	store := photos.NewStore()
	p := store.Route("station_1", "session_1", filepath.Join(root, "src.jpg"), 1, time.Now(), time.Now(), time.Now())
	store.MarkOriginalSaved(p.PhotoID, filepath.Join(root, "missing.jpg"), time.Now())
	res := StartupRecovery{OriginalSaver: &OriginalSaver{Photos: store, Persist: func() error { return os.ErrPermission }}, Processor: &GradedProcessor{Photos: store}}.Recover()
	if res.Summary.Failed != 1 || res.Summary.Errors != 1 {
		t.Fatalf("persist failure should be reported truthfully: %+v", res.Summary)
	}
}

func TestProcessingGuardPreventsRecoveryManualDoubleStart(t *testing.T) {
	guard := NewProcessingGuard()
	photoID := "photo_same"
	var starts int
	var mu sync.Mutex
	start := func() {
		if guard.TryStart(photoID) {
			mu.Lock()
			starts++
			mu.Unlock()
			time.Sleep(20 * time.Millisecond)
			guard.Done(photoID)
		}
	}
	go start()
	go start()
	time.Sleep(60 * time.Millisecond)
	if starts != 1 {
		t.Fatalf("starts=%d want 1", starts)
	}
}

func testSessionStore(root string) *sessions.Store {
	store := sessions.NewStore()
	_ = store.ReplaceAll([]sessions.Session{{SessionID: "session_1", StationID: "station_1", Status: sessions.StatusActive, CustomerName: "Alice", OrderNumber: "1", TimerSeconds: 300, StartedAt: time.Now(), EndsAt: time.Now().Add(time.Hour), StationSnapshot: sessions.StationSnapshot{StationName: "Station 1", OutputFolder: root}}})
	return store
}

func TestStartupRecoveryPublishesSafeSummaryOnly(t *testing.T) {
	acts := activity.NewStore(10)
	broker := events.NewBroker()
	res := StartupRecovery{Activity: acts, Broker: broker}.Recover()
	if res.Summary != (StartupRecoverySummary{}) {
		t.Fatalf("unexpected empty summary: %+v", res)
	}
	store := photos.NewStore()
	root := t.TempDir()
	p := store.Route("station_1", "session_1", filepath.Join(root, "raw-secret.jpg"), 1, time.Now(), time.Now(), time.Now())
	store.MarkOriginalSaved(p.PhotoID, filepath.Join(root, "missing-secret.jpg"), time.Now())
	StartupRecovery{OriginalSaver: &OriginalSaver{Photos: store}, Processor: &GradedProcessor{Photos: store}, Activity: acts, Broker: broker}.Recover()
	entries := acts.Recent(5, "processing.recovered")
	if len(entries) == 0 {
		t.Fatal("expected recovery activity")
	}
	if strings.Contains(entries[0].Message, root) || strings.Contains(entries[0].Message, "secret") {
		t.Fatalf("activity leaked path: %q", entries[0].Message)
	}
}
