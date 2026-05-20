package activity

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

const DefaultMaxEntries = 200

var fallbackCounter uint64

type Result string

const (
	ResultSuccess Result = "success"
	ResultFailure Result = "failure"
)

type Entry struct {
	ID         string    `json:"id"`
	OccurredAt time.Time `json:"occurred_at"`
	ActionType string    `json:"action_type"`
	Result     Result    `json:"result"`
	Message    string    `json:"message"`
	StationID  *string   `json:"station_id"`
	SessionID  *string   `json:"session_id"`
}

type Store struct {
	entries    []Entry
	maxEntries int
	mu         sync.RWMutex
}

func NewStore(maxEntries int) *Store {
	if maxEntries <= 0 {
		maxEntries = DefaultMaxEntries
	}
	return &Store{entries: make([]Entry, 0, maxEntries), maxEntries: maxEntries}
}

func (s *Store) Record(actionType string, result Result, message string) Entry {
	return s.RecordWithRefs(actionType, result, message, nil, nil)
}

func (s *Store) RecordWithRefs(actionType string, result Result, message string, stationID *string, sessionID *string) Entry {
	entry := Entry{
		ID:         newEntryID(),
		OccurredAt: time.Now().UTC(),
		ActionType: actionType,
		Result:     result,
		Message:    message,
		StationID:  stationID,
		SessionID:  sessionID,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries = append([]Entry{entry}, s.entries...)
	if len(s.entries) > s.maxEntries {
		s.entries = s.entries[:s.maxEntries]
	}

	return entry
}

func (s *Store) Recent(limit int, actionType string) []Entry {
	if limit <= 0 {
		limit = 50
	}
	if limit > s.maxEntries {
		limit = s.maxEntries
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := make([]Entry, 0, min(limit, len(s.entries)))
	for _, entry := range s.entries {
		if actionType != "" && entry.ActionType != actionType {
			continue
		}
		entries = append(entries, entry)
		if len(entries) == limit {
			break
		}
	}

	return slices.Clone(entries)
}

func newEntryID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("act_fallback_%d_%d", time.Now().UnixNano(), atomic.AddUint64(&fallbackCounter, 1))
	}
	return "act_" + hex.EncodeToString(bytes)
}
