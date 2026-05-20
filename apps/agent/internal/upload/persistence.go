package upload

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const stateVersion = 1

type stateFile struct {
	Version int                  `json:"version"`
	SavedAt time.Time            `json:"saved_at"`
	Targets []SessionCloudTarget `json:"targets"`
}

type Persistence struct{ path string }

func NewPersistence(localDataDir string) Persistence {
	return Persistence{path: filepath.Join(localDataDir, "state", "upload_targets.json")}
}
func (p Persistence) Path() string { return p.path }

func (p Persistence) LoadOrDefault() (*Store, error) {
	if _, err := os.Stat(p.path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewStore(), nil
		}
		return nil, err
	}
	data, err := os.ReadFile(p.path)
	if err != nil {
		return nil, err
	}
	var f stateFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	if f.Version != stateVersion {
		return nil, errors.New("invalid upload target state version")
	}
	return NewStoreFromRecords(f.Targets)
}

func (p Persistence) Save(store *Store) error {
	if store == nil {
		return errors.New("upload target store is nil")
	}
	if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(stateFile{Version: stateVersion, SavedAt: time.Now().UTC(), Targets: store.List()}, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p.path), ".upload-targets-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	_ = tmp.Chmod(0o600)
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		_ = os.Remove(p.path)
	}
	return os.Rename(name, p.path)
}

type Store struct{ targets map[string]SessionCloudTarget }

func NewStore() *Store { return &Store{targets: map[string]SessionCloudTarget{}} }
func NewStoreFromRecords(records []SessionCloudTarget) (*Store, error) {
	s := NewStore()
	seen := map[string]bool{}
	for _, r := range records {
		if err := validateTarget(r); err != nil {
			return nil, err
		}
		if seen[r.SessionID] {
			return nil, errors.New("duplicate upload target session_id")
		}
		seen[r.SessionID] = true
		s.targets[r.SessionID] = r
	}
	return s, nil
}
func (s *Store) Get(sessionID string) (SessionCloudTarget, bool) {
	t, ok := s.targets[sessionID]
	return t, ok
}
func (s *Store) Upsert(t SessionCloudTarget) error {
	if err := validateTarget(t); err != nil {
		return err
	}
	s.targets[t.SessionID] = t
	return nil
}
func (s *Store) List() []SessionCloudTarget {
	out := make([]SessionCloudTarget, 0, len(s.targets))
	for _, t := range s.targets {
		out = append(out, t)
	}
	return out
}

func validateTarget(t SessionCloudTarget) error {
	if t.SessionID == "" || t.StationID == "" {
		return errors.New("upload target missing session/station id")
	}
	switch t.Status {
	case StatusPending, StatusResolving, StatusReady, StatusFailed:
	default:
		return errors.New("invalid upload target status")
	}
	if t.Status == StatusReady && (t.RemoteIdentity == "" || (t.ObjectPrefix == "" && t.DriveFolderPath == "")) {
		return errors.New("ready upload target missing identity")
	}
	if t.ObjectPrefix != "" {
		if err := ValidateObjectPrefix(t.ObjectPrefix); err != nil {
			return err
		}
	}
	return nil
}
