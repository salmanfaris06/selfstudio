package quarantine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const stateVersion = 1

var ErrInvalidQuarantineState = errors.New("invalid quarantine state")

type StateFile struct {
	Version int       `json:"version"`
	SavedAt time.Time `json:"saved_at"`
	Items   []Record  `json:"items"`
}

type Persistence struct{ path string }

func NewPersistence(localDataDir string) Persistence {
	return Persistence{path: filepath.Join(localDataDir, "state", "quarantine.json")}
}

func (p Persistence) Path() string { return p.path }

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
	return NewStoreFromRecords(items)
}

func (p Persistence) Save(store *Store) error { return p.write(store.ListAll()) }

func (p Persistence) write(items []Record) error {
	if err := validateRecords(items); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
		return err
	}
	payload := StateFile{Version: stateVersion, SavedAt: time.Now().UTC(), Items: items}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p.path), ".quarantine-*.tmp")
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

func (p Persistence) read() ([]Record, error) {
	data, err := os.ReadFile(p.path)
	if err != nil {
		return nil, err
	}
	var file StateFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, ErrInvalidQuarantineState
	}
	if file.Version != stateVersion {
		return nil, ErrInvalidQuarantineState
	}
	if err := validateRecords(file.Items); err != nil {
		return nil, err
	}
	return file.Items, nil
}

func validateRecords(items []Record) error {
	ids := map[string]bool{}
	identities := map[string]bool{}
	for _, item := range items {
		if item.QuarantineID == "" || item.StationID == "" || item.SourcePath == "" || item.SourceSizeBytes <= 0 {
			return fmt.Errorf("%w: required quarantine fields missing", ErrInvalidQuarantineState)
		}
		if item.Status != StatusQuarantined && item.Status != StatusAssigned {
			return fmt.Errorf("%w: invalid status", ErrInvalidQuarantineState)
		}
		if item.Reason != ReasonNoActiveSession && item.Reason != ReasonLatePhoto {
			return fmt.Errorf("%w: invalid reason", ErrInvalidQuarantineState)
		}
		if item.Status == StatusAssigned && (item.AssignedSessionID == "" || item.AssignedPhotoID == "" || item.PhotoID == "") {
			return fmt.Errorf("%w: assigned fields missing", ErrInvalidQuarantineState)
		}
		if ids[item.QuarantineID] {
			return fmt.Errorf("%w: duplicate quarantine_id", ErrInvalidQuarantineState)
		}
		ids[item.QuarantineID] = true
		identity := Identity(item.StationID, item.SourcePath, item.SourceSizeBytes)
		if identities[identity] {
			return fmt.Errorf("%w: duplicate source identity", ErrInvalidQuarantineState)
		}
		identities[identity] = true
	}
	return nil
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
