package sessions

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"selfstudio/agent/internal/stations"
)

const (
	StatusActive    = "active"
	StatusLocked    = "locked"
	EndReasonManual = "manual"
	EndReasonTimer  = "timer"
	maxFieldLen     = 256
)

var (
	ErrSessionNotFound      = errors.New("session not found")
	ErrSessionAlreadyActive = errors.New("session already active")
	ErrInvalidSession       = errors.New("invalid session")
)

type StationSnapshot struct {
	StationName      string `json:"station_name"`
	BackgroundName   string `json:"background_name"`
	DefaultLUTPath   string `json:"default_lut_path"`
	InputFolder      string `json:"input_folder"`
	OutputRule       string `json:"output_rule"`
	OutputFolder     string `json:"output_folder"`
	DeviceIdentifier string `json:"device_identifier"`
}

type Session struct {
	SessionID       string          `json:"session_id"`
	StationID       string          `json:"station_id"`
	Status          string          `json:"status"`
	CustomerName    string          `json:"customer_name"`
	OrderNumber     string          `json:"order_number"`
	TimerSeconds    int             `json:"timer_seconds"`
	StartedAt       time.Time       `json:"started_at"`
	EndsAt          time.Time       `json:"ends_at"`
	StationSnapshot StationSnapshot `json:"station_snapshot"`
	EndedAt         *time.Time      `json:"ended_at"`
	EndReason       *string         `json:"end_reason"`
}

type StartSessionRequest struct {
	CustomerName string `json:"customer_name"`
	OrderNumber  string `json:"order_number"`
	TimerSeconds int    `json:"timer_seconds"`
}

type EndSessionRequest struct {
	Reason string `json:"reason"`
}

type Store struct {
	sessions map[string]Session
	order    []string
	mu       sync.RWMutex
}

func NewStore() *Store {
	return &Store{sessions: map[string]Session{}, order: []string{}}
}

func (s *Store) Remove(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	order := s.order[:0]
	for _, id := range s.order {
		if id != sessionID {
			order = append(order, id)
		}
	}
	s.order = order
}

func (s *Store) Get(sessionID string) (Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	return session, nil
}

func (s *Store) List() []Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Session, 0, len(s.order))
	for _, id := range s.order {
		if session, ok := s.sessions[id]; ok {
			out = append(out, session)
		}
	}
	return out
}

func (s *Store) ActiveForStation(stationID string, now time.Time) (Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lockExpiredLocked(now.UTC())
	for _, id := range s.order {
		session, ok := s.sessions[id]
		if ok && session.StationID == stationID && session.Status == StatusActive {
			return session, true
		}
	}
	return Session{}, false
}

func (s *Store) LastSessionForStation(stationID string, now time.Time) (Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lockExpiredLocked(now.UTC())
	for i := len(s.order) - 1; i >= 0; i-- {
		session, ok := s.sessions[s.order[i]]
		if ok && session.StationID == stationID {
			return session, true
		}
	}
	return Session{}, false
}

func (s *Store) ReplaceAll(items []Session) error {
	if err := validateSessions(items); err != nil {
		return err
	}
	next := map[string]Session{}
	order := make([]string, 0, len(items))
	for _, session := range items {
		normalized := normalizeSession(session)
		next[normalized.SessionID] = normalized
		order = append(order, normalized.SessionID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions = next
	s.order = order
	return nil
}

func (s *Store) Start(station stations.Station, request StartSessionRequest, outputFolder string, now time.Time) (Session, error) {
	request.CustomerName = strings.TrimSpace(request.CustomerName)
	request.OrderNumber = strings.TrimSpace(request.OrderNumber)
	if err := validateStartRequest(request); err != nil {
		return Session{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, session := range s.sessions {
		if session.StationID == station.StationID && session.Status == StatusActive {
			return Session{}, ErrSessionAlreadyActive
		}
	}
	session := Session{
		SessionID:    newSessionID(),
		StationID:    station.StationID,
		Status:       StatusActive,
		CustomerName: request.CustomerName,
		OrderNumber:  request.OrderNumber,
		TimerSeconds: request.TimerSeconds,
		StartedAt:    now.UTC(),
		EndsAt:       now.UTC().Add(time.Duration(request.TimerSeconds) * time.Second),
		StationSnapshot: StationSnapshot{
			StationName:      station.Name,
			BackgroundName:   station.BackgroundName,
			DefaultLUTPath:   station.DefaultLUTPath,
			InputFolder:      station.InputFolder,
			OutputRule:       station.OutputRule,
			OutputFolder:     outputFolder,
			DeviceIdentifier: station.DeviceIdentifier,
		},
	}
	s.sessions[session.SessionID] = session
	s.order = append(s.order, session.SessionID)
	return session, nil
}

func (s *Store) End(sessionID string, reason string, now time.Time) (Session, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = EndReasonManual
	}
	if reason != EndReasonManual && reason != EndReasonTimer {
		return Session{}, ErrInvalidSession
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lockExpiredLocked(now.UTC())
	session, ok := s.sessions[sessionID]
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	if session.Status == StatusLocked {
		return session, nil
	}
	endedAt := now.UTC()
	session.Status = StatusLocked
	session.EndedAt = &endedAt
	session.EndReason = &reason
	s.sessions[sessionID] = session
	return session, nil
}

func (s *Store) LockExpired(now time.Time) []Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lockExpiredLocked(now.UTC())
}

func (s *Store) lockExpiredLocked(now time.Time) []Session {
	expired := []Session{}
	for id, session := range s.sessions {
		if session.Status == StatusActive && !session.EndsAt.After(now) {
			endedAt := session.EndsAt.UTC()
			reason := EndReasonTimer
			session.Status = StatusLocked
			session.EndedAt = &endedAt
			session.EndReason = &reason
			s.sessions[id] = session
			expired = append(expired, session)
		}
	}
	return expired
}

func validateStartRequest(request StartSessionRequest) error {
	if request.CustomerName == "" || request.OrderNumber == "" || request.TimerSeconds < 60 || request.TimerSeconds > 24*60*60 {
		return ErrInvalidSession
	}
	if len(request.CustomerName) > maxFieldLen || len(request.OrderNumber) > maxFieldLen {
		return ErrInvalidSession
	}
	return nil
}

func validateSessions(items []Session) error {
	activeByStation := map[string]bool{}
	seen := map[string]bool{}
	for _, session := range items {
		if session.SessionID == "" || seen[session.SessionID] || session.StationID == "" || (session.Status != StatusActive && session.Status != StatusLocked) || strings.TrimSpace(session.CustomerName) == "" || strings.TrimSpace(session.OrderNumber) == "" || session.TimerSeconds < 60 || session.TimerSeconds > 24*60*60 || session.StartedAt.IsZero() || session.EndsAt.IsZero() || !session.EndsAt.After(session.StartedAt) || session.StationSnapshot.OutputFolder == "" {
			return ErrInvalidSession
		}
		if session.Status == StatusLocked && (session.EndedAt == nil || session.EndReason == nil || (*session.EndReason != EndReasonManual && *session.EndReason != EndReasonTimer)) {
			return ErrInvalidSession
		}
		seen[session.SessionID] = true
		if session.Status == StatusActive {
			if activeByStation[session.StationID] {
				return ErrSessionAlreadyActive
			}
			activeByStation[session.StationID] = true
		}
	}
	return nil
}

func normalizeSession(session Session) Session {
	session.CustomerName = strings.TrimSpace(session.CustomerName)
	session.OrderNumber = strings.TrimSpace(session.OrderNumber)
	return session
}

func newSessionID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "sess_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
	}
	return "sess_" + hex.EncodeToString(b[:])
}
