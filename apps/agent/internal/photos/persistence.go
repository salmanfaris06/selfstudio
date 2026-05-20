package photos

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

var ErrInvalidPhotoState = errors.New("invalid photo state")

type StateFile struct {
	Version int       `json:"version"`
	SavedAt time.Time `json:"saved_at"`
	Photos  []Photo   `json:"photos"`
}

type Persistence struct{ path string }

func NewPersistence(localDataDir string) Persistence {
	return Persistence{path: filepath.Join(localDataDir, "state", "photos.json")}
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

func (p Persistence) write(items []Photo) error {
	if err := validatePhotos(items); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
		return err
	}
	payload := StateFile{Version: stateVersion, SavedAt: time.Now().UTC(), Photos: items}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p.path), ".photos-*.tmp")
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

func (p Persistence) read() ([]Photo, error) {
	data, err := os.ReadFile(p.path)
	if err != nil {
		return nil, err
	}
	var file StateFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, ErrInvalidPhotoState
	}
	if file.Version != stateVersion {
		return nil, ErrInvalidPhotoState
	}
	if err := validatePhotos(file.Photos); err != nil {
		return nil, err
	}
	return file.Photos, nil
}

func validOriginalStatus(status string) bool {
	return status == OriginalStatusPending || status == OriginalStatusSaving || status == OriginalStatusSaved || status == OriginalStatusFailed
}

func validProcessingStatus(status string) bool {
	return status == ProcessingStatusNotEligible || status == ProcessingStatusEligible
}

func validGradedStatus(status string) bool {
	return status == GradedStatusNotEligible || status == GradedStatusPending || status == GradedStatusProcessing || status == GradedStatusProcessed || status == GradedStatusFailed
}

func validatePhotos(items []Photo) error {
	ids := map[string]bool{}
	identities := map[string]bool{}
	for _, item := range items {
		item = normalizeOriginalState(item)
		if item.PhotoID == "" || item.StationID == "" || item.SessionID == "" || item.SourcePath == "" || item.SourceSizeBytes <= 0 || item.Status != StatusRouted {
			return fmt.Errorf("%w: required routed photo fields missing", ErrInvalidPhotoState)
		}
		if !validOriginalStatus(item.OriginalSaveStatus) || !validProcessingStatus(item.ProcessingStatus) || !validGradedStatus(item.GradedProcessingStatus) {
			return fmt.Errorf("%w: invalid photo processing status", ErrInvalidPhotoState)
		}
		if item.OriginalSaveStatus == OriginalStatusSaved && item.LocalOriginalPath == "" {
			return fmt.Errorf("%w: saved original path missing", ErrInvalidPhotoState)
		}
		if item.GradedProcessingStatus == GradedStatusProcessed && item.LocalGradedPath == "" {
			return fmt.Errorf("%w: processed graded path missing", ErrInvalidPhotoState)
		}
		if ids[item.PhotoID] {
			return fmt.Errorf("%w: duplicate photo_id", ErrInvalidPhotoState)
		}
		ids[item.PhotoID] = true
		identity := Identity(item.StationID, item.SourcePath, item.SourceSizeBytes)
		if identities[identity] {
			return fmt.Errorf("%w: duplicate source identity", ErrInvalidPhotoState)
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
