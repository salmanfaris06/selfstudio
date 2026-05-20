package sessions

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const stateVersion = 1

type StateFile struct {
	Version  int       `json:"version"`
	SavedAt  time.Time `json:"saved_at"`
	Sessions []Session `json:"sessions"`
}

type Persistence struct {
	path string
}

func NewPersistence(localDataDir string) Persistence {
	return Persistence{path: filepath.Join(localDataDir, "state", "sessions.json")}
}

func (p Persistence) OutputRoot() string {
	return filepath.Dir(filepath.Dir(p.path))
}

func (p Persistence) LoadOrDefault() (*Store, error) {
	if _, err := os.Stat(p.path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewStore(), nil
		}
		return nil, err
	}
	items, err := p.read()
	if err != nil {
		return nil, err
	}
	store := NewStore()
	if err := store.ReplaceAll(items); err != nil {
		return nil, err
	}
	return store, nil
}

func (p Persistence) Save(store *Store) error {
	return p.write(store.List())
}

func (p Persistence) write(items []Session) error {
	if err := validateSessions(items); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
		return err
	}
	payload := StateFile{Version: stateVersion, SavedAt: time.Now().UTC(), Sessions: items}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p.path), ".sessions-*.tmp")
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
	if err := replaceFile(tmpName, p.path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(p.path))
}

func (p Persistence) read() ([]Session, error) {
	data, err := os.ReadFile(p.path)
	if err != nil {
		return nil, err
	}
	var file StateFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, ErrInvalidSession
	}
	if file.Version != stateVersion {
		return nil, ErrInvalidSession
	}
	if err := validateSessions(file.Sessions); err != nil {
		return nil, err
	}
	return file.Sessions, nil
}

func replaceFile(source string, target string) error {
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(source, target)
}

func syncDir(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
