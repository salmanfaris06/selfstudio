package processing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/sessions"
)

func testSession(root string) sessions.Session {
	now := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	return sessions.Session{SessionID: "sess_1", StationID: "station_1", Status: sessions.StatusActive, CustomerName: `Acme: Client`, OrderNumber: `Order/42`, TimerSeconds: 60, StartedAt: now, EndsAt: now.Add(time.Minute), StationSnapshot: sessions.StationSnapshot{StationName: `Station*One`, OutputFolder: root}}
}

func testPhoto(source string, size int64) photos.Photo {
	now := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	return photos.NewStore().Route("station_1", "sess_1", source, size, now, now, now)
}

func TestOriginalPathSanitizesAndUsesStablePhotoIDSuffix(t *testing.T) {
	root := t.TempDir()
	photo := testPhoto(filepath.Join(t.TempDir(), "IMG:001.JPG"), 10)
	path, err := OriginalPath(testSession(root), photo)
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if !strings.Contains(path, "Acme_ Client_Order_42") || !strings.Contains(path, "Station_One") || !strings.Contains(path, "originals") {
		t.Fatalf("unexpected path %s", path)
	}
	if !strings.HasSuffix(path, "__"+photo.PhotoID+".jpg") {
		t.Fatalf("missing deterministic suffix: %s", path)
	}
	rel, _ := filepath.Rel(root, path)
	if strings.HasPrefix(rel, "..") {
		t.Fatalf("escaped root: %s", path)
	}
}

func TestOriginalPathPreventsTraversalFromSegments(t *testing.T) {
	root := t.TempDir()
	session := testSession(root)
	session.CustomerName = `..\\evil`
	session.OrderNumber = `../order`
	path, err := OriginalPath(session, testPhoto(filepath.Join(t.TempDir(), "a.jpeg"), 3))
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	rel, _ := filepath.Rel(root, path)
	if strings.HasPrefix(rel, "..") || strings.Contains(rel, ".."+string(os.PathSeparator)) {
		t.Fatalf("unsafe rel %s", rel)
	}
	if !strings.HasSuffix(path, ".jpeg") {
		t.Fatalf("expected jpeg extension: %s", path)
	}
}

func TestSaveCopiesOriginalVerifiesAndLeavesSourceUntouched(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(t.TempDir(), "same.jpg")
	if err := os.WriteFile(source, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}
	photoStore := photos.NewStore()
	now := time.Now().UTC()
	photo := photoStore.Route("station_1", "sess_1", source, 6, now, now, now)
	sessionStore := sessions.NewStore()
	if err := sessionStore.ReplaceAll([]sessions.Session{testSession(root)}); err != nil {
		t.Fatal(err)
	}
	saves := 0
	result := OriginalSaver{Photos: photoStore, Sessions: sessionStore, Persist: func() error { saves++; return nil }, Now: func() time.Time { return now }}.Save(photo.PhotoID)
	if result.Err != nil {
		t.Fatalf("save: %v", result.Err)
	}
	got, _ := photoStore.Get(photo.PhotoID)
	if got.OriginalSaveStatus != photos.OriginalStatusSaved || got.ProcessingStatus != photos.ProcessingStatusEligible {
		t.Fatalf("bad state %+v", got)
	}
	if data, _ := os.ReadFile(source); string(data) != "abcdef" {
		t.Fatalf("source mutated")
	}
	if info, err := os.Stat(got.LocalOriginalPath); err != nil || info.Size() != 6 {
		t.Fatalf("target invalid: %v %v", info, err)
	}
	if saves < 2 {
		t.Fatalf("expected persistence around save, got %d", saves)
	}
}

func TestSaveFailureDoesNotClaimSuccess(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(t.TempDir(), "missing.jpg")
	photoStore := photos.NewStore()
	now := time.Now().UTC()
	photo := photoStore.Route("station_1", "sess_1", missing, 6, now, now, now)
	sessionStore := sessions.NewStore()
	_ = sessionStore.ReplaceAll([]sessions.Session{testSession(root)})
	result := OriginalSaver{Photos: photoStore, Sessions: sessionStore}.Save(photo.PhotoID)
	if result.Err == nil {
		t.Fatalf("expected error")
	}
	got, _ := photoStore.Get(photo.PhotoID)
	if got.OriginalSaveStatus != photos.OriginalStatusFailed || got.ProcessingStatus == photos.ProcessingStatusEligible {
		t.Fatalf("bad failure state %+v", got)
	}
}

func TestSaveRejectsExistingTargetWithSameSizeDifferentContent(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(t.TempDir(), "same-size.jpg")
	if err := os.WriteFile(source, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	photoStore := photos.NewStore()
	now := time.Now().UTC()
	photo := photoStore.Route("station_1", "sess_1", source, 6, now, now, now)
	session := testSession(root)
	target, err := OriginalPath(session, photo)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("WRONG!"), 0o644); err != nil {
		t.Fatal(err)
	}
	sessionStore := sessions.NewStore()
	_ = sessionStore.ReplaceAll([]sessions.Session{session})
	result := OriginalSaver{Photos: photoStore, Sessions: sessionStore}.Save(photo.PhotoID)
	if result.Err == nil {
		t.Fatalf("expected same-size content mismatch to fail")
	}
	got, _ := photoStore.Get(photo.PhotoID)
	if got.OriginalSaveStatus != photos.OriginalStatusFailed || got.ProcessingStatus == photos.ProcessingStatusEligible {
		t.Fatalf("bad failure state %+v", got)
	}
	if data, _ := os.ReadFile(target); string(data) != "WRONG!" {
		t.Fatalf("existing target was overwritten")
	}
}

func TestSaveAcceptsExistingTargetOnlyWhenContentMatches(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(t.TempDir(), "same-content.jpg")
	if err := os.WriteFile(source, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	photoStore := photos.NewStore()
	now := time.Now().UTC()
	photo := photoStore.Route("station_1", "sess_1", source, 6, now, now, now)
	session := testSession(root)
	target, err := OriginalPath(session, photo)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	sessionStore := sessions.NewStore()
	_ = sessionStore.ReplaceAll([]sessions.Session{session})
	result := OriginalSaver{Photos: photoStore, Sessions: sessionStore}.Save(photo.PhotoID)
	if result.Err != nil {
		t.Fatalf("save: %v", result.Err)
	}
	got, _ := photoStore.Get(photo.PhotoID)
	if got.OriginalSaveStatus != photos.OriginalStatusSaved || got.ProcessingStatus != photos.ProcessingStatusEligible {
		t.Fatalf("bad saved state %+v", got)
	}
}

func TestDuplicateRouteDoesNotCreateSecondOriginalPath(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(t.TempDir(), "dup.jpg")
	_ = os.WriteFile(source, []byte("abc"), 0o644)
	photoStore := photos.NewStore()
	now := time.Now().UTC()
	first := photoStore.Route("station_1", "sess_1", source, 3, now, now, now)
	dup := photoStore.Route("station_1", "sess_1", source, 3, now, now, now)
	if !dup.Duplicate || dup.PhotoID != first.PhotoID {
		t.Fatalf("expected duplicate")
	}
	sessionStore := sessions.NewStore()
	_ = sessionStore.ReplaceAll([]sessions.Session{testSession(root)})
	saver := OriginalSaver{Photos: photoStore, Sessions: sessionStore}
	if res := saver.Save(first.PhotoID); res.Err != nil {
		t.Fatal(res.Err)
	}
	one, _ := photoStore.Get(first.PhotoID)
	if res := saver.Save(dup.PhotoID); res.Err != nil {
		t.Fatal(res.Err)
	}
	two, _ := photoStore.Get(first.PhotoID)
	if one.LocalOriginalPath != two.LocalOriginalPath {
		t.Fatalf("path changed")
	}
}

func TestReconcileMarksMissingSavedOriginalFailed(t *testing.T) {
	photoStore := photos.NewStore()
	now := time.Now().UTC()
	p := photoStore.Route("station_1", "sess_1", filepath.Join(t.TempDir(), "src.jpg"), 3, now, now, now)
	photoStore.MarkOriginalSaved(p.PhotoID, filepath.Join(t.TempDir(), "gone.jpg"), now)
	results := OriginalSaver{Photos: photoStore, Sessions: sessions.NewStore()}.ReconcilePending()
	if len(results) != 1 {
		t.Fatalf("results=%d", len(results))
	}
	got, _ := photoStore.Get(p.PhotoID)
	if got.OriginalSaveStatus != photos.OriginalStatusFailed {
		t.Fatalf("got %+v", got)
	}
}
