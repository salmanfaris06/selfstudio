package photos

import (
	"testing"
	"time"
)

func TestRouteReturnsExistingPhotoForDuplicateIdentity(t *testing.T) {
	store := NewStore()
	detectedAt := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	stableAt := detectedAt.Add(time.Second)
	first := store.Route("station_1", "sess_1", `C:\Input\A.JPG`, 3, detectedAt, stableAt, stableAt)
	second := store.Route("station_1", "sess_1", `c:\input\a.jpg`, 3, detectedAt.Add(30*time.Second), stableAt, stableAt.Add(time.Second))
	if first.PhotoID != second.PhotoID {
		t.Fatalf("expected duplicate to return same photo id, got %s and %s", first.PhotoID, second.PhotoID)
	}
	if !second.Duplicate {
		t.Fatalf("expected duplicate flag")
	}
	if count := store.CountBySession("sess_1"); count != 1 {
		t.Fatalf("count=%d", count)
	}
}

func TestRouteIdentityUsesStableTimestampInsteadOfDetectedAt(t *testing.T) {
	store := NewStore()
	detectedAt := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	stableAt := time.Date(2026, 5, 19, 10, 0, 5, 0, time.UTC)

	first := store.Route("station_1", "sess_1", `C:\Input\A.JPG`, 3, detectedAt, stableAt, stableAt)
	second := store.Route("station_1", "sess_1", `C:\Input\A.JPG`, 3, detectedAt.Add(time.Hour), stableAt, stableAt.Add(time.Hour))

	if first.PhotoID != second.PhotoID {
		t.Fatalf("expected same file with different detectedAt to reuse photo id, got %s and %s", first.PhotoID, second.PhotoID)
	}
	if !second.Duplicate {
		t.Fatalf("expected duplicate flag for same stable file identity")
	}
	if count := store.CountBySession("sess_1"); count != 1 {
		t.Fatalf("count=%d", count)
	}
}
