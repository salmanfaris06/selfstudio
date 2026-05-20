package stations

import (
	"errors"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const maxFieldLength = 512

const (
	Station1ID = "station_1"
	Station2ID = "station_2"
	Station3ID = "station_3"
)

var (
	ErrStationNotFound           = errors.New("station not found")
	ErrInvalidStationConfig      = errors.New("invalid station config")
	ErrDuplicateInputFolder      = errors.New("duplicate input folder")
	ErrDuplicateCameraAssignment = errors.New("duplicate camera assignment")
	ErrInputFolderUnavailable    = errors.New("input folder unavailable")
	ErrValidationCancelled       = errors.New("validation cancelled")
)

type CameraAssignment struct {
	IdentityKey string     `json:"identity_key"`
	CameraName  string     `json:"camera_name"`
	Port        string     `json:"port"`
	Runtime     string     `json:"runtime"`
	AssignedAt  time.Time  `json:"assigned_at"`
	LastSeenAt  *time.Time `json:"last_seen_at,omitempty"`
	Connected   bool       `json:"connected"`
}

type Station struct {
	StationID        string            `json:"station_id"`
	Name             string            `json:"name"`
	DeviceIdentifier string            `json:"device_identifier"`
	InputFolder      string            `json:"input_folder"`
	BackgroundName   string            `json:"background_name"`
	DefaultLUTPath   string            `json:"default_lut_path"`
	OutputRule       string            `json:"output_rule"`
	CameraAssignment *CameraAssignment `json:"camera_assignment,omitempty"`
}

type UpdateStation struct {
	Name             string            `json:"name"`
	DeviceIdentifier string            `json:"device_identifier"`
	InputFolder      string            `json:"input_folder"`
	BackgroundName   string            `json:"background_name"`
	DefaultLUTPath   string            `json:"default_lut_path"`
	OutputRule       string            `json:"output_rule"`
	CameraAssignment *CameraAssignment `json:"camera_assignment,omitempty"`
}

type UpdateCameraAssignment struct {
	IdentityKey string `json:"identity_key"`
	CameraName  string `json:"camera_name"`
	Port        string `json:"port"`
	Runtime     string `json:"runtime"`
	Connected   bool   `json:"connected"`
}

type Store struct {
	stations map[string]Station
	order    []string
	mu       sync.RWMutex
}

func NewStore() *Store {
	order := []string{Station1ID, Station2ID, Station3ID}
	stations := map[string]Station{
		Station1ID: defaultStation(Station1ID, "Station 1", "station-1"),
		Station2ID: defaultStation(Station2ID, "Station 2", "station-2"),
		Station3ID: defaultStation(Station3ID, "Station 3", "station-3"),
	}
	return &Store{stations: stations, order: order}
}

func defaultStation(id string, name string, folderName string) Station {
	return Station{
		StationID:        id,
		Name:             name,
		DeviceIdentifier: name,
		InputFolder:      filepath.Join("local-data", "input", folderName),
		BackgroundName:   "Default Background",
		DefaultLUTPath:   filepath.Join("local-data", "luts", "default.cube"),
		OutputRule:       "{customer_name}/{order_number}/{station_id}",
	}
}

func (s *Store) List() []Station {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stations := make([]Station, 0, len(s.order))
	for _, id := range s.order {
		stations = append(stations, s.stations[id])
	}
	return stations
}

func (s *Store) Get(stationID string) (Station, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	station, ok := s.stations[stationID]
	if !ok {
		return Station{}, ErrStationNotFound
	}
	return station, nil
}

func (s *Store) ReplaceAll(items []Station) error {
	if err := validateStationSet(items); err != nil {
		return err
	}
	next := map[string]Station{}
	for _, station := range items {
		next[station.StationID] = Station{
			StationID:        station.StationID,
			Name:             strings.TrimSpace(station.Name),
			DeviceIdentifier: strings.TrimSpace(station.DeviceIdentifier),
			InputFolder:      strings.TrimSpace(station.InputFolder),
			BackgroundName:   strings.TrimSpace(station.BackgroundName),
			DefaultLUTPath:   strings.TrimSpace(station.DefaultLUTPath),
			OutputRule:       strings.TrimSpace(station.OutputRule),
			CameraAssignment: normalizeAssignment(station.CameraAssignment),
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stations = next
	return nil
}

func (s *Store) Update(stationID string, update UpdateStation) (Station, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.stations[stationID]
	if !ok {
		return Station{}, ErrStationNotFound
	}

	updated := Station{
		StationID:        current.StationID,
		Name:             strings.TrimSpace(update.Name),
		DeviceIdentifier: strings.TrimSpace(update.DeviceIdentifier),
		InputFolder:      strings.TrimSpace(update.InputFolder),
		BackgroundName:   strings.TrimSpace(update.BackgroundName),
		DefaultLUTPath:   strings.TrimSpace(update.DefaultLUTPath),
		OutputRule:       strings.TrimSpace(update.OutputRule),
		CameraAssignment: normalizeAssignment(update.CameraAssignment),
	}

	if err := validateStation(updated); err != nil {
		return Station{}, err
	}
	if s.hasDuplicateInputFolderLocked(stationID, updated.InputFolder) {
		return Station{}, ErrDuplicateInputFolder
	}
	if s.hasDuplicateCameraAssignmentLocked(stationID, updated.CameraAssignment) {
		return Station{}, ErrDuplicateCameraAssignment
	}

	s.stations[stationID] = updated
	return updated, nil
}

func (s *Store) UpdateCameraAssignment(stationID string, update UpdateCameraAssignment) (Station, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.stations[stationID]
	if !ok {
		return Station{}, ErrStationNotFound
	}
	assignment := normalizeAssignment(&CameraAssignment{IdentityKey: update.IdentityKey, CameraName: update.CameraName, Port: update.Port, Runtime: update.Runtime, AssignedAt: time.Now().UTC(), Connected: update.Connected})
	updated := current
	updated.CameraAssignment = assignment
	if err := validateStation(updated); err != nil {
		return Station{}, err
	}
	if s.hasDuplicateCameraAssignmentLocked(stationID, assignment) {
		return Station{}, ErrDuplicateCameraAssignment
	}
	s.stations[stationID] = updated
	return updated, nil
}

func validateStation(station Station) error {
	missing := []string{}
	if station.Name == "" {
		missing = append(missing, "name")
	}
	if station.DeviceIdentifier == "" {
		missing = append(missing, "device_identifier")
	}
	if station.InputFolder == "" {
		missing = append(missing, "input_folder")
	}
	if station.BackgroundName == "" {
		missing = append(missing, "background_name")
	}
	if station.DefaultLUTPath == "" {
		missing = append(missing, "default_lut_path")
	}
	if station.OutputRule == "" {
		missing = append(missing, "output_rule")
	}
	if len(station.Name) > maxFieldLength {
		missing = append(missing, "name")
	}
	if len(station.DeviceIdentifier) > maxFieldLength {
		missing = append(missing, "device_identifier")
	}
	if len(station.InputFolder) > maxFieldLength {
		missing = append(missing, "input_folder")
	}
	if len(station.BackgroundName) > maxFieldLength {
		missing = append(missing, "background_name")
	}
	if len(station.DefaultLUTPath) > maxFieldLength {
		missing = append(missing, "default_lut_path")
	}
	if len(station.OutputRule) > maxFieldLength || !validOutputRule(station.OutputRule) {
		missing = append(missing, "output_rule")
	}
	if station.CameraAssignment != nil {
		if len(station.CameraAssignment.IdentityKey) > maxFieldLength || len(station.CameraAssignment.CameraName) > maxFieldLength || len(station.CameraAssignment.Port) > maxFieldLength || len(station.CameraAssignment.Runtime) > maxFieldLength {
			missing = append(missing, "camera_assignment")
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return FieldError{Fields: missing}
	}
	return nil
}

func validOutputRule(rule string) bool {
	trimmed := strings.TrimSpace(rule)
	if !strings.Contains(trimmed, "{station_id}") || looksLikeWindowsAbsolutePath(trimmed) {
		return false
	}
	cleaned := filepath.ToSlash(filepath.Clean(strings.ReplaceAll(trimmed, "\\", "/")))
	if strings.HasPrefix(cleaned, "../") || cleaned == ".." || strings.Contains(cleaned, "/../") {
		return false
	}
	return !filepath.IsAbs(trimmed)
}

func looksLikeWindowsAbsolutePath(path string) bool {
	if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//") {
		return true
	}
	if len(path) >= 3 && ((path[0] >= 'A' && path[0] <= 'Z') || (path[0] >= 'a' && path[0] <= 'z')) && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		return true
	}
	return false
}

func (s *Store) hasDuplicateCameraAssignmentLocked(stationID string, assignment *CameraAssignment) bool {
	candidate := normalizeCameraIdentity("")
	if assignment != nil {
		candidate = normalizeCameraIdentity(assignment.IdentityKey)
	}
	if candidate == "" {
		return false
	}
	for otherID, station := range s.stations {
		if otherID == stationID || station.CameraAssignment == nil {
			continue
		}
		if normalizeCameraIdentity(station.CameraAssignment.IdentityKey) == candidate {
			return true
		}
	}
	return false
}

func normalizeAssignment(assignment *CameraAssignment) *CameraAssignment {
	if assignment == nil || strings.TrimSpace(assignment.IdentityKey) == "" {
		return nil
	}
	copy := *assignment
	copy.IdentityKey = strings.TrimSpace(copy.IdentityKey)
	copy.CameraName = strings.TrimSpace(copy.CameraName)
	copy.Port = strings.TrimSpace(copy.Port)
	copy.Runtime = strings.TrimSpace(copy.Runtime)
	if copy.AssignedAt.IsZero() {
		copy.AssignedAt = time.Now().UTC()
	}
	return &copy
}

func normalizeCameraIdentity(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.ReplaceAll(trimmed, "\\", "/")
	trimmed = strings.ReplaceAll(trimmed, " | ", "|")
	return strings.ToLower(trimmed)
}

func (s *Store) hasDuplicateInputFolderLocked(stationID string, inputFolder string) bool {
	candidate := normalizeFolder(inputFolder)
	for otherID, station := range s.stations {
		if otherID == stationID {
			continue
		}
		if normalizeFolder(station.InputFolder) == candidate {
			return true
		}
	}
	return false
}

func normalizeFolder(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.ReplaceAll(trimmed, "\\", string(filepath.Separator))
	trimmed = strings.ReplaceAll(trimmed, "/", string(filepath.Separator))
	cleaned := filepath.Clean(trimmed)
	if abs, err := filepath.Abs(cleaned); err == nil {
		cleaned = abs
	}
	if runtime.GOOS == "windows" {
		return strings.ToLower(cleaned)
	}
	return cleaned
}

type FieldError struct {
	Fields []string
}

func (e FieldError) Error() string {
	return ErrInvalidStationConfig.Error()
}
