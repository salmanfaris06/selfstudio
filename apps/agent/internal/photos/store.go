package photos

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const StatusRouted = "routed"

const (
	OriginalStatusPending       = "pending"
	OriginalStatusSaving        = "saving"
	OriginalStatusSaved         = "saved_original"
	OriginalStatusFailed        = "failed"
	ProcessingStatusNotEligible = "not_eligible"
	ProcessingStatusEligible    = "eligible"
	GradedStatusNotEligible     = "not_eligible"
	GradedStatusPending         = "pending"
	GradedStatusProcessing      = "processing"
	GradedStatusProcessed       = "processed"
	GradedStatusFailed          = "failed"
)

type Photo struct {
	PhotoID                   string     `json:"photo_id"`
	StationID                 string     `json:"station_id"`
	SessionID                 string     `json:"session_id"`
	SourcePath                string     `json:"source_path"`
	SourceSizeBytes           int64      `json:"source_size_bytes"`
	DetectedAt                time.Time  `json:"detected_at"`
	StableAt                  time.Time  `json:"stable_at"`
	RoutedAt                  time.Time  `json:"routed_at"`
	Status                    string     `json:"status"`
	LocalOriginalPath         string     `json:"local_original_path,omitempty"`
	OriginalSaveStatus        string     `json:"original_save_status"`
	LastError                 string     `json:"last_error,omitempty"`
	AttemptCount              int        `json:"attempt_count"`
	OriginalSaveStartedAt     *time.Time `json:"original_save_started_at,omitempty"`
	OriginalSavedAt           *time.Time `json:"original_saved_at,omitempty"`
	ProcessingStatus          string     `json:"processing_status"`
	LocalGradedPath           string     `json:"local_graded_path,omitempty"`
	GradedProcessingStatus    string     `json:"graded_processing_status"`
	GradedLastError           string     `json:"graded_last_error,omitempty"`
	GradedAttemptCount        int        `json:"graded_attempt_count"`
	GradedProcessingStartedAt *time.Time `json:"graded_processing_started_at,omitempty"`
	GradedProcessedAt         *time.Time `json:"graded_processed_at,omitempty"`
	LUTSnapshotPath           string     `json:"lut_snapshot_path,omitempty"`
	Duplicate                 bool       `json:"duplicate"`
}

type Store struct {
	photos     map[string]Photo
	identityTo stringMap
	order      []string
	mu         sync.RWMutex
}

type stringMap map[string]string

func NewStore() *Store {
	return &Store{photos: map[string]Photo{}, identityTo: stringMap{}, order: []string{}}
}

func NewStoreFromRecords(records []Photo) (*Store, error) {
	store := NewStore()
	if err := store.ReplaceAll(records); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) ReplaceAll(records []Photo) error {
	if err := validatePhotos(records); err != nil {
		return err
	}
	photosByID := map[string]Photo{}
	identityTo := stringMap{}
	order := []string{}
	for _, record := range records {
		record.SourcePath = filepath.Clean(record.SourcePath)
		record.DetectedAt = record.DetectedAt.UTC()
		record.StableAt = record.StableAt.UTC()
		record.RoutedAt = record.RoutedAt.UTC()
		record.Duplicate = false
		record = normalizeOriginalState(record)
		photosByID[record.PhotoID] = record
		identityTo[Identity(record.StationID, record.SourcePath, record.SourceSizeBytes)] = record.PhotoID
		order = append(order, record.PhotoID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.photos = photosByID
	s.identityTo = identityTo
	s.order = order
	return nil
}

func (s *Store) Route(stationID string, sessionID string, sourcePath string, sourceSizeBytes int64, detectedAt time.Time, stableAt time.Time, routedAt time.Time) Photo {
	identity := Identity(stationID, sourcePath, sourceSizeBytes)
	s.mu.Lock()
	defer s.mu.Unlock()
	if existingID, ok := s.identityTo[identity]; ok {
		existing := s.photos[existingID]
		existing.Duplicate = true
		return existing
	}
	photo := normalizeOriginalState(Photo{PhotoID: newPhotoID(identity), StationID: stationID, SessionID: sessionID, SourcePath: filepath.Clean(sourcePath), SourceSizeBytes: sourceSizeBytes, DetectedAt: detectedAt.UTC(), StableAt: stableAt.UTC(), RoutedAt: routedAt.UTC(), Status: StatusRouted, Duplicate: false})
	s.photos[photo.PhotoID] = photo
	s.identityTo[identity] = photo.PhotoID
	s.order = append(s.order, photo.PhotoID)
	return photo
}

func (s *Store) ListBySession(sessionID string, limit int) []Photo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Photo{}
	for i := len(s.order) - 1; i >= 0; i-- {
		photo := s.photos[s.order[i]]
		if photo.SessionID != sessionID {
			continue
		}
		photo.Duplicate = false
		out = append(out, photo)
		if limit > 0 && len(out) == limit {
			break
		}
	}
	return out
}

func (s *Store) CountBySession(sessionID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, photo := range s.photos {
		if photo.SessionID == sessionID {
			count++
		}
	}
	return count
}

func (s *Store) Get(photoID string) (Photo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	photo, ok := s.photos[photoID]
	if !ok {
		return Photo{}, false
	}
	photo.Duplicate = false
	return photo, true
}

func (s *Store) GetBySourceIdentity(stationID string, sourcePath string, sourceSizeBytes int64) (Photo, bool) {
	identity := Identity(stationID, sourcePath, sourceSizeBytes)
	s.mu.RLock()
	defer s.mu.RUnlock()
	photoID, ok := s.identityTo[identity]
	if !ok {
		return Photo{}, false
	}
	photo := s.photos[photoID]
	photo.Duplicate = false
	return photo, true
}

func Identity(stationID string, sourcePath string, sourceSizeBytes int64) string {
	normalizedPath := strings.ToLower(filepath.Clean(sourcePath))
	return stationID + "|" + normalizedPath + "|" + strings.TrimSpace(strings.ToLower(formatInt(sourceSizeBytes)))
}

func newPhotoID(identity string) string {
	sum := sha256.Sum256([]byte(identity))
	return "photo_" + hex.EncodeToString(sum[:12])
}

func formatInt(value int64) string {
	return strconv.FormatInt(value, 10)
}

func (s *Store) MarkOriginalSaving(photoID string, path string, now time.Time) Photo {
	s.mu.Lock()
	defer s.mu.Unlock()
	photo := s.photos[photoID]
	now = now.UTC()
	photo.LocalOriginalPath = filepath.Clean(path)
	photo.OriginalSaveStatus = OriginalStatusSaving
	photo.LastError = ""
	photo.AttemptCount++
	photo.OriginalSaveStartedAt = &now
	photo.ProcessingStatus = ProcessingStatusNotEligible
	s.photos[photoID] = photo
	return photo
}

func (s *Store) MarkOriginalSaved(photoID string, path string, now time.Time) Photo {
	s.mu.Lock()
	defer s.mu.Unlock()
	photo := s.photos[photoID]
	now = now.UTC()
	photo.LocalOriginalPath = filepath.Clean(path)
	photo.OriginalSaveStatus = OriginalStatusSaved
	photo.LastError = ""
	photo.OriginalSavedAt = &now
	photo.ProcessingStatus = ProcessingStatusEligible
	if photo.GradedProcessingStatus == "" || photo.GradedProcessingStatus == GradedStatusNotEligible {
		photo.GradedProcessingStatus = GradedStatusPending
	}
	s.photos[photoID] = photo
	return photo
}

func (s *Store) MarkOriginalFailed(photoID string, message string, now time.Time) Photo {
	s.mu.Lock()
	defer s.mu.Unlock()
	photo := s.photos[photoID]
	photo.OriginalSaveStatus = OriginalStatusFailed
	photo.LastError = strings.TrimSpace(message)
	photo.ProcessingStatus = ProcessingStatusNotEligible
	s.photos[photoID] = photo
	return photo
}

func normalizeOriginalState(photo Photo) Photo {
	if photo.OriginalSaveStatus == "" {
		photo.OriginalSaveStatus = OriginalStatusPending
	}
	if photo.ProcessingStatus == "" {
		photo.ProcessingStatus = ProcessingStatusNotEligible
	}
	if photo.GradedProcessingStatus == "" {
		if photo.OriginalSaveStatus == OriginalStatusSaved {
			photo.GradedProcessingStatus = GradedStatusPending
		} else {
			photo.GradedProcessingStatus = GradedStatusNotEligible
		}
	}
	return photo
}

func (s *Store) MarkGradedProcessing(photoID string, path string, lutPath string, now time.Time) Photo {
	s.mu.Lock()
	defer s.mu.Unlock()
	photo := s.photos[photoID]
	now = now.UTC()
	photo.LocalGradedPath = filepath.Clean(path)
	photo.LUTSnapshotPath = filepath.Clean(lutPath)
	photo.GradedProcessingStatus = GradedStatusProcessing
	photo.GradedLastError = ""
	photo.GradedAttemptCount++
	photo.GradedProcessingStartedAt = &now
	s.photos[photoID] = photo
	return photo
}

func (s *Store) MarkGradedProcessed(photoID string, path string, lutPath string, now time.Time) Photo {
	s.mu.Lock()
	defer s.mu.Unlock()
	photo := s.photos[photoID]
	now = now.UTC()
	photo.LocalGradedPath = filepath.Clean(path)
	photo.LUTSnapshotPath = filepath.Clean(lutPath)
	photo.GradedProcessingStatus = GradedStatusProcessed
	photo.GradedLastError = ""
	photo.GradedProcessedAt = &now
	s.photos[photoID] = photo
	return photo
}

func (s *Store) MarkGradedFailed(photoID string, message string, now time.Time) Photo {
	s.mu.Lock()
	defer s.mu.Unlock()
	photo := s.photos[photoID]
	photo.GradedProcessingStatus = GradedStatusFailed
	photo.GradedLastError = strings.TrimSpace(message)
	s.photos[photoID] = photo
	return photo
}

func (s *Store) ListAll() []Photo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Photo, 0, len(s.order))
	ids := append([]string{}, s.order...)
	sort.Strings(ids)
	for _, id := range ids {
		photo := s.photos[id]
		photo.Duplicate = false
		out = append(out, photo)
	}
	return out
}
