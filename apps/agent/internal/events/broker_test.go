package events

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewRequiresDotNotationEventType(t *testing.T) {
	tests := []string{
		"",
		"health",
		".updated",
		"health.",
		"health..updated",
		"health. updated",
		"health\n.updated",
		"Health.updated",
		"health.updated!",
	}

	for _, eventType := range tests {
		if _, err := New(eventType, "service", "agent", nil); err == nil {
			t.Fatalf("New returned nil error for invalid event type %q", eventType)
		}
	}
}

func TestNewCreatesPrefixedEventID(t *testing.T) {
	event, err := New("health.updated", "service", "agent", nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if len(event.EventID) != 36 || event.EventID[:4] != "evt_" {
		t.Fatalf("EventID = %q, want evt_ prefix plus 32 hex chars", event.EventID)
	}
}

func TestEventJSONUsesWrapperShape(t *testing.T) {
	event := Event{
		EventID:    "evt_1",
		EventType:  "health.updated",
		EntityType: "service",
		EntityID:   "selfstudio-agent",
		OccurredAt: time.Date(2026, 5, 18, 10, 30, 0, 0, time.UTC),
		Data:       map[string]any{"status": "ok"},
	}

	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}

	for _, key := range []string{"event_id", "event_type", "entity_type", "entity_id", "occurred_at", "data"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("missing key %s in event payload %s", key, string(payload))
		}
	}
	if decoded["occurred_at"] != "2026-05-18T10:30:00Z" {
		t.Fatalf("occurred_at = %v", decoded["occurred_at"])
	}
}

func TestBrokerPublishAndUnsubscribe(t *testing.T) {
	broker := NewBroker()
	ch, unsubscribe := broker.Subscribe()
	if broker.SubscriberCount() != 1 {
		t.Fatalf("SubscriberCount = %d, want 1", broker.SubscriberCount())
	}

	event, err := New("health.updated", "service", "selfstudio-agent", map[string]any{"status": "ok"})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	broker.Publish(event)

	select {
	case got := <-ch:
		if got.EventType != "health.updated" {
			t.Fatalf("EventType = %q", got.EventType)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}

	unsubscribe()
	if broker.SubscriberCount() != 0 {
		t.Fatalf("SubscriberCount = %d, want 0", broker.SubscriberCount())
	}
}
