package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"selfstudio/agent/internal/events"
)

type EventsHandler struct {
	broker *events.Broker
}

func NewEventsHandler(broker *events.Broker) EventsHandler {
	return EventsHandler{broker: broker}
}

func (h EventsHandler) Stream(w http.ResponseWriter, r *http.Request) {
	if h.broker == nil {
		writeAPIError(w, http.StatusInternalServerError, "EVENTS_UNAVAILABLE", "Event stream belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeAPIError(w, http.StatusInternalServerError, "EVENT_STREAM_UNSUPPORTED", "Event stream tidak didukung oleh server.", "Restart aplikasi lalu coba lagi.")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	eventCh, unsubscribe := h.broker.Subscribe()
	defer unsubscribe()

	if _, err := fmt.Fprint(w, ": connected\n\n"); err != nil {
		return
	}
	flusher.Flush()

	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepalive.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			if err := writeSSEEvent(w, event); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeSSEEvent(w http.ResponseWriter, event events.Event) error {
	if !events.IsDotNotation(event.EventType) {
		return fmt.Errorf("invalid SSE event type %q", event.EventType)
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", event.EventID, event.EventType, payload)
	return err
}
