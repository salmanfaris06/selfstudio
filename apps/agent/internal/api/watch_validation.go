package api

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/stations"
)

type WatchValidationHandler struct {
	store         *stations.Store
	activityStore *activity.Store
	broker        *events.Broker
}

type WatchValidationData struct {
	Validation stations.WatchValidationResult `json:"validation"`
}

func NewWatchValidationHandler(store *stations.Store, activityStore *activity.Store, broker *events.Broker) WatchValidationHandler {
	return WatchValidationHandler{store: store, activityStore: activityStore, broker: broker}
}

func (h WatchValidationHandler) Run(w http.ResponseWriter, r *http.Request) {
	stationID := r.PathValue("station_id")
	if h.store == nil {
		h.recordFailure(stationID, "Watch validation gagal karena station store belum siap.")
		writeAPIError(w, http.StatusInternalServerError, "VALIDATION_UNAVAILABLE", "Watch validation belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}
	request, ok := decodeWatchValidationRequest(w, r)
	if !ok {
		h.recordFailure(stationID, "Watch validation gagal karena request tidak valid.")
		return
	}
	result, err := stations.NewWatchValidator(h.store).Run(r.Context(), stationID, request)
	if err != nil {
		h.handleError(w, stationID, err)
		return
	}
	if result.Status == stations.ValidationStatusSuccess {
		h.recordSuccess(stationID)
	} else {
		h.recordFailure(stationID, result.Label)
	}
	h.publish(stationID, result)
	writeData(w, http.StatusOK, WatchValidationData{Validation: result})
}

func decodeWatchValidationRequest(w http.ResponseWriter, r *http.Request) (stations.WatchValidationRequest, bool) {
	if r.Body == nil || r.ContentLength == 0 {
		return stations.WatchValidationRequest{}, true
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeAPIError(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Content-Type watch validation harus application/json.", "Kirim request dengan Content-Type application/json atau tanpa body.")
		return stations.WatchValidationRequest{}, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxStationBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request stations.WatchValidationRequest
	if err := decoder.Decode(&request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_VALIDATION_REQUEST", "Request watch validation tidak valid.", "Periksa timeout_ms dan stability_ms lalu coba lagi.")
		return stations.WatchValidationRequest{}, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeAPIError(w, http.StatusBadRequest, "INVALID_VALIDATION_REQUEST", "Request watch validation tidak valid.", "Kirim satu payload JSON watch validation yang valid.")
		return stations.WatchValidationRequest{}, false
	}
	return request, true
}

func (h WatchValidationHandler) handleError(w http.ResponseWriter, stationID string, err error) {
	if errors.Is(err, stations.ErrStationNotFound) {
		h.recordFailure(stationID, "Watch validation gagal karena station tidak ditemukan.")
		writeAPIError(w, http.StatusNotFound, "STATION_NOT_FOUND", "Station tidak ditemukan.", "Refresh dashboard lalu pilih station yang tersedia.")
		return
	}
	if errors.Is(err, stations.ErrValidationCancelled) {
		h.recordFailure(stationID, "Watch validation dibatalkan.")
		writeAPIError(w, http.StatusRequestTimeout, "VALIDATION_CANCELLED", "Watch validation dibatalkan.", "Jalankan ulang validation jika masih diperlukan.")
		return
	}
	if errors.Is(err, stations.ErrInputFolderUnavailable) {
		h.recordFailure(stationID, "Watch validation gagal karena input folder tidak bisa dibaca.")
		writeAPIError(w, http.StatusBadRequest, "INPUT_FOLDER_UNAVAILABLE", "Input folder station tidak bisa dibaca.", "Periksa path dan izin folder input station lalu coba lagi.")
		return
	}
	h.recordFailure(stationID, "Watch validation gagal.")
	writeAPIError(w, http.StatusInternalServerError, "VALIDATION_UNAVAILABLE", "Watch validation gagal dijalankan.", "Coba lagi atau restart aplikasi.")
}

func (h WatchValidationHandler) recordSuccess(stationID string) {
	if h.activityStore == nil {
		return
	}
	h.activityStore.RecordWithRefs("station.validation_succeeded", activity.ResultSuccess, "Watch validation berhasil: stable JPG terdeteksi tanpa routing session.", &stationID, nil)
}

func (h WatchValidationHandler) recordFailure(stationID string, message string) {
	if h.activityStore == nil {
		return
	}
	var stationRef *string
	if knownStationID(stationID) {
		stationRef = &stationID
	}
	h.activityStore.RecordWithRefs("station.validation_failed", activity.ResultFailure, message, stationRef, nil)
}

func (h WatchValidationHandler) publish(stationID string, result stations.WatchValidationResult) {
	if h.broker == nil {
		return
	}
	event, err := events.New("station.validation_completed", "station", stationID, map[string]any{"validation": result})
	if err != nil {
		return
	}
	h.broker.Publish(event)
}
