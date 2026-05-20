package api

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/stations"
)

const maxStationBodyBytes = 4096

type StationsHandler struct {
	store         *stations.Store
	persistence   *stations.Persistence
	activityStore *activity.Store
	broker        *events.Broker
}

type StationListData struct {
	Stations []stations.Station `json:"stations"`
}

type StationData struct {
	Station stations.Station `json:"station"`
}

func NewStationsHandler(store *stations.Store, activityStore *activity.Store, broker *events.Broker) StationsHandler {
	return StationsHandler{store: store, activityStore: activityStore, broker: broker}
}

func NewPersistentStationsHandler(store *stations.Store, persistence stations.Persistence, activityStore *activity.Store, broker *events.Broker) StationsHandler {
	return StationsHandler{store: store, persistence: &persistence, activityStore: activityStore, broker: broker}
}

func (h StationsHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeAPIError(w, http.StatusInternalServerError, "STATIONS_UNAVAILABLE", "Konfigurasi station belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}
	writeData(w, http.StatusOK, StationListData{Stations: h.store.List()})
}

func (h StationsHandler) Update(w http.ResponseWriter, r *http.Request) {
	stationID := r.PathValue("station_id")
	if h.store == nil {
		h.recordUpdateFailure(stationID, "Update konfigurasi station gagal karena store belum siap.")
		writeAPIError(w, http.StatusInternalServerError, "STATIONS_UNAVAILABLE", "Konfigurasi station belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}

	request, ok := decodeStationUpdateRequest(w, r)
	if !ok {
		h.recordUpdateFailure(stationID, "Update konfigurasi station gagal karena format request tidak valid.")
		return
	}

	previous := h.store.List()
	station, err := h.store.Update(stationID, request)
	if err != nil {
		h.handleUpdateError(w, stationID, err)
		return
	}
	if h.persistence != nil {
		if err := h.persistence.Save(h.store); err != nil {
			_ = h.store.ReplaceAll(previous)
			h.recordUpdateFailure(stationID, "Update konfigurasi station gagal disimpan ke local storage.")
			writeAPIError(w, http.StatusInternalServerError, "STATION_CONFIG_UNAVAILABLE", "Konfigurasi station gagal disimpan ke local storage.", "Coba simpan ulang atau periksa folder local-data/config.")
			return
		}
		h.recordConfigSaved(station.StationID)
	}

	h.recordUpdateSuccess(station.StationID)
	h.publishStationUpdated(station)
	writeData(w, http.StatusOK, StationData{Station: station})
}

func decodeStationUpdateRequest(w http.ResponseWriter, r *http.Request) (stations.UpdateStation, bool) {
	if r.Body == nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_STATION_CONFIG", "Body konfigurasi station wajib dikirim.", "Kirim payload JSON konfigurasi station lalu coba lagi.")
		return stations.UpdateStation{}, false
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeAPIError(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Content-Type station config harus application/json.", "Kirim request dengan Content-Type application/json.")
		return stations.UpdateStation{}, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxStationBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var request stations.UpdateStation
	if err := decoder.Decode(&request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_STATION_CONFIG", "Format konfigurasi station tidak valid.", "Periksa field station lalu coba simpan lagi.")
		return stations.UpdateStation{}, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeAPIError(w, http.StatusBadRequest, "INVALID_STATION_CONFIG", "Format konfigurasi station tidak valid.", "Kirim satu payload JSON konfigurasi station yang valid.")
		return stations.UpdateStation{}, false
	}

	return request, true
}

func (h StationsHandler) handleUpdateError(w http.ResponseWriter, stationID string, err error) {
	if errors.Is(err, stations.ErrStationNotFound) {
		h.recordUpdateFailure(stationID, "Update konfigurasi station gagal karena station tidak ditemukan.")
		writeAPIError(w, http.StatusNotFound, "STATION_NOT_FOUND", "Station tidak ditemukan.", "Refresh dashboard lalu pilih station yang tersedia.")
		return
	}
	if errors.Is(err, stations.ErrDuplicateInputFolder) {
		h.recordUpdateFailure(stationID, "Update konfigurasi station gagal karena input folder duplikat.")
		writeAPIError(w, http.StatusBadRequest, "DUPLICATE_INPUT_FOLDER", "Input folder sudah dipakai station lain.", "Pilih folder input unik untuk setiap station.")
		return
	}
	var fieldErr stations.FieldError
	if errors.As(err, &fieldErr) {
		h.recordUpdateFailure(stationID, "Update konfigurasi station gagal karena field wajib belum lengkap.")
		writeAPIErrorWithDetails(w, http.StatusBadRequest, "INVALID_STATION_CONFIG", "Konfigurasi station belum lengkap.", "Lengkapi semua field wajib lalu simpan lagi.", map[string]any{"fields": fieldErr.Fields})
		return
	}

	h.recordUpdateFailure(stationID, "Update konfigurasi station gagal.")
	writeAPIError(w, http.StatusInternalServerError, "STATION_UPDATE_FAILED", "Konfigurasi station gagal disimpan.", "Coba simpan ulang atau restart aplikasi.")
}

func (h StationsHandler) recordConfigSaved(stationID string) {
	if h.activityStore == nil {
		return
	}
	h.activityStore.RecordWithRefs("station.config_saved", activity.ResultSuccess, "Konfigurasi station tersimpan ke local storage.", &stationID, nil)
}

func (h StationsHandler) recordUpdateSuccess(stationID string) {
	if h.activityStore == nil {
		return
	}
	h.activityStore.RecordWithRefs("station.config_updated", activity.ResultSuccess, "Konfigurasi station berhasil disimpan.", &stationID, nil)
}

func (h StationsHandler) recordUpdateFailure(stationID string, message string) {
	if h.activityStore == nil {
		return
	}
	var stationRef *string
	if knownStationID(stationID) {
		stationRef = &stationID
	}
	h.activityStore.RecordWithRefs("station.config_update_failed", activity.ResultFailure, message, stationRef, nil)
}

func knownStationID(stationID string) bool {
	switch strings.TrimSpace(stationID) {
	case stations.Station1ID, stations.Station2ID, stations.Station3ID:
		return true
	default:
		return false
	}
}

func (h StationsHandler) publishStationUpdated(station stations.Station) {
	if h.broker == nil {
		return
	}
	event, err := events.New("station.updated", "station", station.StationID, map[string]any{"station": station})
	if err != nil {
		return
	}
	h.broker.Publish(event)
}
