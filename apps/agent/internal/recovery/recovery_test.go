package recovery

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"selfstudio/agent/internal/ingestion"
	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/quarantine"
	"selfstudio/agent/internal/sessions"
	"selfstudio/agent/internal/stations"
)

func TestRecoveryScanSameFileAfterReloadDoesNotDuplicateRoutedPhoto(t *testing.T) {
	now := time.Now().UTC()
	input := t.TempDir()
	writeJPG(t, filepath.Join(input, "a.jpg"))
	stationStore := stations.NewStore()
	station := testStation(input)
	if err := stationStore.ReplaceAll([]stations.Station{station, testStationWithID(stations.Station2ID, t.TempDir()), testStationWithID(stations.Station3ID, t.TempDir())}); err != nil {
		t.Fatalf("stations: %v", err)
	}
	sessionStore := sessions.NewStore()
	session, err := sessionStore.Start(station, sessions.StartSessionRequest{CustomerName: "Customer", OrderNumber: "ORD", TimerSeconds: 900}, t.TempDir(), now)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	photoStore := photos.NewStore()
	photoStore.Route(stations.Station1ID, session.SessionID, filepath.Join(input, "a.jpg"), 3, now, now, now)
	loadedPhotos, err := photos.NewStoreFromRecords(photoStore.ListAll())
	if err != nil {
		t.Fatalf("load photos: %v", err)
	}
	summary := NewService(ingestion.NewScanner(stationStore), ingestion.NewRouterWithQuarantine(sessionStore, loadedPhotos, quarantine.NewStore()), nil, nil).Run(context.Background(), now.Add(time.Second))
	if summary.SkippedDuplicates != 1 || loadedPhotos.CountBySession(session.SessionID) != 1 {
		t.Fatalf("summary=%+v count=%d", summary, loadedPhotos.CountBySession(session.SessionID))
	}
}

func TestRecoveryNoActiveSessionQuarantinesAfterReload(t *testing.T) {
	now := time.Now().UTC()
	input := t.TempDir()
	writeJPG(t, filepath.Join(input, "a.jpg"))
	stationStore := stations.NewStore()
	if err := stationStore.ReplaceAll([]stations.Station{testStation(input), testStationWithID(stations.Station2ID, t.TempDir()), testStationWithID(stations.Station3ID, t.TempDir())}); err != nil {
		t.Fatalf("stations: %v", err)
	}
	quarantineStore := quarantine.NewStore()
	summary := NewService(ingestion.NewScanner(stationStore), ingestion.NewRouterWithQuarantine(sessions.NewStore(), photos.NewStore(), quarantineStore), nil, nil).Run(context.Background(), now)
	if summary.RecoveredQuarantineItems != 1 || quarantineStore.CountByStation(stations.Station1ID) != 1 {
		t.Fatalf("summary=%+v", summary)
	}
}

func testStation(input string) stations.Station {
	return testStationWithID(stations.Station1ID, input)
}

func testStationWithID(id string, input string) stations.Station {
	return stations.Station{StationID: id, Name: id, DeviceIdentifier: id, InputFolder: input, BackgroundName: "Default", DefaultLUTPath: filepath.Join(input, "default.cube"), OutputRule: "{station_id}"}
}

func writeJPG(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte{1, 2, 3}, 0o644); err != nil {
		t.Fatalf("write jpg: %v", err)
	}
}
