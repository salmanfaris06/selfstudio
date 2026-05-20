package cameras

import (
	"context"
	"sync"
	"time"
)

type TetherRecoveryState string

const (
	TetherRecoveryIdle       TetherRecoveryState = "idle"
	TetherRecoveryScheduled  TetherRecoveryState = "scheduled"
	TetherRecoveryAttempting TetherRecoveryState = "attempting"
	TetherRecoverySucceeded  TetherRecoveryState = "succeeded"
	TetherRecoveryFailed     TetherRecoveryState = "failed"
	TetherRecoveryPaused     TetherRecoveryState = "paused"
)

type TetherRecoveryStatus struct {
	StationID       string              `json:"station_id"`
	Status          TetherRecoveryState `json:"status"`
	AttemptCount    int                 `json:"attempt_count"`
	NextAttemptAt   *time.Time          `json:"next_attempt_at,omitempty"`
	LastErrorCode   string              `json:"last_error_code,omitempty"`
	LastErrorAction SafeAction          `json:"last_error_action,omitempty"`
	Message         string              `json:"message"`
	UpdatedAt       time.Time           `json:"updated_at"`
}

type TetherRecoveryNotifier interface {
	TetherRecoveryUpdated(TetherRecoveryStatus)
}

type TetherStationConfigProvider func(stationID string) (TetherStationConfig, bool)

type TetherRecoveryCoordinator struct {
	Supervisor *TetherSupervisor
	Store      *TetherStateStore
	ConfigFor  TetherStationConfigProvider
	Notifier   TetherRecoveryNotifier
	Now        func() time.Time
	AfterFunc  func(time.Duration, func()) *time.Timer

	mu      sync.Mutex
	states  map[string]TetherRecoveryStatus
	timers  map[string]*time.Timer
	backoff []time.Duration
	max     int
}

func NewTetherRecoveryCoordinator(supervisor *TetherSupervisor, store *TetherStateStore, configFor TetherStationConfigProvider, notifier TetherRecoveryNotifier) *TetherRecoveryCoordinator {
	r := &TetherRecoveryCoordinator{Supervisor: supervisor, Store: store, ConfigFor: configFor, Notifier: notifier, states: map[string]TetherRecoveryStatus{}, timers: map[string]*time.Timer{}, backoff: []time.Duration{time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second}, max: 5}
	r.Now = func() time.Time { return time.Now().UTC() }
	r.AfterFunc = time.AfterFunc
	return r
}

func (r *TetherRecoveryCoordinator) Status(stationID string) TetherRecoveryStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	if state, ok := r.states[stationID]; ok {
		return state
	}
	return TetherRecoveryStatus{StationID: stationID, Status: TetherRecoveryIdle, Message: "Tidak ada recovery tether aktif.", UpdatedAt: r.now()}
}

func (r *TetherRecoveryCoordinator) OnUnexpectedExit(stationID string, listener TetherListener) {
	settings, err := r.Store.Get(stationID)
	if err != nil || settings.DesiredState != TetherDesiredRunning || !settings.AutoRestartEnabled {
		r.set(TetherRecoveryStatus{StationID: stationID, Status: TetherRecoveryFailed, AttemptCount: 0, LastErrorCode: listener.LastErrorCode, LastErrorAction: listener.LastErrorAction, Message: "Tether listener butuh perhatian operator; auto-restart tidak aktif.", UpdatedAt: r.now()})
		return
	}
	r.Schedule(stationID, false)
}

func (r *TetherRecoveryCoordinator) StartupRecover() {
	items, err := r.Store.LoadAll()
	if err != nil {
		return
	}
	for _, item := range items {
		if item.DesiredState == TetherDesiredRunning && item.AutoRestartEnabled {
			r.Schedule(item.StationID, false)
		}
	}
}

func (r *TetherRecoveryCoordinator) ManualRetry(stationID string) {
	r.Schedule(stationID, true)
}

func (r *TetherRecoveryCoordinator) MarkManualAttemptFailed(stationID string, code string, action SafeAction, message string) {
	prev := r.Status(stationID)
	attempt := prev.AttemptCount
	if attempt < 1 {
		attempt = 1
	}
	r.failAndMaybeReschedule(stationID, attempt, code, action, message, true)
}

func (r *TetherRecoveryCoordinator) Cancel(stationID string) {
	r.mu.Lock()
	if timer := r.timers[stationID]; timer != nil {
		timer.Stop()
	}
	delete(r.timers, stationID)
	state := TetherRecoveryStatus{StationID: stationID, Status: TetherRecoveryIdle, Message: "Recovery tether dibatalkan karena listener dihentikan.", UpdatedAt: r.now()}
	r.states[stationID] = state
	r.mu.Unlock()
	r.notify(state)
}

func (r *TetherRecoveryCoordinator) Schedule(stationID string, manual bool) {
	r.mu.Lock()
	prev := r.states[stationID]
	attempt := prev.AttemptCount
	if manual {
		attempt = 0
	}
	if attempt >= r.max && !manual {
		state := TetherRecoveryStatus{StationID: stationID, Status: TetherRecoveryPaused, AttemptCount: attempt, LastErrorCode: prev.LastErrorCode, LastErrorAction: nonEmptyAction(prev.LastErrorAction, ActionRetryTetherListener), Message: "Recovery tether dijeda setelah beberapa percobaan gagal. Gunakan retry manual setelah cek kamera.", UpdatedAt: r.now()}
		r.states[stationID] = state
		r.mu.Unlock()
		r.notify(state)
		return
	}
	delay := time.Duration(0)
	if !manual && attempt > 0 {
		idx := attempt - 1
		if idx >= len(r.backoff) {
			idx = len(r.backoff) - 1
		}
		delay = r.backoff[idx]
	}
	next := r.now().Add(delay)
	state := TetherRecoveryStatus{StationID: stationID, Status: TetherRecoveryScheduled, AttemptCount: attempt, NextAttemptAt: &next, LastErrorAction: ActionRetryTetherListener, Message: "Reconnect tether dijadwalkan dengan backoff aman.", UpdatedAt: r.now()}
	if old := r.timers[stationID]; old != nil {
		old.Stop()
	}
	r.states[stationID] = state
	if r.AfterFunc != nil {
		r.timers[stationID] = r.AfterFunc(delay, func() { r.attempt(stationID, manual) })
	}
	r.mu.Unlock()
	r.notify(state)
}

func (r *TetherRecoveryCoordinator) attempt(stationID string, manual bool) {
	if !manual && !r.automaticRecoveryAllowed(stationID) {
		r.set(TetherRecoveryStatus{StationID: stationID, Status: TetherRecoveryFailed, AttemptCount: r.Status(stationID).AttemptCount, LastErrorAction: ActionRetryTetherListener, Message: "Tether listener butuh perhatian operator; auto-restart tidak aktif.", UpdatedAt: r.now()})
		return
	}
	r.mu.Lock()
	prev := r.states[stationID]
	attempt := prev.AttemptCount + 1
	state := TetherRecoveryStatus{StationID: stationID, Status: TetherRecoveryAttempting, AttemptCount: attempt, LastErrorAction: ActionRetryTetherListener, Message: "Mencoba reconnect tether listener secara aman.", UpdatedAt: r.now()}
	r.states[stationID] = state
	r.mu.Unlock()
	r.notify(state)
	cfg, ok := r.ConfigFor(stationID)
	if !ok {
		r.failAndMaybeReschedule(stationID, attempt, "STATION_NOT_FOUND", ActionRetryTetherListener, "Station tidak tersedia untuk recovery tether.", manual)
		return
	}
	listener, err := r.Supervisor.Start(context.Background(), cfg)
	if err == nil && (listener.Status == TetherStatusRunning || listener.AlreadyRunning) {
		r.set(TetherRecoveryStatus{StationID: stationID, Status: TetherRecoverySucceeded, AttemptCount: attempt, Message: "Tether listener berhasil reconnect.", UpdatedAt: r.now()})
		return
	}
	code := listener.LastErrorCode
	if code == "" {
		code = "TETHER_LISTENER_START_FAILED"
	}
	action := listener.LastErrorAction
	if action == "" {
		action = ActionRetryTetherListener
	}
	_ = err
	r.failAndMaybeReschedule(stationID, attempt, code, action, "Reconnect tether gagal aman; ikuti next action di dashboard.", manual)
}

func (r *TetherRecoveryCoordinator) failAndMaybeReschedule(stationID string, attempt int, code string, action SafeAction, message string, manual bool) {
	_, _ = r.Store.Update(stationID, func(s TetherListenerSettings) TetherListenerSettings {
		now := r.now()
		s.LastRecoveryAttemptAt = &now
		s.RecoveryAttemptCount = attempt
		return s
	})
	if code == "" {
		code = "TETHER_LISTENER_START_FAILED"
	}
	if action == "" {
		action = ActionRetryTetherListener
	}
	r.set(TetherRecoveryStatus{StationID: stationID, Status: TetherRecoveryFailed, AttemptCount: attempt, LastErrorCode: code, LastErrorAction: action, Message: message, UpdatedAt: r.now()})
	if r.automaticRecoveryAllowed(stationID) {
		r.Schedule(stationID, false)
	}
}

func (r *TetherRecoveryCoordinator) automaticRecoveryAllowed(stationID string) bool {
	settings, err := r.Store.Get(stationID)
	if err != nil {
		return false
	}
	return settings.DesiredState == TetherDesiredRunning && settings.AutoRestartEnabled
}

func (r *TetherRecoveryCoordinator) set(state TetherRecoveryStatus) {
	r.mu.Lock()
	r.states[state.StationID] = state
	r.mu.Unlock()
	r.notify(state)
}

func (r *TetherRecoveryCoordinator) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

func (r *TetherRecoveryCoordinator) notify(state TetherRecoveryStatus) {
	if r.Notifier != nil {
		r.Notifier.TetherRecoveryUpdated(state)
	}
}

func nonEmptyAction(action SafeAction, fallback SafeAction) SafeAction {
	if action == "" {
		return fallback
	}
	return action
}
