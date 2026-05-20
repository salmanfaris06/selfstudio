package events

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"time"
)

var eventTypePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)

type Event struct {
	EventID    string         `json:"event_id"`
	EventType  string         `json:"event_type"`
	EntityType string         `json:"entity_type"`
	EntityID   string         `json:"entity_id"`
	OccurredAt time.Time      `json:"occurred_at"`
	Data       map[string]any `json:"data"`
}

func New(eventType string, entityType string, entityID string, data map[string]any) (Event, error) {
	if !IsDotNotation(eventType) {
		return Event{}, errors.New("event_type must use dot notation")
	}

	eventID, err := newEventID()
	if err != nil {
		return Event{}, err
	}

	if data == nil {
		data = map[string]any{}
	}

	return Event{
		EventID:    eventID,
		EventType:  eventType,
		EntityType: entityType,
		EntityID:   entityID,
		OccurredAt: time.Now().UTC(),
		Data:       data,
	}, nil
}

func IsDotNotation(eventType string) bool {
	return eventTypePattern.MatchString(eventType)
}

func (e Event) MarshalJSON() ([]byte, error) {
	type alias Event
	return json.Marshal(struct {
		alias
		OccurredAt string `json:"occurred_at"`
	}{
		alias:      alias(e),
		OccurredAt: e.OccurredAt.UTC().Format(time.RFC3339),
	})
}

func newEventID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "evt_" + hex.EncodeToString(bytes), nil
}
