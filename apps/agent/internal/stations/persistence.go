package stations

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const configVersion = 1

var ErrInvalidBackup = errors.New("invalid station backup")

type ConfigFile struct {
	Version  int       `json:"version"`
	SavedAt  time.Time `json:"saved_at"`
	Stations []Station `json:"stations"`
}

type Persistence struct {
	configPath string
	backupDir  string
}

type BackupMetadata struct {
	Filename     string    `json:"filename"`
	CreatedAt    time.Time `json:"created_at"`
	StationCount int       `json:"station_count"`
}

func NewPersistence(localDataDir string) Persistence {
	configDir := filepath.Join(localDataDir, "config")
	return Persistence{configPath: filepath.Join(configDir, "stations.json"), backupDir: filepath.Join(configDir, "backups")}
}

func (p Persistence) LoadOrDefault() (*Store, error) {
	if _, err := os.Stat(p.configPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			store := NewStore()
			return store, nil
		}
		return nil, err
	}
	stations, err := p.readStationsFile(p.configPath)
	if err != nil {
		return nil, err
	}
	store := NewStore()
	if err := store.ReplaceAll(stations); err != nil {
		return nil, err
	}
	return store, nil
}

func (p Persistence) Save(store *Store) error {
	return p.writeStationsFile(p.configPath, store.List())
}

func (p Persistence) Backup(store *Store) (BackupMetadata, error) {
	createdAt := time.Now().UTC()
	filename := "stations-" + createdAt.Format("20060102-150405.000000000") + ".json"
	path := filepath.Join(p.backupDir, filename)
	if err := p.writeStationsFile(path, store.List()); err != nil {
		return BackupMetadata{}, err
	}
	return BackupMetadata{Filename: filename, CreatedAt: createdAt, StationCount: len(store.List())}, nil
}

func (p Persistence) Restore(store *Store, filename string) (int, error) {
	path, err := p.safeBackupPath(filename)
	if err != nil {
		return 0, err
	}
	candidate, err := p.readStationsFile(path)
	if err != nil {
		return 0, err
	}
	if err := validateStationSet(candidate); err != nil {
		return 0, err
	}
	previous := store.List()
	if err := store.ReplaceAll(candidate); err != nil {
		return 0, err
	}
	if err := p.Save(store); err != nil {
		_ = store.ReplaceAll(previous)
		return 0, err
	}
	return len(candidate), nil
}

func (p Persistence) writeStationsFile(path string, stations []Station) error {
	if err := validateStationSet(stations); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload := ConfigFile{Version: configVersion, SavedAt: time.Now().UTC(), Stations: stations}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".stations-*.tmp")
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
	if err := replaceFile(tmpName, path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
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

func (p Persistence) readStationsFile(path string) ([]Station, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file ConfigFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, ErrInvalidBackup
	}
	if file.Version != configVersion {
		return nil, ErrInvalidBackup
	}
	if err := validateStationSet(file.Stations); err != nil {
		return nil, err
	}
	return file.Stations, nil
}

func (p Persistence) safeBackupPath(filename string) (string, error) {
	name := strings.TrimSpace(filename)
	if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\:`) || !strings.HasSuffix(name, ".json") {
		return "", ErrInvalidBackup
	}
	backupAbs, err := filepath.Abs(filepath.Clean(p.backupDir))
	if err != nil {
		return "", err
	}
	candidate, err := filepath.Abs(filepath.Join(backupAbs, name))
	if err != nil {
		return "", err
	}
	if filepath.Dir(candidate) != backupAbs {
		return "", ErrInvalidBackup
	}
	return candidate, nil
}

func validateStationSet(items []Station) error {
	if len(items) != 3 {
		return ErrInvalidBackup
	}
	seen := map[string]Station{}
	for _, station := range items {
		station.StationID = strings.TrimSpace(station.StationID)
		if station.StationID != Station1ID && station.StationID != Station2ID && station.StationID != Station3ID {
			return ErrInvalidBackup
		}
		if _, ok := seen[station.StationID]; ok {
			return ErrInvalidBackup
		}
		if err := validateStation(station); err != nil {
			return err
		}
		seen[station.StationID] = station
	}
	for _, id := range []string{Station1ID, Station2ID, Station3ID} {
		if _, ok := seen[id]; !ok {
			return ErrInvalidBackup
		}
	}
	tmp := NewStore()
	for _, id := range []string{Station1ID, Station2ID, Station3ID} {
		station := seen[id]
		if _, err := tmp.Update(id, UpdateStation{Name: station.Name, DeviceIdentifier: station.DeviceIdentifier, InputFolder: station.InputFolder, BackgroundName: station.BackgroundName, DefaultLUTPath: station.DefaultLUTPath, OutputRule: station.OutputRule, CameraAssignment: station.CameraAssignment}); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidBackup, err)
		}
	}
	return nil
}
