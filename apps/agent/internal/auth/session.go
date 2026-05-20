package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

const (
	SessionCookieName = "selfstudio_session"
	SessionTTL        = 12 * time.Hour
	MaxFailedAttempts = 5
	LockoutDuration   = 1 * time.Minute
)

var (
	ErrInvalidPIN  = errors.New("invalid pin")
	ErrRateLimited = errors.New("too many invalid pin attempts")
)

type sessionRecord struct {
	expiresAt time.Time
}

type attemptRecord struct {
	count     int
	lockedTil time.Time
}

type Manager struct {
	pinSecret []byte
	sessions  map[string]sessionRecord
	attempts  map[string]attemptRecord
	now       func() time.Time
	mu        sync.RWMutex
}

func NewManager(configuredPIN string) (*Manager, error) {
	if configuredPIN == "" {
		return nil, errors.New("auth pin is required")
	}

	return &Manager{
		pinSecret: []byte(configuredPIN),
		sessions:  make(map[string]sessionRecord),
		attempts:  make(map[string]attemptRecord),
		now:       time.Now,
	}, nil
}

func (m *Manager) Login(pin string) (string, time.Time, error) {
	return m.LoginForKey("local", pin)
}

func (m *Manager) LoginForKey(clientKey string, pin string) (string, time.Time, error) {
	if clientKey == "" {
		clientKey = "local"
	}

	if m.rateLimited(clientKey) {
		return "", time.Time{}, ErrRateLimited
	}

	if !m.validPIN(pin) {
		m.recordFailure(clientKey)
		return "", time.Time{}, ErrInvalidPIN
	}

	token, err := newSessionToken()
	if err != nil {
		return "", time.Time{}, err
	}

	expiresAt := m.now().Add(SessionTTL)
	m.mu.Lock()
	m.sessions[token] = sessionRecord{expiresAt: expiresAt}
	delete(m.attempts, clientKey)
	m.mu.Unlock()

	return token, expiresAt, nil
}

func (m *Manager) Authenticated(token string) bool {
	if token == "" {
		return false
	}

	now := m.now()
	m.mu.Lock()
	session, ok := m.sessions[token]
	if !ok {
		m.mu.Unlock()
		return false
	}
	if !now.Before(session.expiresAt) {
		delete(m.sessions, token)
		m.mu.Unlock()
		return false
	}
	m.mu.Unlock()

	return true
}

func (m *Manager) Logout(token string) bool {
	if token == "" {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[token]; !ok {
		return false
	}
	delete(m.sessions, token)
	return true
}

func (m *Manager) validPIN(pin string) bool {
	candidate := []byte(pin)
	if len(candidate) != len(m.pinSecret) {
		// Compare against the configured secret anyway to avoid a fast no-op path.
		subtle.ConstantTimeCompare(m.pinSecret, m.pinSecret)
		return false
	}

	return subtle.ConstantTimeCompare(candidate, m.pinSecret) == 1
}

func (m *Manager) rateLimited(clientKey string) bool {
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()

	attempt := m.attempts[clientKey]
	if attempt.lockedTil.IsZero() || now.After(attempt.lockedTil) {
		if !attempt.lockedTil.IsZero() {
			delete(m.attempts, clientKey)
		}
		return false
	}

	return true
}

func (m *Manager) recordFailure(clientKey string) {
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()

	attempt := m.attempts[clientKey]
	if !attempt.lockedTil.IsZero() && now.After(attempt.lockedTil) {
		attempt = attemptRecord{}
	}
	attempt.count++
	if attempt.count >= MaxFailedAttempts {
		attempt.lockedTil = now.Add(LockoutDuration)
	}
	m.attempts[clientKey] = attempt
}

func newSessionToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
