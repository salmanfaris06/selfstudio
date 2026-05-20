package ingestion

import (
	"testing"
	"time"

	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/quarantine"
	"selfstudio/agent/internal/sessions"
	"selfstudio/agent/internal/stations"
)

func TestRouterRoutesOnlyToActiveSessionForSameStation(t *testing.T) {
	now := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	sessionStore := sessions.NewStore()
	station := stations.Station{StationID: stations.Station1ID, Name: "S1", OutputRule: "{station_id}"}
	session, err := sessionStore.Start(station, sessions.StartSessionRequest{CustomerName: "A", OrderNumber: "O", TimerSeconds: 120}, t.TempDir(), now)
	if err != nil {
		t.Fatal(err)
	}
	router := NewRouter(sessionStore, photos.NewStore())
	result := router.Route(DetectedPhoto{StationID: stations.Station1ID, SourcePath: "a.jpg", SizeBytes: 3, DetectedAt: now, StableAt: now}, now.Add(time.Second))
	if result.Status != PhotoStatus(photos.StatusRouted) || result.SessionID != session.SessionID || result.PhotoID == "" {
		t.Fatalf("unexpected route result: %+v", result)
	}
	other := router.Route(DetectedPhoto{StationID: stations.Station2ID, SourcePath: "b.jpg", SizeBytes: 3, DetectedAt: now, StableAt: now}, now.Add(time.Second))
	if other.Status != PhotoUnassignedPendingQuarantine || other.SessionID != "" {
		t.Fatalf("expected no cross-station routing without quarantine store: %+v", other)
	}
}

func TestRouterQuarantinesNoActiveSession(t *testing.T) {
	now := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	store := quarantine.NewStore()
	router := NewRouterWithQuarantine(sessions.NewStore(), photos.NewStore(), store)
	result := router.Route(DetectedPhoto{StationID: stations.Station1ID, SourcePath: "a.jpg", SizeBytes: 3, DetectedAt: now, StableAt: now}, now)
	if result.Status != PhotoStatus(quarantine.StatusQuarantined) || result.Reason != quarantine.ReasonNoActiveSession || result.QuarantineID == "" || result.SessionID != "" {
		t.Fatalf("expected no-active-session quarantine: %+v", result)
	}
	if count := store.CountByStation(stations.Station1ID); count != 1 {
		t.Fatalf("count=%d", count)
	}
}

func TestRouterQuarantinesExpiredSessionAsLatePhoto(t *testing.T) {
	now := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	sessionStore := sessions.NewStore()
	session, err := sessionStore.Start(stations.Station{StationID: stations.Station1ID, Name: "S1", OutputRule: "{station_id}"}, sessions.StartSessionRequest{CustomerName: "A", OrderNumber: "O", TimerSeconds: 60}, t.TempDir(), now)
	if err != nil {
		t.Fatal(err)
	}
	photoStore := photos.NewStore()
	router := NewRouterWithQuarantine(sessionStore, photoStore, quarantine.NewStore())
	result := router.Route(DetectedPhoto{StationID: stations.Station1ID, SourcePath: "a.jpg", SizeBytes: 3, DetectedAt: now.Add(61 * time.Second), StableAt: now.Add(61 * time.Second)}, now.Add(61*time.Second))
	if result.Status != PhotoStatus(quarantine.StatusQuarantined) || result.SessionID != "" || result.RelatedSessionID != session.SessionID || result.Reason != quarantine.ReasonLatePhoto || result.QuarantineID == "" {
		t.Fatalf("expected expired session late quarantine: %+v", result)
	}
	if count := photoStore.CountBySession(session.SessionID); count != 0 {
		t.Fatalf("expired session received routed photo count=%d", count)
	}
}

func TestRouterDoesNotDuplicateSameFileWithDifferentDetectedAt(t *testing.T) {
	now := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	sessionStore := sessions.NewStore()
	_, err := sessionStore.Start(stations.Station{StationID: stations.Station1ID, Name: "S1", OutputRule: "{station_id}"}, sessions.StartSessionRequest{CustomerName: "A", OrderNumber: "O", TimerSeconds: 120}, t.TempDir(), now)
	if err != nil {
		t.Fatal(err)
	}
	photoStore := photos.NewStore()
	router := NewRouter(sessionStore, photoStore)
	stableAt := now.Add(5 * time.Second)

	first := router.Route(DetectedPhoto{StationID: stations.Station1ID, SourcePath: `C:\Input\A.JPG`, SizeBytes: 3, DetectedAt: now, StableAt: stableAt}, now.Add(6*time.Second))
	second := router.Route(DetectedPhoto{StationID: stations.Station1ID, SourcePath: `C:\Input\A.JPG`, SizeBytes: 3, DetectedAt: now.Add(30 * time.Second), StableAt: stableAt}, now.Add(31*time.Second))

	if first.PhotoID == "" || first.PhotoID != second.PhotoID {
		t.Fatalf("expected same stable file identity to return one routed photo, got %+v and %+v", first, second)
	}
	if !second.Duplicate {
		t.Fatalf("expected duplicate route result for same file with later detectedAt")
	}
	if count := photoStore.CountBySession(first.SessionID); count != 1 {
		t.Fatalf("count=%d", count)
	}
}

func TestRouterDoesNotDuplicateQuarantine(t *testing.T) {
	now := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	store := quarantine.NewStore()
	router := NewRouterWithQuarantine(sessions.NewStore(), photos.NewStore(), store)
	first := router.Route(DetectedPhoto{StationID: stations.Station1ID, SourcePath: `C:\Input\A.JPG`, SizeBytes: 3, DetectedAt: now, StableAt: now}, now)
	second := router.Route(DetectedPhoto{StationID: stations.Station1ID, SourcePath: `C:\Input\A.JPG`, SizeBytes: 3, DetectedAt: now.Add(time.Minute), StableAt: now.Add(time.Minute)}, now.Add(time.Minute))
	if first.QuarantineID == "" || first.QuarantineID != second.QuarantineID || !second.Duplicate {
		t.Fatalf("expected duplicate quarantine result: %+v and %+v", first, second)
	}
	if count := store.CountByStation(stations.Station1ID); count != 1 {
		t.Fatalf("count=%d", count)
	}
}
