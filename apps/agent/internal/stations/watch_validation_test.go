package stations

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWatchValidatorDetectsStableJPG(t *testing.T) {
	store := NewStore()
	input := t.TempDir()
	file := filepath.Join(input, "test.JPG")
	content := []byte("jpg")
	if err := os.WriteFile(file, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Update(Station1ID, UpdateStation{Name: "Station 1", DeviceIdentifier: "Camera", InputFolder: input, BackgroundName: "White", DefaultLUTPath: "D:/lut.cube", OutputRule: "{station_id}"}); err != nil {
		t.Fatal(err)
	}
	result, err := NewWatchValidator(store).Run(context.Background(), Station1ID, WatchValidationRequest{TimeoutMs: 1000, StabilityMs: 200})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != ValidationStatusSuccess || result.SourcePath == nil || !result.ValidationOnly {
		t.Fatalf("result=%+v", result)
	}
	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("file mutated: %q", got)
	}
}

func TestWatchValidatorNoJPGFails(t *testing.T) {
	store := NewStore()
	input := t.TempDir()
	if _, err := store.Update(Station1ID, UpdateStation{Name: "Station 1", DeviceIdentifier: "Camera", InputFolder: input, BackgroundName: "White", DefaultLUTPath: "D:/lut.cube", OutputRule: "{station_id}"}); err != nil {
		t.Fatal(err)
	}
	result, err := NewWatchValidator(store).Run(context.Background(), Station1ID, WatchValidationRequest{TimeoutMs: 1000, StabilityMs: 200})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != ValidationStatusFailed || result.SourcePath != nil {
		t.Fatalf("result=%+v", result)
	}
}

func TestWatchValidatorMissingFolder(t *testing.T) {
	store := NewStore()
	if _, err := store.Update(Station1ID, UpdateStation{Name: "Station 1", DeviceIdentifier: "Camera", InputFolder: filepath.Join(t.TempDir(), "missing"), BackgroundName: "White", DefaultLUTPath: "D:/lut.cube", OutputRule: "{station_id}"}); err != nil {
		t.Fatal(err)
	}
	_, err := NewWatchValidator(store).Run(context.Background(), Station1ID, WatchValidationRequest{TimeoutMs: 1000, StabilityMs: 200})
	if err == nil {
		t.Fatal("expected missing folder error")
	}
}
