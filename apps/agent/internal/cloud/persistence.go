package cloud

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
	Version  int       `json:"version"`
	SavedAt  time.Time `json:"saved_at"`
	Settings Settings  `json:"settings"`
}
type Persistence struct{ path string }

func NewPersistence(localDataDir string) Persistence {
	return Persistence{path: filepath.Join(localDataDir, "state", "cloud_config.json")}
}
func (p Persistence) Path() string { return p.path }
func (p Persistence) LoadOrDefault() (Settings, error) {
	if _, err := os.Stat(p.path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultSettings(), nil
		}
		return Settings{}, err
	}
	data, err := os.ReadFile(p.path)
	if err != nil {
		return Settings{}, err
	}
	var f stateFile
	if err := json.Unmarshal(data, &f); err != nil {
		return Settings{}, err
	}
	if f.Version != stateVersion {
		return Settings{}, errors.New("invalid cloud config version")
	}
	if f.Settings.Provider == "" || f.Settings.Provider == ProviderGCS {
		f.Settings.Provider = ProviderGoogleDrive
	}
	if f.Settings.FolderNamingTemplate == "" {
		f.Settings.FolderNamingTemplate = FolderNamingTemplate
	}
	f.Settings.ObjectNamingTemplate = ObjectNamingTemplate
	if f.Settings.ConnectionStatus == "" {
		f.Settings.ConnectionStatus = StatusNotConfigured
	}
	return f.Settings, nil
}
func (p Persistence) Save(s Settings) error {
	s.ObjectNamingTemplate = ObjectNamingTemplate
	if s.ConnectionStatus == "" {
		s.ConnectionStatus = StatusNotConfigured
	}
	if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(stateFile{Version: stateVersion, SavedAt: time.Now().UTC(), Settings: s}, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p.path), ".cloud-*.tmp")
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
