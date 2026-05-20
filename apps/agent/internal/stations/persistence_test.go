package stations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPersistenceSaveLoadRoundtrip(t *testing.T) {
	p := NewPersistence(t.TempDir())
	store := NewStore()
	updated, err := store.Update(Station1ID, UpdateStation{Name: "Main", DeviceIdentifier: "Sony", InputFolder: "D:/input/main", BackgroundName: "White", DefaultLUTPath: "D:/lut.cube", OutputRule: "{station_id}"})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := p.Save(store); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := p.LoadOrDefault()
	if err != nil {
		t.Fatalf("LoadOrDefault() error = %v", err)
	}
	got, err := loaded.Get(Station1ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != updated.Name || got.InputFolder != updated.InputFolder {
		t.Fatalf("loaded = %+v, want %+v", got, updated)
	}
}

func TestPersistenceSaveLoadCameraAssignmentRoundtrip(t *testing.T) {
	p := NewPersistence(t.TempDir())
	store := NewStore()
	_, err := store.UpdateCameraAssignment(Station1ID, UpdateCameraAssignment{IdentityKey: "wsl|usb:001,004|sony_alpha_a6000", CameraName: "Sony Alpha-A6000", Port: "usb:001,004", Runtime: "wsl", Connected: true})
	if err != nil {
		t.Fatalf("UpdateCameraAssignment() error = %v", err)
	}
	if err := p.Save(store); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := p.LoadOrDefault()
	if err != nil {
		t.Fatalf("LoadOrDefault() error = %v", err)
	}
	got, err := loaded.Get(Station1ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.CameraAssignment == nil || got.CameraAssignment.IdentityKey != "wsl|usb:001,004|sony_alpha_a6000" {
		t.Fatalf("assignment not loaded: %+v", got.CameraAssignment)
	}
}

func TestPersistenceOlderConfigWithoutAssignmentLoads(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config", "stations.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	payload := `{"version":1,"saved_at":"2026-05-20T00:00:00Z","stations":[{"station_id":"station_1","name":"Station 1","device_identifier":"Sony 1","input_folder":"D:/input/1","background_name":"White","default_lut_path":"D:/lut.cube","output_rule":"{station_id}"},{"station_id":"station_2","name":"Station 2","device_identifier":"Sony 2","input_folder":"D:/input/2","background_name":"White","default_lut_path":"D:/lut.cube","output_rule":"{station_id}"},{"station_id":"station_3","name":"Station 3","device_identifier":"Sony 3","input_folder":"D:/input/3","background_name":"White","default_lut_path":"D:/lut.cube","output_rule":"{station_id}"}]}`
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := NewPersistence(root).LoadOrDefault()
	if err != nil {
		t.Fatalf("LoadOrDefault() error = %v", err)
	}
	station, _ := store.Get(Station1ID)
	if station.CameraAssignment != nil {
		t.Fatalf("old config should have nil assignment: %+v", station.CameraAssignment)
	}
}

func TestPersistenceLoadMissingUsesDefaults(t *testing.T) {
	store, err := NewPersistence(t.TempDir()).LoadOrDefault()
	if err != nil {
		t.Fatalf("LoadOrDefault() error = %v", err)
	}
	if got := len(store.List()); got != 3 {
		t.Fatalf("len = %d", got)
	}
}

func TestPersistenceRejectsMalformedConfig(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config", "stations.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := NewPersistence(root).LoadOrDefault()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPersistenceBackupAndRestoreRollback(t *testing.T) {
	p := NewPersistence(t.TempDir())
	store := NewStore()
	original, err := store.Update(Station1ID, UpdateStation{Name: "Original", DeviceIdentifier: "Sony", InputFolder: "D:/input/original", BackgroundName: "White", DefaultLUTPath: "D:/lut.cube", OutputRule: "{station_id}"})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := p.Save(store); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	backup, err := p.Backup(store)
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	if _, err := store.Update(Station1ID, UpdateStation{Name: "Changed", DeviceIdentifier: "Sony", InputFolder: "D:/input/changed", BackgroundName: "White", DefaultLUTPath: "D:/lut.cube", OutputRule: "{station_id}"}); err != nil {
		t.Fatal(err)
	}
	count, err := p.Restore(store, backup.Filename)
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if count != 3 {
		t.Fatalf("count = %d", count)
	}
	got, _ := store.Get(Station1ID)
	if got.Name != original.Name || got.InputFolder != original.InputFolder {
		t.Fatalf("got %+v want %+v", got, original)
	}

	before := got
	bad := filepath.Join(p.backupDir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"version":1,"stations":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Restore(store, "bad.json"); err == nil {
		t.Fatal("expected restore error")
	}
	after, _ := store.Get(Station1ID)
	if after != before {
		t.Fatalf("restore rollback failed: before %+v after %+v", before, after)
	}
}

func TestPersistenceRejectsUnsafeRestoreFilename(t *testing.T) {
	p := NewPersistence(t.TempDir())
	if _, err := p.safeBackupPath("../stations.json"); err == nil {
		t.Fatal("expected traversal error")
	}
	if _, err := p.safeBackupPath("C:/tmp/stations.json"); err == nil {
		t.Fatal("expected absolute error")
	}
}
