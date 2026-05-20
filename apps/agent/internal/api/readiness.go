package api

import (
	"errors"
	"net/http"
	"path/filepath"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/stations"
)

type ReadinessHandler struct {
	store         *stations.Store
	activityStore *activity.Store
	broker        *events.Broker
	validator     stations.ReadinessValidator
}

type StationReadinessData struct {
	Readiness stations.Readiness `json:"readiness"`
}

func NewReadinessHandler(store *stations.Store, activityStore *activity.Store, broker *events.Broker, outputRoot string) ReadinessHandler {
	if outputRoot == "" {
		outputRoot = filepath.Join("local-data", "output")
	}
	return ReadinessHandler{store: store, activityStore: activityStore, broker: broker, validator: stations.NewReadinessValidator(outputRoot)}
}

func NewReadinessHandlerWithValidator(store *stations.Store, activityStore *activity.Store, broker *events.Broker, validator stations.ReadinessValidator) ReadinessHandler {
	return ReadinessHandler{store: store, activityStore: activityStore, broker: broker, validator: validator}
}

func (h ReadinessHandler) Get(w http.ResponseWriter, r *http.Request) {
	h.handle(w, r, false)
}

func (h ReadinessHandler) Check(w http.ResponseWriter, r *http.Request) {
	h.handle(w, r, true)
}

func (h ReadinessHandler) RefreshHealth(w http.ResponseWriter, r *http.Request) {
	h.handleHealthRefresh(w, r)
}

func (h ReadinessHandler) handle(w http.ResponseWriter, r *http.Request, record bool) {
	stationID := r.PathValue("station_id")
	if h.store == nil {
		if record {
			h.record(stationID, activity.ResultFailure, "Readiness check gagal karena station store belum siap.")
		}
		writeAPIError(w, http.StatusInternalServerError, "STATIONS_UNAVAILABLE", "Konfigurasi station belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}

	station, err := h.store.Get(stationID)
	if err != nil {
		if record {
			h.record(stationID, activity.ResultFailure, "Readiness check gagal karena station tidak ditemukan.")
		}
		if errors.Is(err, stations.ErrStationNotFound) {
			writeAPIError(w, http.StatusNotFound, "STATION_NOT_FOUND", "Station tidak ditemukan.", "Refresh dashboard lalu pilih station yang tersedia.")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "READINESS_CHECK_FAILED", "Readiness check gagal dijalankan.", "Coba recheck ulang atau restart aplikasi.")
		return
	}

	readiness := h.validator.Check(station)
	if record {
		result := activity.ResultSuccess
		message := "Readiness station selesai diperiksa."
		if readiness.Status == stations.ReadinessFailed {
			result = activity.ResultFailure
			message = "Readiness station selesai diperiksa dengan status gagal."
		}
		h.record(stationID, result, message)
		h.publish(readiness)
	}
	writeData(w, http.StatusOK, StationReadinessData{Readiness: readiness})
}

func (h ReadinessHandler) handleHealthRefresh(w http.ResponseWriter, r *http.Request) {
	stationID := r.PathValue("station_id")
	if h.store == nil {
		h.recordHealthRefreshFailure(stationID, "Station health refresh gagal karena station store belum siap.")
		writeAPIError(w, http.StatusInternalServerError, "STATIONS_UNAVAILABLE", "Konfigurasi station belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}
	station, err := h.store.Get(stationID)
	if err != nil {
		h.recordHealthRefreshFailure(stationID, "Station health refresh gagal karena station tidak ditemukan.")
		if errors.Is(err, stations.ErrStationNotFound) {
			writeAPIError(w, http.StatusNotFound, "STATION_NOT_FOUND", "Station tidak ditemukan.", "Refresh dashboard lalu pilih station yang tersedia.")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "READINESS_CHECK_FAILED", "Station health refresh gagal dijalankan.", "Coba refresh ulang atau restart aplikasi.")
		return
	}
	readiness := h.validator.Check(station)
	if readiness.Status == stations.ReadinessFailed {
		h.recordHealthRefreshFailure(stationID, "Station health refresh selesai dengan status failed.")
	} else {
		h.recordHealthRefreshSuccess(stationID)
	}
	h.publishHealthRefreshed(readiness)
	writeData(w, http.StatusOK, StationReadinessData{Readiness: readiness})
}

func (h ReadinessHandler) record(stationID string, result activity.Result, message string) {
	if h.activityStore == nil {
		return
	}
	var stationRef *string
	if knownStationID(stationID) {
		stationRef = &stationID
	}
	actionType := "station.readiness_checked"
	if result == activity.ResultFailure {
		actionType = "station.readiness_check_failed"
	}
	h.activityStore.RecordWithRefs(actionType, result, message, stationRef, nil)
}

func (h ReadinessHandler) recordHealthRefreshSuccess(stationID string) {
	if h.activityStore == nil {
		return
	}
	h.activityStore.RecordWithRefs("station.health_refreshed", activity.ResultSuccess, "Station health berhasil direfresh.", &stationID, nil)
}

func (h ReadinessHandler) recordHealthRefreshFailure(stationID string, message string) {
	if h.activityStore == nil {
		return
	}
	var stationRef *string
	if knownStationID(stationID) {
		stationRef = &stationID
	}
	h.activityStore.RecordWithRefs("station.health_refresh_failed", activity.ResultFailure, message, stationRef, nil)
}

func (h ReadinessHandler) publishHealthRefreshed(readiness stations.Readiness) {
	if h.broker == nil {
		return
	}
	event, err := events.New("station.health_refreshed", "station", readiness.StationID, map[string]any{"readiness": readiness})
	if err != nil {
		return
	}
	h.broker.Publish(event)
}

func (h ReadinessHandler) publish(readiness stations.Readiness) {
	if h.broker == nil {
		return
	}
	event, err := events.New("station.readiness_checked", "station", readiness.StationID, map[string]any{"readiness": readiness})
	if err != nil {
		return
	}
	h.broker.Publish(event)
}
