package quarantine

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	StatusQuarantined     = "quarantined"
	StatusAssigned        = "assigned"
	ReasonNoActiveSession = "no_active_session"
	ReasonLatePhoto       = "late_photo"
)

var (
	ErrNotFound                        = errors.New("quarantine not found")
	ErrNotAssignable                   = errors.New("quarantine is not assignable")
	ErrAlreadyAssignedDifferentSession = errors.New("quarantine already assigned to different session")
)

type ListFilter struct {
	Status    string
	StationID string
	Limit     int
}

type Record struct {
	QuarantineID      string    `json:"quarantine_id"`
	PhotoID           string    `json:"photo_id,omitempty"`
	StationID         string    `json:"station_id"`
	RelatedSessionID  string    `json:"related_session_id,omitempty"`
	SourcePath        string    `json:"source_path"`
	SourceSizeBytes   int64     `json:"source_size_bytes"`
	DetectedAt        time.Time `json:"detected_at"`
	StableAt          time.Time `json:"stable_at"`
	QuarantinedAt     time.Time `json:"quarantined_at"`
	Reason            string    `json:"reason"`
	Status            string    `json:"status"`
	AssignedSessionID string    `json:"assigned_session_id,omitempty"`
	AssignedPhotoID   string    `json:"assigned_photo_id,omitempty"`
	AssignedAt        time.Time `json:"assigned_at,omitempty"`
	Duplicate         bool      `json:"duplicate"`
}

type Store struct {
	records    map[string]Record
	identityTo map[string]string
	order      []string
	mu         sync.RWMutex
}

func NewStore() *Store {
	return &Store{records: map[string]Record{}, identityTo: map[string]string{}, order: []string{}}
}

func NewStoreFromRecords(records []Record) (*Store, error) {
	store := NewStore()
	if err := store.ReplaceAll(records); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) ReplaceAll(records []Record) error {
	if err := validateRecords(records); err != nil {
		return err
	}
	recordsByID := map[string]Record{}
	identityTo := map[string]string{}
	order := []string{}
	for _, record := range records {
		record.SourcePath = filepath.Clean(record.SourcePath)
		record.DetectedAt = record.DetectedAt.UTC()
		record.StableAt = record.StableAt.UTC()
		record.QuarantinedAt = record.QuarantinedAt.UTC()
		record.AssignedAt = record.AssignedAt.UTC()
		record.Duplicate = false
		recordsByID[record.QuarantineID] = record
		identityTo[Identity(record.StationID, record.SourcePath, record.SourceSizeBytes)] = record.QuarantineID
		order = append(order, record.QuarantineID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = recordsByID
	s.identityTo = identityTo
	s.order = order
	return nil
}

func (s *Store) Quarantine(stationID string, relatedSessionID string, sourcePath string, sourceSizeBytes int64, detectedAt time.Time, stableAt time.Time, quarantinedAt time.Time, reason string) Record {
	identity := Identity(stationID, sourcePath, sourceSizeBytes)
	s.mu.Lock()
	defer s.mu.Unlock()
	if existingID, ok := s.identityTo[identity]; ok {
		existing := s.records[existingID]
		existing.Duplicate = true
		return existing
	}
	if reason != ReasonLatePhoto && reason != ReasonNoActiveSession {
		reason = ReasonNoActiveSession
	}
	record := Record{QuarantineID: newQuarantineID(identity), StationID: stationID, RelatedSessionID: relatedSessionID, SourcePath: filepath.Clean(sourcePath), SourceSizeBytes: sourceSizeBytes, DetectedAt: detectedAt.UTC(), StableAt: stableAt.UTC(), QuarantinedAt: quarantinedAt.UTC(), Reason: reason, Status: StatusQuarantined, Duplicate: false}
	s.records[record.QuarantineID] = record
	s.identityTo[identity] = record.QuarantineID
	s.order = append(s.order, record.QuarantineID)
	return record
}

func (s *Store) CountByStation(stationID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, record := range s.records {
		if record.StationID == stationID && record.Status == StatusQuarantined {
			count++
		}
	}
	return count
}

func (s *Store) CountByRelatedSession(sessionID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, record := range s.records {
		if record.RelatedSessionID == sessionID && record.Status == StatusQuarantined {
			count++
		}
	}
	return count
}

func (s *Store) ListByStation(stationID string, limit int) []Record {
	return s.List(ListFilter{StationID: stationID, Status: StatusQuarantined, Limit: limit})
}

func (s *Store) List(filter ListFilter) []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Record{}
	for i := len(s.order) - 1; i >= 0; i-- {
		record := s.records[s.order[i]]
		if filter.StationID != "" && record.StationID != filter.StationID {
			continue
		}
		if filter.Status != "" && record.Status != filter.Status {
			continue
		}
		record.Duplicate = false
		out = append(out, record)
		if filter.Limit > 0 && len(out) == filter.Limit {
			break
		}
	}
	return out
}

func (s *Store) Get(quarantineID string) (Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.records[quarantineID]
	if !ok {
		return Record{}, ErrNotFound
	}
	record.Duplicate = false
	return record, nil
}

func (s *Store) Assign(quarantineID string, sessionID string, photoID string, assignedAt time.Time) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[quarantineID]
	if !ok {
		return Record{}, ErrNotFound
	}
	if record.Status == StatusAssigned {
		if record.AssignedSessionID == sessionID {
			record.Duplicate = false
			return record, nil
		}
		return Record{}, ErrAlreadyAssignedDifferentSession
	}
	if record.Status != StatusQuarantined {
		return Record{}, ErrNotAssignable
	}
	record.Status = StatusAssigned
	record.AssignedSessionID = sessionID
	record.AssignedPhotoID = photoID
	record.PhotoID = photoID
	record.AssignedAt = assignedAt.UTC()
	record.Duplicate = false
	s.records[quarantineID] = record
	return record, nil
}

func (s *Store) ListByRelatedSession(sessionID string, limit int) []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Record{}
	for i := len(s.order) - 1; i >= 0; i-- {
		record := s.records[s.order[i]]
		if record.RelatedSessionID != sessionID {
			continue
		}
		record.Duplicate = false
		out = append(out, record)
		if limit > 0 && len(out) == limit {
			break
		}
	}
	return out
}

func (s *Store) ListAll() []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := append([]string{}, s.order...)
	sort.Strings(ids)
	out := make([]Record, 0, len(ids))
	for _, id := range ids {
		record := s.records[id]
		record.Duplicate = false
		out = append(out, record)
	}
	return out
}

func Identity(stationID string, sourcePath string, sourceSizeBytes int64) string {
	normalizedPath := strings.ToLower(filepath.Clean(sourcePath))
	return stationID + "|" + normalizedPath + "|" + strings.TrimSpace(strings.ToLower(strconv.FormatInt(sourceSizeBytes, 10)))
}

func newQuarantineID(identity string) string {
	sum := sha256.Sum256([]byte(identity))
	return "quar_" + hex.EncodeToString(sum[:12])
}
