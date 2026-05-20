package cameras

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type recoveryNotify struct{ states []TetherRecoveryStatus }

func (n *recoveryNotify) TetherRecoveryUpdated(s TetherRecoveryStatus) {
	n.states = append(n.states, s)
}

type recoveryFailStarter struct{ starts int }

func (s *recoveryFailStarter) Start(ctx context.Context, spec TetherCommandSpec, output func(string)) (TetherProcess, error) {
	s.starts++
	return nil, errors.New("no camera")
}

func TestRecoverySchedulesOnlyWhenDesiredRunningAndEnabled(t *testing.T) {
	store := NewTetherStateStoreAt(filepath.Join(t.TempDir(), "tether.json"))
	_, _ = store.Update("station_1", func(s TetherListenerSettings) TetherListenerSettings {
		s.DesiredState = TetherDesiredRunning
		s.AutoRestartEnabled = true
		return s
	})
	_, _ = store.Update("station_2", func(s TetherListenerSettings) TetherListenerSettings { s.DesiredState = TetherDesiredRunning; return s })
	notify := &recoveryNotify{}
	r := NewTetherRecoveryCoordinator(NewTetherSupervisor(&fakeTetherStarter{}), store, func(stationID string) (TetherStationConfig, bool) { return testTetherConfig(t, stationID), true }, notify)
	timers := []func(){}
	r.AfterFunc = func(d time.Duration, f func()) *time.Timer {
		timers = append(timers, f)
		return time.NewTimer(time.Hour)
	}
	r.StartupRecover()
	if r.Status("station_1").Status != TetherRecoveryScheduled {
		t.Fatalf("station_1 not scheduled: %+v", r.Status("station_1"))
	}
	if r.Status("station_2").Status == TetherRecoveryScheduled {
		t.Fatalf("station_2 should not schedule")
	}
	if len(timers) != 1 || len(notify.states) == 0 {
		t.Fatalf("timers=%d notify=%d", len(timers), len(notify.states))
	}
}

func TestRecoveryBackoffAndPausePreventsTightLoop(t *testing.T) {
	store := NewTetherStateStoreAt(filepath.Join(t.TempDir(), "tether.json"))
	_, _ = store.Update("station_1", func(s TetherListenerSettings) TetherListenerSettings {
		s.DesiredState = TetherDesiredRunning
		s.AutoRestartEnabled = true
		return s
	})
	starter := &recoveryFailStarter{}
	r := NewTetherRecoveryCoordinator(NewTetherSupervisor(starter), store, func(stationID string) (TetherStationConfig, bool) { return testTetherConfig(t, stationID), true }, nil)
	delays := []time.Duration{}
	timers := []func(){}
	r.AfterFunc = func(d time.Duration, f func()) *time.Timer {
		delays = append(delays, d)
		timers = append(timers, f)
		return nil
	}
	r.Schedule("station_1", false)
	for i := 0; i < 5; i++ {
		timers[i]()
	}
	state := r.Status("station_1")
	if state.Status != TetherRecoveryPaused || starter.starts != 5 {
		t.Fatalf("state=%+v starts=%d", state, starter.starts)
	}
	if len(delays) < 5 || delays[1] <= 0 || delays[2] <= delays[1] {
		t.Fatalf("backoff delays not increasing: %+v", delays)
	}
}

func TestRecoveryScheduledAttemptRechecksSettingsBeforeStart(t *testing.T) {
	store := NewTetherStateStoreAt(filepath.Join(t.TempDir(), "tether.json"))
	_, _ = store.Update("station_1", func(s TetherListenerSettings) TetherListenerSettings {
		s.DesiredState = TetherDesiredRunning
		s.AutoRestartEnabled = true
		return s
	})
	starter := &fakeTetherStarter{}
	r := NewTetherRecoveryCoordinator(NewTetherSupervisor(starter), store, func(stationID string) (TetherStationConfig, bool) { return testTetherConfig(t, stationID), true }, nil)
	var timer func()
	r.AfterFunc = func(d time.Duration, f func()) *time.Timer {
		timer = f
		return time.NewTimer(time.Hour)
	}
	r.Schedule("station_1", false)
	_, _ = store.Update("station_1", func(s TetherListenerSettings) TetherListenerSettings {
		s.AutoRestartEnabled = false
		return s
	})
	timer()
	if starter.count() != 0 {
		t.Fatalf("scheduled automatic attempt started after auto-restart disabled: starts=%d", starter.count())
	}
	state := r.Status("station_1")
	if state.Status != TetherRecoveryFailed {
		t.Fatalf("expected actionable failed state after disabled gate, got %+v", state)
	}
}

func TestRecoveryManualRetryFailureIsOneShotWhenAutoRestartDisabled(t *testing.T) {
	store := NewTetherStateStoreAt(filepath.Join(t.TempDir(), "tether.json"))
	_, _ = store.Update("station_1", func(s TetherListenerSettings) TetherListenerSettings {
		s.DesiredState = TetherDesiredRunning
		s.AutoRestartEnabled = false
		return s
	})
	starter := &recoveryFailStarter{}
	r := NewTetherRecoveryCoordinator(NewTetherSupervisor(starter), store, func(stationID string) (TetherStationConfig, bool) { return testTetherConfig(t, stationID), true }, nil)
	timers := []func(){}
	r.AfterFunc = func(d time.Duration, f func()) *time.Timer {
		timers = append(timers, f)
		return time.NewTimer(time.Hour)
	}
	r.ManualRetry("station_1")
	timers[0]()
	if starter.starts != 1 {
		t.Fatalf("manual retry should attempt exactly once, starts=%d", starter.starts)
	}
	if len(timers) != 1 {
		t.Fatalf("manual retry failure should not schedule automatic retry when disabled, timers=%d", len(timers))
	}
	state := r.Status("station_1")
	if state.Status != TetherRecoveryFailed || state.AttemptCount != 1 {
		t.Fatalf("state=%+v", state)
	}
}

func TestRecoveryManualRetryResetsPausedBackoffAndDuplicateStartNoop(t *testing.T) {
	store := NewTetherStateStoreAt(filepath.Join(t.TempDir(), "tether.json"))
	_, _ = store.Update("station_1", func(s TetherListenerSettings) TetherListenerSettings {
		s.DesiredState = TetherDesiredRunning
		s.AutoRestartEnabled = false
		return s
	})
	starter := &fakeTetherStarter{}
	sup := NewTetherSupervisor(starter)
	cfg := testTetherConfig(t, "station_1")
	_, _ = sup.Start(context.Background(), cfg)
	r := NewTetherRecoveryCoordinator(sup, store, func(stationID string) (TetherStationConfig, bool) { return cfg, true }, nil)
	timers := []func(){}
	r.AfterFunc = func(d time.Duration, f func()) *time.Timer {
		timers = append(timers, f)
		return time.NewTimer(time.Hour)
	}
	r.ManualRetry("station_1")
	timers[0]()
	if starter.count() != 1 {
		t.Fatalf("manual retry duplicated process starts=%d", starter.count())
	}
	if r.Status("station_1").Status != TetherRecoverySucceeded {
		t.Fatalf("status=%+v", r.Status("station_1"))
	}
}

func TestRecoveryPackageNoForbiddenCommandsOrIngestionImports(t *testing.T) {
	files := []string{"tether_recovery.go", "tether_supervisor.go", "gphoto_runner.go"}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := strings.ToLower(string(data))
		for _, forbidden := range []string{"usbipd bind", "usbipd attach", "winget", "choco", "apt install", "powershell", "cmd /c", "taskkill", "pkill", "internal/photos", "internal/sessions", "internal/processing", "internal/upload"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s contains forbidden token %q", file, forbidden)
			}
		}
	}
}
