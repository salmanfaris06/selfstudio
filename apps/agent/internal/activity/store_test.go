package activity

import "testing"

func TestStoreRecordsRecentEntriesNewestFirst(t *testing.T) {
	store := NewStore(3)
	store.Record("login.success", ResultSuccess, "Operator login berhasil.")
	store.Record("health.recheck", ResultSuccess, "Health check dibuka.")

	entries := store.Recent(10, "")
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].ActionType != "health.recheck" {
		t.Fatalf("first action = %q", entries[0].ActionType)
	}
	if entries[0].ID == "" || entries[0].OccurredAt.IsZero() {
		t.Fatalf("entry missing id/time: %+v", entries[0])
	}
}

func TestStoreBoundsEntries(t *testing.T) {
	store := NewStore(2)
	store.Record("one", ResultSuccess, "one")
	store.Record("two", ResultSuccess, "two")
	store.Record("three", ResultSuccess, "three")

	entries := store.Recent(10, "")
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[1].ActionType != "two" {
		t.Fatalf("oldest kept action = %q", entries[1].ActionType)
	}
}

func TestStoreClampsLimitToCapacity(t *testing.T) {
	store := NewStore(2)
	store.Record("one", ResultSuccess, "one")
	store.Record("two", ResultSuccess, "two")

	entries := store.Recent(100, "")
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
}

func TestStoreRecordsOptionalReferences(t *testing.T) {
	store := NewStore(10)
	stationID := "station-1"
	sessionID := "sess_abc"
	entry := store.RecordWithRefs("session.updated", ResultSuccess, "Session updated.", &stationID, &sessionID)

	if entry.StationID == nil || *entry.StationID != stationID {
		t.Fatalf("StationID = %+v", entry.StationID)
	}
	if entry.SessionID == nil || *entry.SessionID != sessionID {
		t.Fatalf("SessionID = %+v", entry.SessionID)
	}
}

func TestStoreFiltersByActionType(t *testing.T) {
	store := NewStore(10)
	store.Record("login.success", ResultSuccess, "Operator login berhasil.")
	store.Record("login.failure", ResultFailure, "Login gagal.")

	entries := store.Recent(10, "login.failure")
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].ActionType != "login.failure" || entries[0].Result != ResultFailure {
		t.Fatalf("unexpected filtered entry: %+v", entries[0])
	}
}
