package cameras

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTetherStateStoreMissingDefaultsSafe(t *testing.T) {
	store := NewTetherStateStoreAt(filepath.Join(t.TempDir(), "config", "tether_listeners.json"))
	got, err := store.Get("station_1")
	if err != nil {
		t.Fatal(err)
	}
	if got.DesiredState != TetherDesiredStopped || got.AutoRestartEnabled {
		t.Fatalf("unsafe default: %+v", got)
	}
}

func TestTetherStateStorePersistsDesiredAndAutoRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "tether_listeners.json")
	store := NewTetherStateStoreAt(path)
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	started, err := store.Update("station_1", func(s TetherListenerSettings) TetherListenerSettings {
		s.DesiredState = TetherDesiredRunning
		s.AutoRestartEnabled = true
		s.LastStartedAt = &now
		return s
	})
	if err != nil {
		t.Fatal(err)
	}
	if started.DesiredState != TetherDesiredRunning || !started.AutoRestartEnabled || started.LastStartedAt == nil {
		t.Fatalf("started=%+v", started)
	}
	reloaded := NewTetherStateStoreAt(path)
	got, err := reloaded.Get("station_1")
	if err != nil {
		t.Fatal(err)
	}
	if got.DesiredState != TetherDesiredRunning || !got.AutoRestartEnabled {
		t.Fatalf("reload=%+v", got)
	}
	stoppedAt := now.Add(time.Minute)
	stopped, err := reloaded.Update("station_1", func(s TetherListenerSettings) TetherListenerSettings {
		s.DesiredState = TetherDesiredStopped
		s.LastStoppedAt = &stoppedAt
		return s
	})
	if err != nil {
		t.Fatal(err)
	}
	if stopped.DesiredState != TetherDesiredStopped || stopped.LastStoppedAt == nil || !stopped.AutoRestartEnabled {
		t.Fatalf("stopped=%+v", stopped)
	}
}

func TestTetherStateStoreCorruptFileFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "tether_listeners.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"stations":`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewTetherStateStoreAt(path)
	if _, err := store.LoadAll(); err == nil {
		t.Fatal("expected corrupt settings error")
	}
}
