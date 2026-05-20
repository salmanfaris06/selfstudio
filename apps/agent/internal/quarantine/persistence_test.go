package quarantine

import (
	"testing"
	"time"
)

func TestPersistenceRoundTripPreservesAssignedStateAndOpenCounts(t *testing.T) {
	now := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	store := NewStore()
	open := store.Quarantine("station_1", "", `C:\Input\A.JPG`, 3, now, now, now, ReasonNoActiveSession)
	assigned := store.Quarantine("station_1", "sess_locked", `C:\Input\B.JPG`, 4, now, now, now, ReasonLatePhoto)
	if _, err := store.Assign(assigned.QuarantineID, "sess_locked", "photo_1", now.Add(time.Minute)); err != nil {
		t.Fatalf("assign: %v", err)
	}
	p := NewPersistence(t.TempDir())
	if err := p.Save(store); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := p.LoadOrDefault()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := loaded.CountByStation("station_1"); got != 1 {
		t.Fatalf("open count=%d", got)
	}
	if _, err := loaded.Get(open.QuarantineID); err != nil {
		t.Fatalf("open missing: %v", err)
	}
	reloadedAssigned, err := loaded.Get(assigned.QuarantineID)
	if err != nil {
		t.Fatalf("assigned missing: %v", err)
	}
	if reloadedAssigned.Status != StatusAssigned || reloadedAssigned.AssignedSessionID != "sess_locked" || reloadedAssigned.PhotoID != "photo_1" {
		t.Fatalf("assignment lost: %+v", reloadedAssigned)
	}
	again := loaded.Quarantine("station_1", "sess_locked", `c:\input\b.jpg`, 4, now.Add(time.Hour), now.Add(time.Hour), now.Add(time.Hour), ReasonLatePhoto)
	if again.QuarantineID != assigned.QuarantineID || !again.Duplicate || again.Status != StatusAssigned {
		t.Fatalf("expected assigned duplicate, got %+v", again)
	}
}

func TestNewStoreFromRecordsRejectsInvalidAssignedRecord(t *testing.T) {
	now := time.Now().UTC()
	record := Record{QuarantineID: "quar_1", StationID: "station_1", SourcePath: "a.jpg", SourceSizeBytes: 3, DetectedAt: now, StableAt: now, QuarantinedAt: now, Reason: ReasonNoActiveSession, Status: StatusAssigned}
	if _, err := NewStoreFromRecords([]Record{record}); err == nil {
		t.Fatal("expected invalid assigned state")
	}
}
