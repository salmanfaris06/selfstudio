package cameras

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type TetherDesiredState string

const (
	TetherDesiredStopped TetherDesiredState = "stopped"
	TetherDesiredRunning TetherDesiredState = "running"
)

type TetherListenerSettings struct {
	StationID             string             `json:"station_id"`
	DesiredState          TetherDesiredState `json:"desired_state"`
	AutoRestartEnabled    bool               `json:"auto_restart_enabled"`
	LastStartedAt         *time.Time         `json:"last_started_at,omitempty"`
	LastStoppedAt         *time.Time         `json:"last_stopped_at,omitempty"`
	LastRecoveryAttemptAt *time.Time         `json:"last_recovery_attempt_at,omitempty"`
	RecoveryAttemptCount  int                `json:"recovery_attempt_count,omitempty"`
	UpdatedAt             time.Time          `json:"updated_at"`
}

type tetherSettingsFile struct {
	Version  int                      `json:"version"`
	SavedAt  time.Time                `json:"saved_at"`
	Stations []TetherListenerSettings `json:"stations"`
}

type TetherStateStore struct {
	path string
	mu   sync.Mutex
	now  func() time.Time
}

func NewTetherStateStore(localDataDir string) *TetherStateStore {
	return NewTetherStateStoreAt(filepath.Join(localDataDir, "config", "tether_listeners.json"))
}

func NewTetherStateStoreAt(path string) *TetherStateStore {
	return &TetherStateStore{path: path, now: func() time.Time { return time.Now().UTC() }}
}

func (s *TetherStateStore) Get(stationID string) (TetherListenerSettings, error) {
	items, err := s.LoadAll()
	if err != nil {
		return TetherListenerSettings{}, err
	}
	for _, item := range items {
		if item.StationID == stationID {
			return item, nil
		}
	}
	return defaultTetherSettings(stationID, s.now()), nil
}

func (s *TetherStateStore) LoadAll() ([]TetherListenerSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []TetherListenerSettings{}, nil
		}
		return nil, err
	}
	var file tetherSettingsFile
	if err := json.Unmarshal(data, &file); err != nil || file.Version != 1 {
		return nil, errors.New("invalid tether listener settings")
	}
	for i := range file.Stations {
		file.Stations[i] = normalizeTetherSettings(file.Stations[i], s.now())
	}
	return file.Stations, nil
}

func (s *TetherStateStore) Update(stationID string, mutate func(TetherListenerSettings) TetherListenerSettings) (TetherListenerSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.loadAllLocked()
	if err != nil {
		return TetherListenerSettings{}, err
	}
	idx := -1
	for i := range items {
		if items[i].StationID == stationID {
			idx = i
			break
		}
	}
	current := defaultTetherSettings(stationID, s.now())
	if idx >= 0 {
		current = items[idx]
	}
	next := normalizeTetherSettings(mutate(current), s.now())
	next.StationID = stationID
	next.UpdatedAt = s.now()
	if idx >= 0 {
		items[idx] = next
	} else {
		items = append(items, next)
	}
	if err := s.saveAllLocked(items); err != nil {
		return TetherListenerSettings{}, err
	}
	return next, nil
}

func (s *TetherStateStore) loadAllLocked() ([]TetherListenerSettings, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []TetherListenerSettings{}, nil
		}
		return nil, err
	}
	var file tetherSettingsFile
	if err := json.Unmarshal(data, &file); err != nil || file.Version != 1 {
		return nil, errors.New("invalid tether listener settings")
	}
	for i := range file.Stations {
		file.Stations[i] = normalizeTetherSettings(file.Stations[i], s.now())
	}
	return file.Stations, nil
}

func (s *TetherStateStore) saveAllLocked(items []TetherListenerSettings) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	payload := tetherSettingsFile{Version: 1, SavedAt: s.now(), Stations: items}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".tether-listeners-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(tmpName, s.path)
}

func defaultTetherSettings(stationID string, now time.Time) TetherListenerSettings {
	return TetherListenerSettings{StationID: stationID, DesiredState: TetherDesiredStopped, AutoRestartEnabled: false, UpdatedAt: now.UTC()}
}

func normalizeTetherSettings(s TetherListenerSettings, now time.Time) TetherListenerSettings {
	if s.DesiredState != TetherDesiredRunning {
		s.DesiredState = TetherDesiredStopped
	}
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = now.UTC()
	}
	return s
}
