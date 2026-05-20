package readiness

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"selfstudio/agent/internal/stations"
)

func TestChecklistPlaceholderWhenStationsValidButPlaceholdersRemain(t *testing.T) {
	store, outputRoot := readyStore(t)
	checklist, err := NewBuilder(store, stations.NewReadinessValidator(outputRoot), outputRoot).Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if checklist.Status != StatusPlaceholder {
		t.Fatalf("Status = %q, want placeholder", checklist.Status)
	}
	if !checklist.SessionStartAvailable {
		t.Fatal("SessionStartAvailable = false, want true")
	}
	if item := findItem(t, checklist, CategoryOperator, "session_start"); item.Status != StatusReady {
		t.Fatalf("session_start status = %q", item.Status)
	}
	if item := findItem(t, checklist, CategoryCloud, "google_drive"); item.Status != StatusPlaceholder {
		t.Fatalf("google_drive status = %q", item.Status)
	}
	if item := findItem(t, checklist, CategoryStations, stations.Station1ID+".device"); item.Status != StatusUnknown {
		t.Fatalf("device status = %q", item.Status)
	}
}

func TestChecklistUnavailableWhenStoreNil(t *testing.T) {
	_, err := NewBuilder(nil, stations.NewReadinessValidator(t.TempDir()), t.TempDir()).Build()
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}

func TestChecklistFailedWhenStationLUTMissing(t *testing.T) {
	store, outputRoot := readyStore(t)
	station, err := store.Get(stations.Station1ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	station.DefaultLUTPath = filepath.Join(t.TempDir(), "missing.cube")
	if _, err := store.Update(stations.Station1ID, stations.UpdateStation{
		Name:             station.Name,
		DeviceIdentifier: station.DeviceIdentifier,
		InputFolder:      station.InputFolder,
		BackgroundName:   station.BackgroundName,
		DefaultLUTPath:   station.DefaultLUTPath,
		OutputRule:       station.OutputRule,
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	checklist, err := NewBuilder(store, stations.NewReadinessValidator(outputRoot), outputRoot).Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if checklist.Status != StatusFailed {
		t.Fatalf("Status = %q, want failed", checklist.Status)
	}
	if item := findItem(t, checklist, CategoryStations, stations.Station1ID+".default_lut"); item.Status != StatusFailed {
		t.Fatalf("station lut item status = %q", item.Status)
	}
}

func TestChecklistFailedWhenOutputRootMissing(t *testing.T) {
	store, outputRoot := readyStore(t)
	if err := os.RemoveAll(outputRoot); err != nil {
		t.Fatalf("RemoveAll() error = %v", err)
	}
	checklist, err := NewBuilder(store, stations.NewReadinessValidator(outputRoot), outputRoot).Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if checklist.Status != StatusFailed {
		t.Fatalf("Status = %q, want failed", checklist.Status)
	}
	if item := findItem(t, checklist, CategoryStorage, "local_output_root"); item.Status != StatusFailed {
		t.Fatalf("root storage item status = %q", item.Status)
	}
}

func TestChecklistFailedWhenOutputFolderMissing(t *testing.T) {
	store, outputRoot := readyStore(t)
	if err := os.RemoveAll(filepath.Join(outputRoot, stations.Station1ID)); err != nil {
		t.Fatalf("RemoveAll() error = %v", err)
	}
	checklist, err := NewBuilder(store, stations.NewReadinessValidator(outputRoot), outputRoot).Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if checklist.Status != StatusFailed {
		t.Fatalf("Status = %q, want failed", checklist.Status)
	}
	if item := findItem(t, checklist, CategoryStorage, stations.Station1ID+".output_folder"); item.Status != StatusFailed {
		t.Fatalf("storage item status = %q", item.Status)
	}
}

func readyStore(t *testing.T) (*stations.Store, string) {
	t.Helper()
	root := t.TempDir()
	outputRoot := filepath.Join(root, "output")
	store := stations.NewStore()
	for _, station := range store.List() {
		input := filepath.Join(root, station.StationID, "input")
		output := filepath.Join(outputRoot, station.StationID)
		lut := filepath.Join(root, station.StationID, "default.cube")
		if err := os.MkdirAll(input, 0o755); err != nil {
			t.Fatalf("MkdirAll(input) error = %v", err)
		}
		if err := os.MkdirAll(output, 0o755); err != nil {
			t.Fatalf("MkdirAll(output) error = %v", err)
		}
		if err := os.WriteFile(lut, []byte("TITLE test"), 0o644); err != nil {
			t.Fatalf("WriteFile(lut) error = %v", err)
		}
		if _, err := store.Update(station.StationID, stations.UpdateStation{
			Name:             station.Name,
			DeviceIdentifier: station.DeviceIdentifier,
			InputFolder:      input,
			BackgroundName:   station.BackgroundName,
			DefaultLUTPath:   lut,
			OutputRule:       "{station_id}",
		}); err != nil {
			t.Fatalf("Update(%s) error = %v", station.StationID, err)
		}
	}
	return store, outputRoot
}

func findItem(t *testing.T, checklist Checklist, category Category, key string) Item {
	t.Helper()
	for _, item := range checklist.Items {
		if item.Category == category && item.ItemKey == key {
			return item
		}
	}
	t.Fatalf("item %s/%s not found", category, key)
	return Item{}
}
