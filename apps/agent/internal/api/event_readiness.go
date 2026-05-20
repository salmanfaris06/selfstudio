package api

import (
	"errors"
	"net/http"
	"path/filepath"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/events"
	eventreadiness "selfstudio/agent/internal/readiness"
	"selfstudio/agent/internal/stations"
)

type EventReadinessHandler struct {
	builder       eventreadiness.Builder
	activityStore *activity.Store
	broker        *events.Broker
}

type EventReadinessData struct {
	Readiness eventreadiness.Checklist `json:"readiness"`
}

func NewEventReadinessHandler(store *stations.Store, activityStore *activity.Store, broker *events.Broker, outputRoot string) EventReadinessHandler {
	if outputRoot == "" {
		outputRoot = filepath.Join("local-data", "output")
	}
	return EventReadinessHandler{builder: eventreadiness.NewBuilder(store, stations.NewReadinessValidator(outputRoot), outputRoot), activityStore: activityStore, broker: broker}
}

func NewEventReadinessHandlerWithValidator(store *stations.Store, activityStore *activity.Store, broker *events.Broker, outputRoot string, validator stations.ReadinessValidator) EventReadinessHandler {
	if outputRoot == "" {
		outputRoot = filepath.Join("local-data", "output")
	}
	return EventReadinessHandler{builder: eventreadiness.NewBuilder(store, validator, outputRoot), activityStore: activityStore, broker: broker}
}

func (h EventReadinessHandler) Get(w http.ResponseWriter, r *http.Request) {
	checklist, err := h.builder.Build()
	if err != nil {
		h.writeReadinessError(w, err)
		return
	}
	writeData(w, http.StatusOK, EventReadinessData{Readiness: checklist})
}

func (h EventReadinessHandler) Check(w http.ResponseWriter, r *http.Request) {
	if h.activityStore == nil || h.broker == nil {
		writeAPIError(w, http.StatusInternalServerError, "READINESS_UNAVAILABLE", "Event readiness belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}
	checklist, err := h.builder.Build()
	if err != nil {
		h.writeReadinessError(w, err)
		return
	}
	result := activity.ResultSuccess
	message := "Event readiness checklist selesai diperiksa."
	if checklist.Status == eventreadiness.StatusFailed {
		result = activity.ResultFailure
		message = "Event readiness checklist selesai dengan status gagal."
	} else if checklist.Status != eventreadiness.StatusReady {
		message = "Event readiness checklist selesai dengan status " + string(checklist.Status) + "."
	}
	h.record(result, message)
	h.publish(checklist)
	writeData(w, http.StatusOK, EventReadinessData{Readiness: checklist})
}

func (h EventReadinessHandler) writeReadinessError(w http.ResponseWriter, err error) {
	if errors.Is(err, eventreadiness.ErrUnavailable) {
		writeAPIError(w, http.StatusInternalServerError, "READINESS_UNAVAILABLE", "Event readiness belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}
	writeAPIError(w, http.StatusInternalServerError, "READINESS_CHECK_FAILED", "Event readiness check gagal dijalankan.", "Coba recheck ulang atau restart aplikasi.")
}

func (h EventReadinessHandler) record(result activity.Result, message string) {
	h.activityStore.RecordWithRefs(actionTypeForChecklist(result), result, message, nil, nil)
}

func actionTypeForChecklist(result activity.Result) string {
	if result == activity.ResultFailure {
		return "readiness.check_failed"
	}
	return "readiness.checked"
}

func (h EventReadinessHandler) publish(checklist eventreadiness.Checklist) {
	event, err := events.New("readiness.checked", "readiness", "event", map[string]any{"readiness": checklist})
	if err != nil {
		return
	}
	h.broker.Publish(event)
}
