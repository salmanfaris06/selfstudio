package quarantine

import (
	"errors"
	"testing"
	"time"
)

func TestQuarantineDuplicateIdentityIgnoresTimestamps(t *testing.T) {
	store := NewStore()
	first := store.Quarantine("station_1", "", "C:/input/IMG_001.JPG", 100, time.Unix(1, 0), time.Unix(2, 0), time.Unix(3, 0), ReasonNoActiveSession)
	second := store.Quarantine("station_1", "", "c:/input/img_001.jpg", 100, time.Unix(10, 0), time.Unix(11, 0), time.Unix(12, 0), ReasonLatePhoto)
	if first.QuarantineID != second.QuarantineID {
		t.Fatalf("expected duplicate quarantine id, got %s and %s", first.QuarantineID, second.QuarantineID)
	}
	if !second.Duplicate {
		t.Fatalf("expected duplicate flag on repeated quarantine")
	}
}

func TestListFiltersOpenAndAssignedQuarantine(t *testing.T) {
	store := NewStore()
	now := time.Now()
	open := store.Quarantine("station_1", "", "C:/input/open.jpg", 100, now, now, now, ReasonNoActiveSession)
	assigned := store.Quarantine("station_1", "sess_1", "C:/input/assigned.jpg", 101, now, now, now, ReasonLatePhoto)
	if _, err := store.Assign(assigned.QuarantineID, "sess_1", "photo_1", now.Add(time.Second)); err != nil {
		t.Fatalf("Assign returned error: %v", err)
	}
	items := store.List(ListFilter{Status: StatusQuarantined, StationID: "station_1", Limit: 10})
	if len(items) != 1 || items[0].QuarantineID != open.QuarantineID {
		t.Fatalf("expected only open quarantine, got %#v", items)
	}
	assignedItems := store.List(ListFilter{Status: StatusAssigned, StationID: "station_1", Limit: 10})
	if len(assignedItems) != 1 || assignedItems[0].QuarantineID != assigned.QuarantineID || assignedItems[0].AssignedSessionID != "sess_1" || assignedItems[0].AssignedPhotoID != "photo_1" {
		t.Fatalf("expected assigned item with assignment trace, got %#v", assignedItems)
	}
	if got := store.CountByStation("station_1"); got != 1 {
		t.Fatalf("expected open station count 1, got %d", got)
	}
	if got := store.CountByRelatedSession("sess_1"); got != 0 {
		t.Fatalf("expected assigned quarantine to be removed from open related count, got %d", got)
	}
}

func TestAssignIdempotentAndConflict(t *testing.T) {
	store := NewStore()
	now := time.Now()
	record := store.Quarantine("station_1", "", "C:/input/a.jpg", 100, now, now, now, ReasonNoActiveSession)
	assigned, err := store.Assign(record.QuarantineID, "sess_1", "photo_1", now)
	if err != nil {
		t.Fatalf("Assign returned error: %v", err)
	}
	again, err := store.Assign(record.QuarantineID, "sess_1", "photo_1", now.Add(time.Second))
	if err != nil {
		t.Fatalf("idempotent Assign returned error: %v", err)
	}
	if !again.AssignedAt.Equal(assigned.AssignedAt) || again.AssignedPhotoID != assigned.AssignedPhotoID {
		t.Fatalf("expected existing assignment result, got %#v", again)
	}
	_, err = store.Assign(record.QuarantineID, "sess_2", "photo_2", now.Add(2*time.Second))
	if !errors.Is(err, ErrAlreadyAssignedDifferentSession) {
		t.Fatalf("expected different-session conflict, got %v", err)
	}
}
