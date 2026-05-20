package processing

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/sessions"
)

type fakeLUT struct {
	err    error
	input  string
	lut    string
	output string
}

func (f *fakeLUT) Apply(ctx context.Context, inputPath, lutPath, outputPath string) error {
	f.input = inputPath
	f.lut = lutPath
	f.output = outputPath
	if f.err != nil {
		return f.err
	}
	return os.WriteFile(outputPath, []byte("graded"), 0o644)
}

func TestGradedPathUsesSessionSnapshotAndPhotoID(t *testing.T) {
	root := t.TempDir()
	photo := photos.Photo{PhotoID: "photo_same", SourcePath: filepath.Join(root, "camera", "IMG_0001.jpeg")}
	session := sessions.Session{SessionID: "s1", StationID: "station_1", CustomerName: "Alice", OrderNumber: "ORD/1", StationSnapshot: sessions.StationSnapshot{StationName: "Front", OutputFolder: root}}
	path, err := GradedPath(session, photo)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "Alice_ORD_1", "Front", "graded", "IMG_0001__photo_same.jpg")
	if path != want {
		t.Fatalf("path=%q want %q", path, want)
	}
}

func TestGradedProcessingRequiresSavedOriginalAndUsesLocalOriginal(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "missing-camera.jpg")
	orig := filepath.Join(root, "orig.jpg")
	lut := filepath.Join(root, "look.cube")
	if err := os.WriteFile(orig, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lut, []byte("cube"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := photos.NewStore()
	routed := store.Route("station_1", "session_1", src, int64(len("original")), time.Now(), time.Now(), time.Now())
	store.MarkOriginalSaved(routed.PhotoID, orig, time.Now())
	sessionStore := sessions.NewStore()
	_ = sessionStore.ReplaceAll([]sessions.Session{{SessionID: "session_1", StationID: "station_1", Status: sessions.StatusActive, CustomerName: "Alice", OrderNumber: "1", TimerSeconds: 300, StartedAt: time.Now(), EndsAt: time.Now().Add(time.Hour), StationSnapshot: sessions.StationSnapshot{StationName: "Station 1", DefaultLUTPath: lut, OutputFolder: root}}})
	fake := &fakeLUT{}
	res := GradedProcessor{Photos: store, Sessions: sessionStore, Processor: fake}.Process(context.Background(), routed.PhotoID)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if fake.input != orig {
		t.Fatalf("processor input=%q want local original %q", fake.input, orig)
	}
	if res.Photo.GradedProcessingStatus != photos.GradedStatusProcessed || res.Photo.LocalGradedPath == "" {
		t.Fatalf("unexpected graded state: %+v", res.Photo)
	}
}

func TestGradedProcessingFailsSafelyForMissingLUT(t *testing.T) {
	root := t.TempDir()
	orig := filepath.Join(root, "orig.jpg")
	_ = os.WriteFile(orig, []byte("original"), 0o644)
	store := photos.NewStore()
	routed := store.Route("station_1", "session_1", filepath.Join(root, "src.jpg"), int64(len("original")), time.Now(), time.Now(), time.Now())
	store.MarkOriginalSaved(routed.PhotoID, orig, time.Now())
	sessionStore := sessions.NewStore()
	_ = sessionStore.ReplaceAll([]sessions.Session{{SessionID: "session_1", StationID: "station_1", Status: sessions.StatusActive, CustomerName: "Alice", OrderNumber: "1", TimerSeconds: 300, StartedAt: time.Now(), EndsAt: time.Now().Add(time.Hour), StationSnapshot: sessions.StationSnapshot{StationName: "Station 1", DefaultLUTPath: filepath.Join(root, "missing.cube"), OutputFolder: root}}})
	res := GradedProcessor{Photos: store, Sessions: sessionStore, Processor: &fakeLUT{}}.Process(context.Background(), routed.PhotoID)
	if res.Err == nil {
		t.Fatal("expected missing LUT failure")
	}
	if res.Photo.GradedProcessingStatus != photos.GradedStatusFailed || res.Photo.LocalOriginalPath != orig {
		t.Fatalf("failure did not preserve original/state: %+v", res.Photo)
	}
}

func TestWriteGradedRejectsExistingUnverifiedTarget(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "input.jpg")
	lut := filepath.Join(root, "look.cube")
	target := filepath.Join(root, "graded.jpg")
	_ = os.WriteFile(input, []byte("original"), 0o644)
	_ = os.WriteFile(lut, []byte("cube"), 0o644)
	_ = os.WriteFile(target, []byte("someone-else-output"), 0o644)
	fake := &fakeLUT{}
	if err := writeGraded(context.Background(), fake, input, lut, target); err == nil {
		t.Fatal("expected existing target to be rejected")
	}
	if fake.output != "" {
		t.Fatalf("processor should not run against existing target, got output %q", fake.output)
	}
}

func TestReconcileDoesNotAutomaticallyRetryFailedPhotoAtLimit(t *testing.T) {
	root := t.TempDir()
	orig := filepath.Join(root, "orig.jpg")
	lut := filepath.Join(root, "look.cube")
	_ = os.WriteFile(orig, []byte("original"), 0o644)
	_ = os.WriteFile(lut, []byte("cube"), 0o644)
	store := photos.NewStore()
	routed := store.Route("station_1", "session_1", filepath.Join(root, "src.jpg"), int64(len("original")), time.Now(), time.Now(), time.Now())
	store.MarkOriginalSaved(routed.PhotoID, orig, time.Now())
	store.MarkGradedProcessing(routed.PhotoID, filepath.Join(root, "graded.jpg"), lut, time.Now())
	store.MarkGradedFailed(routed.PhotoID, "LUT_PROCESSING_FAILED", time.Now())
	store.MarkGradedProcessing(routed.PhotoID, filepath.Join(root, "graded.jpg"), lut, time.Now())
	store.MarkGradedFailed(routed.PhotoID, "LUT_PROCESSING_FAILED", time.Now())
	store.MarkGradedProcessing(routed.PhotoID, filepath.Join(root, "graded.jpg"), lut, time.Now())
	store.MarkGradedFailed(routed.PhotoID, "LUT_PROCESSING_FAILED", time.Now())
	sessionStore := sessions.NewStore()
	_ = sessionStore.ReplaceAll([]sessions.Session{{SessionID: "session_1", StationID: "station_1", Status: sessions.StatusActive, CustomerName: "Alice", OrderNumber: "1", TimerSeconds: 300, StartedAt: time.Now(), EndsAt: time.Now().Add(time.Hour), StationSnapshot: sessions.StationSnapshot{StationName: "Station 1", DefaultLUTPath: lut, OutputFolder: root}}})
	fake := &fakeLUT{}
	results := GradedProcessor{Photos: store, Sessions: sessionStore, Processor: fake}.Reconcile(context.Background())
	if len(results) != 0 {
		t.Fatalf("expected no automatic reconciliation retry at limit, got %+v", results)
	}
	photo, _ := store.Get(routed.PhotoID)
	if photo.GradedAttemptCount != MaxGradedAttempts || photo.GradedProcessingStatus != photos.GradedStatusFailed {
		t.Fatalf("unexpected photo state after reconcile: %+v", photo)
	}
	if fake.output != "" {
		t.Fatalf("processor should not run after automatic retry limit, got %q", fake.output)
	}
}

func TestImageMagickCubeCommandFixture(t *testing.T) {
	if _, err := exec.LookPath("magick"); err != nil {
		t.Skip("ImageMagick magick command is not available")
	}
	root := t.TempDir()
	input := filepath.Join(root, "input.ppm")
	lut := filepath.Join(root, "identity.cube")
	output := filepath.Join(root, "output.jpg")
	ppm := "P3\n1 1\n255\n128 64 32\n"
	cube := "TITLE \"identity\"\nLUT_3D_SIZE 2\nDOMAIN_MIN 0 0 0\nDOMAIN_MAX 1 1 1\n0 0 0\n0 0 1\n0 1 0\n0 1 1\n1 0 0\n1 0 1\n1 1 0\n1 1 1\n"
	if err := os.WriteFile(input, []byte(ppm), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lut, []byte(cube), 0o644); err != nil {
		t.Fatal(err)
	}
	processor := ImageMagickLUTProcessor{Timeout: 10 * time.Second}
	if err := processor.Apply(context.Background(), input, lut, output); err != nil {
		t.Fatalf("ImageMagick .cube command fixture failed: %v", err)
	}
	if err := validGradedPath(output); err != nil {
		t.Fatalf("fixture output invalid: %v", err)
	}
}

func TestReconcileDoesNotClaimMissingProcessedOutput(t *testing.T) {
	root := t.TempDir()
	store := photos.NewStore()
	routed := store.Route("station_1", "session_1", filepath.Join(root, "src.jpg"), 1, time.Now(), time.Now(), time.Now())
	store.MarkOriginalSaved(routed.PhotoID, filepath.Join(root, "orig.jpg"), time.Now())
	store.MarkGradedProcessed(routed.PhotoID, filepath.Join(root, "missing.jpg"), filepath.Join(root, "look.cube"), time.Now())
	results := GradedProcessor{Photos: store}.Reconcile(context.Background())
	if len(results) != 1 || results[0].Photo.GradedProcessingStatus != photos.GradedStatusFailed {
		t.Fatalf("unexpected reconcile: %+v", results)
	}
}
