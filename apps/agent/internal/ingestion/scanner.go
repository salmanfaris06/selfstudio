package ingestion

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"selfstudio/agent/internal/stations"
)

type PhotoStatus string

const PhotoDetected PhotoStatus = "detected"

type DetectedPhoto struct {
	StationID  string      `json:"station_id"`
	SourcePath string      `json:"source_path"`
	SizeBytes  int64       `json:"size_bytes"`
	DetectedAt time.Time   `json:"detected_at"`
	StableAt   time.Time   `json:"stable_at"`
	Status     PhotoStatus `json:"status"`
}

type StationScanError struct {
	StationID string `json:"station_id"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

type ScanResult struct {
	Photos []DetectedPhoto    `json:"photos"`
	Errors []StationScanError `json:"errors"`
}

type Scanner struct {
	store     *stations.Store
	seen      map[string]struct{}
	mu        sync.Mutex
	stability time.Duration
}

func NewScanner(store *stations.Store) *Scanner {
	return &Scanner{store: store, seen: map[string]struct{}{}, stability: 500 * time.Millisecond}
}

func (s *Scanner) Scan(ctx context.Context) (ScanResult, error) {
	if s.store == nil {
		return ScanResult{}, stations.ErrStationNotFound
	}
	result := ScanResult{Photos: []DetectedPhoto{}, Errors: []StationScanError{}}
	for _, station := range s.store.List() {
		photos, err := s.scanStation(ctx, station)
		if err != nil {
			result.Errors = append(result.Errors, StationScanError{StationID: station.StationID, Code: "INPUT_FOLDER_UNAVAILABLE", Message: "Input folder tidak bisa discan."})
			continue
		}
		result.Photos = append(result.Photos, photos...)
	}
	return result, nil
}

func (s *Scanner) scanStation(ctx context.Context, station stations.Station) ([]DetectedPhoto, error) {
	entries, err := os.ReadDir(station.InputFolder)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	photos := []DetectedPhoto{}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return photos, err
		}
		if entry.IsDir() || !isJPG(entry.Name()) {
			continue
		}
		path := filepath.Clean(filepath.Join(station.InputFolder, entry.Name()))
		identity := strings.ToLower(path)
		s.mu.Lock()
		_, exists := s.seen[identity]
		if !exists {
			s.seen[identity] = struct{}{}
		}
		s.mu.Unlock()
		if exists {
			continue
		}
		first, err := os.Stat(path)
		if err != nil || first.IsDir() || first.Size() <= 0 {
			s.forget(identity)
			continue
		}
		detectedAt := time.Now().UTC()
		if !sleep(ctx, s.stability) {
			return photos, ctx.Err()
		}
		second, err := os.Stat(path)
		if err != nil || second.IsDir() || second.Size() <= 0 {
			s.forget(identity)
			continue
		}
		if first.Size() != second.Size() || !first.ModTime().Equal(second.ModTime()) {
			s.forget(identity)
			continue
		}
		photo := DetectedPhoto{StationID: station.StationID, SourcePath: path, SizeBytes: second.Size(), DetectedAt: detectedAt, StableAt: time.Now().UTC(), Status: PhotoDetected}
		photos = append(photos, photo)
	}
	return photos, nil
}

func (s *Scanner) forget(identity string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.seen, identity)
}

func isJPG(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".jpg" || ext == ".jpeg"
}
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
