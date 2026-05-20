package photos

import (
	"errors"
	"testing"
	"time"
)

func TestPersistenceRoundTripPreservesIdentityAndDuplicates(t *testing.T) {
	now := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	store := NewStore()
	first := store.Route("station_1", "sess_1", `C:\Input\A.JPG`, 3, now, now, now)
	p := NewPersistence(t.TempDir())
	if err := p.Save(store); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := p.LoadOrDefault()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	again := loaded.Route("station_1", "sess_1", `c:\input\a.jpg`, 3, now.Add(time.Hour), now.Add(time.Hour), now.Add(time.Hour))
	if again.PhotoID != first.PhotoID || !again.Duplicate {
		t.Fatalf("expected duplicate after reload, got %+v", again)
	}
	if got := loaded.CountBySession("sess_1"); got != 1 {
		t.Fatalf("count=%d", got)
	}
}

func TestNewStoreFromRecordsRejectsDuplicateIdentityIgnoringTimestamps(t *testing.T) {
	now := time.Now().UTC()
	one := Photo{PhotoID: "photo_1", StationID: "station_1", SessionID: "sess_1", SourcePath: `C:\Input\A.JPG`, SourceSizeBytes: 3, DetectedAt: now, StableAt: now, RoutedAt: now, Status: StatusRouted}
	two := one
	two.PhotoID = "photo_2"
	two.DetectedAt = now.Add(time.Hour)
	_, err := NewStoreFromRecords([]Photo{one, two})
	if !errors.Is(err, ErrInvalidPhotoState) {
		t.Fatalf("expected invalid state, got %v", err)
	}
}

func TestPersistenceRejectsCorruptState(t *testing.T) {
	dir := t.TempDir()
	p := NewPersistence(dir)
	if err := p.write([]Photo{{PhotoID: "photo_1"}}); !errors.Is(err, ErrInvalidPhotoState) {
		t.Fatalf("expected invalid write, got %v", err)
	}
}
